package graphql

import (
	"context"
	"encoding/json"
	"github.com/teaelephant/TeaElephantMemory/common"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// GraphQL response fields deliberately use the public camelCase contract.
const ratingValueVariable = "value"
const ratingFloatTolerance = 0.00001
const ratingFields = `teaID average count myRating canRate`
const ratingSetMutation = `mutation($tea:ID!,$value:Int!){setTeaRating(teaID:$tea,rating:$value){` + ratingFields + `}}`
const ratingClearMutation = `mutation($tea:ID!){clearTeaRating(teaID:$tea){` + ratingFields + `}}`
const ratingReadQuery = `query($tea:ID!){tea(id:$tea){rating{` + ratingFields + `}}}`
const ratingUpdatedAtSQL = `SELECT updated_at::text FROM tea_ratings WHERE user_id=$1 AND tea_id=$2`
const ratingValueSQL = `SELECT rating FROM tea_ratings WHERE user_id=$1 AND tea_id=$2`
const ratingMigrationPath = "../../../../db/migrations/20261104_tea_ratings.sql"

//nolint:tagliatelle // GraphQL transport fields use camelCase.
type ratingWire struct {
	TeaID    uuid.UUID `json:"teaID"`
	Average  *float64  `json:"average"`
	Count    int       `json:"count"`
	MyRating *int      `json:"myRating"`
	CanRate  bool      `json:"canRate"`
}

func ratingSet(t *testing.T, f mergeFixture, token string, teaID uuid.UUID, value int) ratingWire {
	t.Helper()
	reply := imageGraphQLCall(t, f.server, token, ratingSetMutation, map[string]any{testTeaVariable: teaID.String(), ratingValueVariable: value})
	require.Empty(t, reply.Errors)

	var result ratingWire
	require.NoError(t, json.Unmarshal(reply.Data["setTeaRating"], &result))
	require.Equal(t, teaID, result.TeaID)

	return result
}

func ratingRead(t *testing.T, f mergeFixture, token string) ratingWire {
	t.Helper()
	reply := imageGraphQLCall(t, f.server, token, ratingReadQuery, map[string]any{testTeaVariable: f.teaID.String()})
	require.Empty(t, reply.Errors)

	var result struct {
		Rating ratingWire `json:"rating"`
	}
	require.NoError(t, json.Unmarshal(reply.Data["tea"], &result))
	require.Equal(t, f.teaID, result.Rating.TeaID)

	return result.Rating
}

func TestTeaRatingsAuthorizationEligibilityAndRange(t *testing.T) {
	f := newMergeFixture(t)
	for _, token := range []string{"", f.adminToken} {
		reply := imageGraphQLCall(t, f.server, token, ratingSetMutation, map[string]any{testTeaVariable: f.teaID.String(), ratingValueVariable: 5})
		assertRoleDenied(t, reply, testImageUnauthorizedCode)
		reply = imageGraphQLCall(t, f.server, token, ratingClearMutation, map[string]any{testTeaVariable: f.teaID.String()})
		assertRoleDenied(t, reply, testImageUnauthorizedCode)
	}

	for _, value := range []int{-1, 0, 6} {
		reply := imageGraphQLCall(t, f.server, f.ownerToken, ratingSetMutation, map[string]any{testTeaVariable: f.teaID.String(), ratingValueVariable: value})
		assertRoleDenied(t, reply, badUserInputCode)
	}

	reply := imageGraphQLCall(t, f.server, f.otherToken, ratingSetMutation, map[string]any{testTeaVariable: f.teaID.String(), ratingValueVariable: 3})
	assertRoleDenied(t, reply, testImageForbiddenCode)
	reply = imageGraphQLCall(t, f.server, f.ownerToken, ratingSetMutation, map[string]any{testTeaVariable: uuid.NewString(), ratingValueVariable: 3})
	assertRoleDenied(t, reply, roleNotFoundCode)
	empty := ratingRead(t, f, "")
	require.Nil(t, empty.Average)
	require.Zero(t, empty.Count)
	require.Nil(t, empty.MyRating)
	require.False(t, empty.CanRate)
	require.True(t, ratingRead(t, f, f.ownerToken).CanRate)

	for value := 1; value <= 5; value++ {
		got := ratingSet(t, f, f.ownerToken, f.teaID, value)
		require.Equal(t, value, *got.MyRating)
		require.InDelta(t, float64(value), *got.Average, ratingFloatTolerance)
		require.Equal(t, 1, got.Count)
	}

	for _, token := range []string{"", f.adminToken, f.otherToken} {
		got := ratingRead(t, f, token)
		require.Nil(t, got.MyRating)
		require.False(t, got.CanRate)
		require.Equal(t, 1, got.Count)
	}
}

func TestTeaRatingsOneVoteRetentionAndClear(t *testing.T) {
	f := newMergeFixture(t)
	otherCollection := uuid.New()
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO collections(id,user_id,name) VALUES($1,$2,'Other'),($3,$4,'Duplicate');`, otherCollection, f.otherID, uuid.New(), f.ownerID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO collection_qr_items(collection_id,qr_id) SELECT id,$1 FROM collections ON CONFLICT DO NOTHING`, f.labelID)
	require.NoError(t, err)
	ratingSet(t, f, f.ownerToken, f.teaID, 1)
	got := ratingSet(t, f, f.otherToken, f.teaID, 4)
	require.Equal(t, 2, got.Count)
	require.InDelta(t, 2.5, *got.Average, ratingFloatTolerance)

	var before, after string
	require.NoError(t, f.db.QueryRowContext(t.Context(), ratingUpdatedAtSQL, f.ownerID, f.teaID).Scan(&before))
	ratingSet(t, f, f.ownerToken, f.teaID, 1)
	require.NoError(t, f.db.QueryRowContext(t.Context(), ratingUpdatedAtSQL, f.ownerID, f.teaID).Scan(&after))
	require.Equal(t, before, after, "same-value set is idempotent, including merge conflict timestamp")
	_, err = f.db.ExecContext(t.Context(), `DELETE FROM collections WHERE user_id=$1`, f.ownerID)
	require.NoError(t, err)
	got = ratingRead(t, f, f.ownerToken)
	require.False(t, got.CanRate)
	require.Equal(t, 1, *got.MyRating)
	require.Equal(t, 2, got.Count)
	reply := imageGraphQLCall(t, f.server, f.ownerToken, ratingSetMutation, map[string]any{testTeaVariable: f.teaID.String(), ratingValueVariable: 5})
	assertRoleDenied(t, reply, testImageForbiddenCode)

	for range 2 {
		reply = imageGraphQLCall(t, f.server, f.ownerToken, ratingClearMutation, map[string]any{testTeaVariable: f.teaID.String()})
		require.Empty(t, reply.Errors)
		require.NoError(t, json.Unmarshal(reply.Data["clearTeaRating"], &got))
		require.Nil(t, got.MyRating)
		require.Equal(t, 1, got.Count)
		require.InDelta(t, 4.0, *got.Average, ratingFloatTolerance)
	}

	_, err = f.db.ExecContext(t.Context(), `INSERT INTO tea_ratings(user_id,tea_id,rating) VALUES($1,$2,0)`, f.ownerID, f.teaID)
	require.Error(t, err, "database must enforce integer scale")

	reply = imageGraphQLCall(t, f.server, f.otherToken, ratingClearMutation, map[string]any{testTeaVariable: f.teaID.String()})
	require.Empty(t, reply.Errors)
	require.NoError(t, json.Unmarshal(reply.Data["clearTeaRating"], &got))
	require.Zero(t, got.Count)
	require.Nil(t, got.Average)
	require.Nil(t, got.MyRating)
}

func TestTeaRatingsMigrationRepeatableAndPreservesLegacyReceipts(t *testing.T) {
	f := newMergeFixture(t)
	ratingSet(t, f, f.ownerToken, f.teaID, 5)
	preview := mergePreview(t, f)
	args := mergeArguments(f, preview, uuid.NewString())
	original := mergeCommit(t, f, args)
	source, err := os.ReadFile(ratingMigrationPath)
	require.NoError(t, err)

	for range 2 {
		_, err = f.db.ExecContext(t.Context(), string(source))
		require.NoError(t, err)
	}

	replay := mergeCommit(t, f, args)
	require.True(t, replay.Replayed)
	require.Equal(t, original.Receipt, replay.Receipt)

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM tea_ratings WHERE tea_id=$1 AND rating=5`, f.targetID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestTeaRatingsMergeNewestConflictsAndPreviewInvalidation(t *testing.T) {
	f := newMergeFixture(t)
	ratingSet(t, f, f.ownerToken, f.teaID, 5)
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO tea_ratings(user_id,tea_id,rating,updated_at) VALUES($1,$2,1,'2000-01-01'),($3,$2,3,'2000-01-01'),($3,$4,2,'2000-01-01')`, f.ownerID, f.targetID, f.otherID, f.teaID)
	require.NoError(t, err)
	preview := mergePreview(t, f)
	ratingSet(t, f, f.ownerToken, f.teaID, 4)
	reply := imageGraphQLCall(t, f.server, f.adminToken, mergeMutation, mergeArguments(f, preview, uuid.NewString()))
	assertRoleDenied(t, reply, mergePreviewConflictCode)
	preview = mergePreview(t, f)
	args := mergeArguments(f, preview, uuid.NewString())
	mergeCommit(t, f, args)

	for user, want := range map[uuid.UUID]int{f.ownerID: 4, f.otherID: 3} {
		var value int
		require.NoError(t, f.db.QueryRowContext(t.Context(), ratingValueSQL, user, f.targetID).Scan(&value))
		require.Equal(t, want, value)
	}

	reply = imageGraphQLCall(t, f.server, f.adminToken, `query($source:ID!,$target:ID!,$sr:String!,$tr:String!,$token:String!,$key:String!){editorTeaMergeReceipt(sourceTeaID:$source,targetTeaID:$target,expectedSourceRevision:$sr,expectedTargetRevision:$tr,previewToken:$token,idempotencyKey:$key){ratingsMoved ratingConflictsResolved}}`, args)
	require.Empty(t, reply.Errors)
	//nolint:tagliatelle // GraphQL transport fields use camelCase.
	var counts struct {
		Moved     int `json:"ratingsMoved"`
		Conflicts int `json:"ratingConflictsResolved"`
	}
	require.NoError(t, json.Unmarshal(reply.Data["editorTeaMergeReceipt"], &counts))
	require.Equal(t, 2, counts.Moved)
	require.Equal(t, 2, counts.Conflicts)
}

func TestTeaRatingsMergeLateFailureRestoresVotesAndTimestamps(t *testing.T) {
	f := newMergeFixture(t)
	ratingSet(t, f, f.ownerToken, f.teaID, 5)
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO tea_ratings(user_id,tea_id,rating,updated_at) VALUES($1,$2,1,'2000-01-01')`, f.ownerID, f.targetID)
	require.NoError(t, err)
	before := ratingSnapshot(t, f)
	preview := mergePreview(t, f)
	_, err = f.db.ExecContext(t.Context(), `CREATE FUNCTION reject_rating_merge() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'test rejects merge receipt'; END $$ LANGUAGE plpgsql; CREATE TRIGGER reject_rating_merge BEFORE INSERT ON tea_merge_receipt_details FOR EACH ROW EXECUTE FUNCTION reject_rating_merge()`)
	require.NoError(t, err)

	key := uuid.NewString()
	reply := imageGraphQLCall(t, f.server, f.adminToken, mergeMutation, mergeArguments(f, preview, key))
	require.NotEmpty(t, reply.Errors)
	require.Equal(t, before, ratingSnapshot(t, f))
	require.Equal(t, preview.PreviewToken, mergePreview(t, f).PreviewToken)

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), mergeCountReceiptSQL, key).Scan(&count))
	require.Zero(t, count)
}

func ratingSnapshot(t *testing.T, f mergeFixture) string {
	t.Helper()

	var snapshot string
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY user_id,tea_id),'[]'::jsonb)::text FROM tea_ratings r`).Scan(&snapshot))

	return snapshot
}

func TestTeaRatingsInFlightVoteSerializesMergeAndInvalidatesPreview(t *testing.T) {
	f := newMergeFixture(t)
	preview := mergePreview(t, f)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	tx, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)

	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(851176)`)
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, `CREATE FUNCTION pause_rating_insert() RETURNS trigger AS $$ BEGIN PERFORM pg_advisory_xact_lock(851176); RETURN NEW; END $$ LANGUAGE plpgsql; CREATE TRIGGER pause_rating_insert BEFORE INSERT ON tea_ratings FOR EACH ROW EXECUTE FUNCTION pause_rating_insert()`)
	require.NoError(t, err)

	votes := make(chan agentCallResult, 1)
	go func() {
		votes <- concurrentAgentGraphQLCall(ctx, f.server, f.ownerToken, ratingSetMutation, map[string]any{testTeaVariable: f.teaID.String(), ratingValueVariable: 5})
	}()

	require.Eventually(t, func() bool {
		var waiting bool

		err := f.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND objid=851176 AND NOT granted)`).Scan(&waiting)

		return err == nil && waiting
	}, time.Second, 5*time.Millisecond, "vote must reach the insert while holding the catalog lock")

	merges := make(chan agentCallResult, 1)
	go func() {
		merges <- concurrentAgentGraphQLCall(ctx, f.server, f.adminToken, mergeMutation, mergeArguments(f, preview, uuid.NewString()))
	}()

	require.Eventually(t, func() bool {
		var waiting bool

		err := f.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND ((classid::bigint << 32) | objid::bigint)=$1 AND NOT granted)`, common.CatalogueWriteLock).Scan(&waiting)

		return err == nil && waiting
	}, time.Second, 5*time.Millisecond, "merge must wait for the in-flight vote")
	require.NoError(t, tx.Commit())

	vote := <-votes
	require.NoError(t, vote.Err)
	require.Empty(t, vote.Reply.Errors)

	merged := <-merges
	require.NoError(t, merged.Err)
	assertRoleDenied(t, merged.Reply, mergePreviewConflictCode)
	mergeCommit(t, f, mergeArguments(f, mergePreview(t, f), uuid.NewString()))

	var value int
	require.NoError(t, f.db.QueryRowContext(t.Context(), ratingValueSQL, f.ownerID, f.targetID).Scan(&value))
	require.Equal(t, 5, value)
}

func TestTeaRatingsRemovedMembershipCannotRaceEligibility(t *testing.T) {
	f := newMergeFixture(t)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	tx, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)

	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `DELETE FROM collection_qr_items WHERE collection_id=$1`, f.collectionID)
	require.NoError(t, err)

	votes := make(chan agentCallResult, 1)
	go func() {
		votes <- concurrentAgentGraphQLCall(ctx, f.server, f.ownerToken, ratingSetMutation, map[string]any{testTeaVariable: f.teaID.String(), ratingValueVariable: 5})
	}()

	require.Eventually(t, func() bool {
		var waiting bool

		err := f.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='collection_qr_items'::regclass AND NOT granted)`).Scan(&waiting)

		return err == nil && waiting
	}, time.Second, 5*time.Millisecond)
	require.NoError(t, tx.Commit())

	vote := <-votes
	require.NoError(t, vote.Err)
	assertRoleDenied(t, vote.Reply, testImageForbiddenCode)
	require.Zero(t, ratingRead(t, f, f.ownerToken).Count)
}

func TestTeaRatingsUpgradePreservesPreRatingMergeReceipt(t *testing.T) {
	f := newMergeFixture(t)
	args := mergeArguments(f, mergePreview(t, f), uuid.NewString())
	original := mergeCommit(t, f, args)
	_, err := f.db.ExecContext(t.Context(), `DROP TABLE tea_ratings; ALTER TABLE tea_merge_receipt_details DROP COLUMN ratings_moved, DROP COLUMN rating_conflicts_resolved;`)
	require.NoError(t, err)
	source, err := os.ReadFile(ratingMigrationPath)
	require.NoError(t, err)

	for range 2 {
		_, err = f.db.ExecContext(t.Context(), string(source))
		require.NoError(t, err)
	}

	replay := mergeCommit(t, f, args)
	require.True(t, replay.Replayed)
	require.Equal(t, original.Receipt, replay.Receipt)
}

func TestTeaRatingsMergeKeepsNewerTargetAndChosenTimestamp(t *testing.T) {
	f := newMergeFixture(t)
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO tea_ratings(user_id,tea_id,rating,updated_at) VALUES($1,$2,1,'2000-01-01'),($1,$3,5,'2001-01-01')`, f.ownerID, f.teaID, f.targetID)
	require.NoError(t, err)
	mergeCommit(t, f, mergeArguments(f, mergePreview(t, f), uuid.NewString()))

	var (
		value int
		stamp time.Time
	)
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT rating,updated_at FROM tea_ratings WHERE user_id=$1 AND tea_id=$2`, f.ownerID, f.targetID).Scan(&value, &stamp))
	require.Equal(t, 5, value)
	require.Equal(t, time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC), stamp.UTC())
}
