//nolint:tagliatelle // Wire fixtures intentionally match the public GraphQL camelCase field names.
package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/internal/consumption"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
	pgadapter "github.com/teaelephant/TeaElephantMemory/pkg/pg"
)

const mergePreviewQuery = `query($source:ID!,$target:ID!){previewTeaMerge(sourceTeaID:$source,targetTeaID:$target){sourceTea{id revision name type description tags{id revision name category{id name revision}} images{id url isCover createdAt}} targetTea{id revision name type description tags{id revision name category{id name revision}} images{id url isCover createdAt}} previewToken qrCount collectionMembershipCount tagsToAdd imagesToMove consumptionCount resultingImageCount resultingCoverImageID blockers metadataPolicy coverPolicy}}`
const mergeResultSelection = `receipt{idempotencyKey operation teaID resourceID resultRevision completedAt} targetTea{id revision name description tags{id} images{id isCover}} replayed sourceTeaID targetTeaID qrCount collectionMembershipCount tagsAdded imagesMoved consumptionsMoved`
const mergeMutation = `mutation($source:ID!,$target:ID!,$sr:String!,$tr:String!,$token:String!,$key:String!){mergeTeas(sourceTeaID:$source,targetTeaID:$target,expectedSourceRevision:$sr,expectedTargetRevision:$tr,previewToken:$token,idempotencyKey:$key){` + mergeResultSelection + `}}`
const mergeReceiptQuery = `query($source:ID!,$target:ID!,$sr:String!,$tr:String!,$token:String!,$key:String!){editorTeaMergeReceipt(sourceTeaID:$source,targetTeaID:$target,expectedSourceRevision:$sr,expectedTargetRevision:$tr,previewToken:$token,idempotencyKey:$key){` + mergeResultSelection + `}}`
const mergePreviewField = "previewTeaMerge"
const mergeMutationField = "mergeTeas"
const mergeSourceVar = "source"
const mergeTargetVar = "target"
const mergeTokenVar = "token"
const mergePreviewConflictCode = "PREVIEW_CONFLICT"
const mergeBlockedCode = "MERGE_BLOCKED"
const mergeUpdatedDescription = "Later target edit"

const mergeInsertMembership = `INSERT INTO collection_qr_items(collection_id,qr_id) VALUES($1,$2)`
const mergeInsertTagAssignment = `INSERT INTO tea_tags(tea_id,tag_id) VALUES($1,$2)`
const mergeCanonicalName = `Canonical Jasmine`
const mergeCountMembership = `SELECT count(*) FROM collection_qr_items WHERE collection_id=$1 AND qr_id=$2`
const mergeCountTags = `SELECT count(*) FROM tea_tags WHERE tea_id=$1`
const mergeQRMetadataCase = `qr_metadata`
const mergeHistoryCase = `history`
const mergeDeleteTeaSQL = `DELETE FROM teas WHERE id=$1`
const mergeImageParentSQL = `SELECT tea_id FROM tea_images WHERE id=$1`
const mergeCountReceiptSQL = `SELECT count(*) FROM catalog_operation_receipts WHERE operation_key=$1`
const mergeNullJSON = `null`
const mergeHistoricalReceiptsSQL = `SELECT jsonb_agg(to_jsonb(r) ORDER BY operation_key)::text FROM catalog_operation_receipts r WHERE operation_key IN($1,$2)`
const mergeCountUserHistorySQL = `SELECT count(*) FROM consumptions WHERE tea_id=$1 AND user_id=$2`

// Wire structs use the public GraphQL JSON field names.
type mergeTeaWire struct {
	ID          uuid.UUID `json:"id"`
	Revision    string    `json:"revision"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []struct {
		ID uuid.UUID `json:"id"`
	} `json:"tags"`
	Images []struct {
		ID      uuid.UUID `json:"id"`
		IsCover bool      `json:"isCover"`
	} `json:"images"`
}

type mergePreviewWire struct {
	SourceTea                 *mergeTeaWire `json:"sourceTea"`
	TargetTea                 *mergeTeaWire `json:"targetTea"`
	PreviewToken              string        `json:"previewToken"`
	MetadataPolicy            string        `json:"metadataPolicy"`
	CoverPolicy               string        `json:"coverPolicy"`
	QRCount                   int           `json:"qrCount"`
	CollectionMembershipCount int           `json:"collectionMembershipCount"`
	TagsToAdd                 int           `json:"tagsToAdd"`
	ImagesToMove              int           `json:"imagesToMove"`
	ConsumptionCount          int           `json:"consumptionCount"`
	ResultingImageCount       int           `json:"resultingImageCount"`
	Blockers                  []string      `json:"blockers"`
	ResultingCoverImageID     *uuid.UUID    `json:"resultingCoverImageID"`
}

type mergeReceiptWire struct {
	IdempotencyKey string    `json:"idempotencyKey"`
	Operation      string    `json:"operation"`
	ResultRevision string    `json:"resultRevision"`
	TeaID          uuid.UUID `json:"teaID"`
	ResourceID     uuid.UUID `json:"resourceID"`
	CompletedAt    time.Time `json:"completedAt"`
}

type mergeResultWire struct {
	Receipt                   *mergeReceiptWire `json:"receipt"`
	TargetTea                 *mergeTeaWire     `json:"targetTea"`
	Replayed                  bool              `json:"replayed"`
	SourceTeaID               uuid.UUID         `json:"sourceTeaID"`
	TargetTeaID               uuid.UUID         `json:"targetTeaID"`
	QRCount                   int               `json:"qrCount"`
	CollectionMembershipCount int               `json:"collectionMembershipCount"`
	TagsAdded                 int               `json:"tagsAdded"`
	ImagesMoved               int               `json:"imagesMoved"`
	ConsumptionsMoved         int               `json:"consumptionsMoved"`
}

type mergeFixture struct {
	roleFixture
	targetID     uuid.UUID
	labelID      uuid.UUID
	collectionID uuid.UUID
}

func newMergeFixture(t *testing.T) mergeFixture {
	t.Helper()
	f := newAgentFixture(t)
	target, label, collectionID := uuid.New(), uuid.New(), uuid.New()
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO teas(id,name,type,description) VALUES($1,'Canonical Jasmine','tea','Target description')`, target)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO qr_records(id,tea_id,boiling_temp,expiration_date) VALUES($1,$2,85,'2030-01-01T00:00:00Z')`, label, f.teaID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO collections(id,user_id,name) VALUES($1,$2,'Shelf')`, collectionID, f.ownerID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), mergeInsertMembership, collectionID, label)
	require.NoError(t, err)

	return mergeFixture{roleFixture: f, targetID: target, labelID: label, collectionID: collectionID}
}

func mergePreview(t *testing.T, f mergeFixture) mergePreviewWire {
	t.Helper()
	reply := imageGraphQLCall(t, f.server, f.adminToken, mergePreviewQuery, map[string]any{mergeSourceVar: f.teaID.String(), mergeTargetVar: f.targetID.String()})
	require.Empty(t, reply.Errors)

	var value mergePreviewWire
	require.NoError(t, json.Unmarshal(reply.Data[mergePreviewField], &value))

	return value
}

func mergeArguments(f mergeFixture, preview mergePreviewWire, key string) map[string]any {
	return map[string]any{mergeSourceVar: f.teaID.String(), mergeTargetVar: f.targetID.String(), "sr": preview.SourceTea.Revision, "tr": preview.TargetTea.Revision, mergeTokenVar: preview.PreviewToken, agentKeyVariable: key}
}

func mergeCommit(t *testing.T, f mergeFixture, arguments map[string]any) mergeResultWire {
	t.Helper()
	reply := imageGraphQLCall(t, f.server, f.adminToken, mergeMutation, arguments)
	require.Empty(t, reply.Errors)

	var result mergeResultWire
	require.NoError(t, json.Unmarshal(reply.Data[mergeMutationField], &result))

	return result
}

func mergeSeedTagsImagesHistory(t *testing.T, f mergeFixture) []uuid.UUID {
	t.Helper()
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO tea_tags(tea_id,tag_id) VALUES($1,$2),($3,$2)`, f.teaID, f.tagID, f.targetID)
	require.NoError(t, err)

	anotherTag := uuid.New()
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO tags(id,name,color,category_id) VALUES($1,'Unique source','green',$2);`, anotherTag, f.categoryID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), mergeInsertTagAssignment, f.teaID, anotherTag)
	require.NoError(t, err)

	images := []uuid.UUID{uuid.New(), uuid.New()}
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO tea_images(id,tea_id,content_type,width,height,data,is_cover) VALUES($1,$2,'image/png',1,1,'source-bytes',true),($3,$4,'image/png',1,1,'target-bytes',true)`, images[0], f.teaID, images[1], f.targetID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO consumptions(user_id,ts,tea_id) VALUES($1,'2026-10-12T08:00:00Z',$2),($3,'2026-10-12T08:01:00Z',$2),($1,'2026-10-12T08:02:00Z',$4)`, f.ownerID, f.teaID, f.otherID, f.targetID)
	require.NoError(t, err)

	return images
}

func TestTeaMergePreservesQRCollectionsImagesTagsAndHistory(t *testing.T) {
	f := newMergeFixture(t)
	images := mergeSeedTagsImagesHistory(t, f)
	preview := mergePreview(t, f)
	require.Empty(t, preview.Blockers)
	require.Equal(t, 1, preview.QRCount)
	require.Equal(t, 1, preview.CollectionMembershipCount)
	require.Equal(t, 1, preview.TagsToAdd)
	require.Equal(t, 2, preview.ConsumptionCount)
	require.Equal(t, common.TeaMergeMetadataPolicy, preview.MetadataPolicy)
	require.Equal(t, common.TeaMergeCoverPolicy, preview.CoverPolicy)
	require.Equal(t, preview.PreviewToken, mergePreview(t, f).PreviewToken)
	result := mergeCommit(t, f, mergeArguments(f, preview, uuid.NewString()))
	require.False(t, result.Replayed)
	require.Equal(t, f.teaID, result.SourceTeaID)
	require.Equal(t, f.targetID, result.TargetTeaID)
	require.Equal(t, mergeCanonicalName, result.TargetTea.Name)
	require.Equal(t, "Target description", result.TargetTea.Description)
	require.NotEqual(t, preview.TargetTea.Revision, result.TargetTea.Revision)
	require.Equal(t, 1, result.TagsAdded)
	require.Equal(t, 2, result.ConsumptionsMoved)

	var (
		teaID  uuid.UUID
		temp   int
		expiry time.Time
	)
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT tea_id,boiling_temp,expiration_date FROM qr_records WHERE id=$1`, f.labelID).Scan(&teaID, &temp, &expiry))
	require.Equal(t, f.targetID, teaID)
	require.Equal(t, 85, temp)
	require.Equal(t, 2030, expiry.Year())
	assertMergePreservedItems(t, f, images)
	cols := imageGraphQLCall(t, f.server, f.ownerToken, `query{collections{id records{id tea{id}}}}`, nil)
	require.Empty(t, cols.Errors)
	require.Contains(t, string(cols.Data["collections"]), f.labelID.String())
	require.Contains(t, string(cols.Data["collections"]), f.targetID.String())
}

func assertMergePreservedItems(t *testing.T, f mergeFixture, images []uuid.UUID) {
	t.Helper()

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), mergeCountMembership, f.collectionID, f.labelID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM teas WHERE id=$1`, f.teaID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, f.db.QueryRowContext(t.Context(), mergeCountTags, f.targetID).Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM consumptions WHERE tea_id=$1`, f.targetID).Scan(&count))
	require.Equal(t, 3, count)

	var (
		cover bool
		data  []byte
		teaID uuid.UUID
	)
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT tea_id,is_cover,data FROM tea_images WHERE id=$1`, images[0]).Scan(&teaID, &cover, &data))
	require.Equal(t, f.targetID, teaID)
	require.False(t, cover)
	require.Equal(t, []byte("source-bytes"), data)
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT is_cover FROM tea_images WHERE id=$1`, images[1]).Scan(&cover))
	require.True(t, cover)
}

func TestTeaMergeRefusesStaleDependenciesWithoutTeaRevisionChange(t *testing.T) {
	changes := map[string]string{
		mergeQRMetadataCase: `UPDATE qr_records SET boiling_temp=99 WHERE id=$1`,
		"qr_binding":        `UPDATE qr_records SET tea_id=$2 WHERE id=$1`,
		"membership":        `DELETE FROM collection_qr_items WHERE qr_id=$1 AND $2::uuid IS NOT NULL`,
		mergeHistoryCase:    `INSERT INTO consumptions(user_id,ts,tea_id) SELECT user_id,now(),$2 FROM collections WHERE id=$1`,
	}
	for name, statement := range changes {
		t.Run(name, func(t *testing.T) {
			f := newMergeFixture(t)
			preview := mergePreview(t, f)

			id := f.labelID
			if name == mergeHistoryCase {
				id = f.collectionID
			}

			if name == mergeQRMetadataCase {
				_, err := f.db.ExecContext(t.Context(), statement, id)
				require.NoError(t, err)
			} else {
				_, err := f.db.ExecContext(t.Context(), statement, id, f.targetID)
				require.NoError(t, err)
			}

			require.Equal(t, preview.SourceTea.Revision, agentRevision(t, f.roleFixture, agentTeaTable, f.teaID))
			require.Equal(t, preview.TargetTea.Revision, agentRevision(t, f.roleFixture, agentTeaTable, f.targetID))
			reply := imageGraphQLCall(t, f.server, f.adminToken, mergeMutation, mergeArguments(f, preview, uuid.NewString()))
			require.Len(t, reply.Errors, 1)
			require.Equal(t, mergePreviewConflictCode, reply.Errors[0].Extensions["code"])
			require.NotEqual(t, preview.PreviewToken, mergePreview(t, f).PreviewToken)
		})
	}
}

func TestTeaMergeBlocksImageOverflowAndConsumptionCollision(t *testing.T) {
	for _, reason := range []string{common.TeaMergeImageLimit, common.TeaMergeConsumptionCollision} {
		t.Run(reason, func(t *testing.T) {
			f := newMergeFixture(t)

			if reason == common.TeaMergeImageLimit {
				for index := range common.MaxTeaImages + 1 {
					teaID := f.teaID
					if index >= common.MaxTeaImages {
						teaID = f.targetID
					}

					_, err := f.db.ExecContext(t.Context(), `INSERT INTO tea_images(id,tea_id,content_type,width,height,data,is_cover) VALUES($1,$2,'image/png',1,1,'bytes',false)`, uuid.New(), teaID)
					require.NoError(t, err)
				}
			} else {
				_, err := f.db.ExecContext(t.Context(), `INSERT INTO consumptions(user_id,ts,tea_id) VALUES($1,'2026-10-12T08:00:00Z',$2),($1,'2026-10-12T08:00:00Z',$3)`, f.ownerID, f.teaID, f.targetID)
				require.NoError(t, err)
			}

			preview := mergePreview(t, f)
			require.Contains(t, preview.Blockers, reason)
			reply := imageGraphQLCall(t, f.server, f.adminToken, mergeMutation, mergeArguments(f, preview, uuid.NewString()))
			require.Len(t, reply.Errors, 1)
			require.Equal(t, mergeBlockedCode, reply.Errors[0].Extensions["code"])
			require.Equal(t, preview.SourceTea.Revision, agentRevision(t, f.roleFixture, agentTeaTable, f.teaID))
		})
	}
}

func TestTeaMergeReplayRecoversHistoricalResultAfterEditAndDeletion(t *testing.T) {
	f := newMergeFixture(t)
	mergeSeedTagsImagesHistory(t, f)
	preview := mergePreview(t, f)
	key := uuid.NewString()
	arguments := mergeArguments(f, preview, key)
	first := mergeCommit(t, f, arguments)
	_, err := f.db.ExecContext(t.Context(), `UPDATE teas SET description=$2 WHERE id=$1`, f.targetID, mergeUpdatedDescription)
	require.NoError(t, err)
	replay := mergeCommit(t, f, arguments)
	require.True(t, replay.Replayed)
	require.Equal(t, mergeUpdatedDescription, replay.TargetTea.Description)
	require.Equal(t, first.Receipt, replay.Receipt)
	require.Equal(t, first.ConsumptionsMoved, replay.ConsumptionsMoved)

	mutated := mergeArguments(f, preview, key)
	mutated[mergeTokenVar] = strings.Repeat("0", 64)
	bad := imageGraphQLCall(t, f.server, f.adminToken, mergeMutation, mutated)
	require.Len(t, bad.Errors, 1)
	require.Equal(t, agentIdempotencyConflictCode, bad.Errors[0].Extensions["code"])
	_, err = f.db.ExecContext(t.Context(), mergeDeleteTeaSQL, f.targetID)
	require.NoError(t, err)
	replay = mergeCommit(t, f, arguments)
	require.Nil(t, replay.TargetTea)
	require.Equal(t, first.Receipt, replay.Receipt)
	require.Equal(t, first.ImagesMoved, replay.ImagesMoved)
	receipt := imageGraphQLCall(t, f.server, f.adminToken, mergeReceiptQuery, arguments)
	require.Empty(t, receipt.Errors)

	var recovered mergeResultWire
	require.NoError(t, json.Unmarshal(receipt.Data["editorTeaMergeReceipt"], &recovered))
	require.Equal(t, first.Receipt, recovered.Receipt)
	require.Nil(t, recovered.TargetTea)
}

func TestTeaMergeLateFailureRollsBackEveryTransfer(t *testing.T) {
	f := newMergeFixture(t)
	images := mergeSeedTagsImagesHistory(t, f)
	preview := mergePreview(t, f)
	_, err := f.db.ExecContext(t.Context(), `CREATE FUNCTION reject_merge_receipt() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'test rejects merge receipt'; END $$ LANGUAGE plpgsql; CREATE TRIGGER reject_merge_receipt BEFORE INSERT ON tea_merge_receipt_details FOR EACH ROW EXECUTE FUNCTION reject_merge_receipt()`)
	require.NoError(t, err)

	key := uuid.NewString()
	reply := imageGraphQLCall(t, f.server, f.adminToken, mergeMutation, mergeArguments(f, preview, key))
	require.NotEmpty(t, reply.Errors)
	require.Equal(t, preview.PreviewToken, mergePreview(t, f).PreviewToken)

	var (
		teaID uuid.UUID
		count int
	)

	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT tea_id FROM qr_records WHERE id=$1`, f.labelID).Scan(&teaID))
	require.Equal(t, f.teaID, teaID)
	require.NoError(t, f.db.QueryRowContext(t.Context(), mergeImageParentSQL, images[0]).Scan(&teaID))
	require.Equal(t, f.teaID, teaID)
	require.NoError(t, f.db.QueryRowContext(t.Context(), mergeCountReceiptSQL, key).Scan(&count))
	require.Zero(t, count)
}

func TestTeaMergeRequiresAdminAndReportsUnsupportedMigration(t *testing.T) {
	f := newMergeFixture(t)

	arguments := mergeArguments(f, mergePreview(t, f), uuid.NewString())
	for _, token := range []string{"", f.ownerToken, f.otherToken} {
		expected := "FORBIDDEN"
		if token == "" {
			expected = "UNAUTHENTICATED"
		}

		for _, query := range []string{mergePreviewQuery, mergeMutation, mergeReceiptQuery} {
			reply := imageGraphQLCall(t, f.server, token, query, arguments)
			require.Len(t, reply.Errors, 1)
			require.Equal(t, expected, reply.Errors[0].Extensions["code"])
		}
	}

	missing := imageGraphQLCall(t, f.server, f.adminToken, mergeReceiptQuery, mergeArguments(f, mergePreview(t, f), uuid.NewString()))
	require.Empty(t, missing.Errors)
	require.JSONEq(t, mergeNullJSON, string(missing.Data["editorTeaMergeReceipt"]))
	_, err := f.db.ExecContext(t.Context(), `DROP TABLE tea_merge_receipt_details`)
	require.NoError(t, err)
	capReply := imageGraphQLCall(t, f.server, f.adminToken, `query{editorCapabilities{supportsRevisions supportsIdempotency supportsTeaMerge}}`, nil)
	require.Empty(t, capReply.Errors)
	require.Contains(t, string(capReply.Data["editorCapabilities"]), `"supportsTeaMerge":false`)
	reply := imageGraphQLCall(t, f.server, f.adminToken, mergeMutation, arguments)
	require.Len(t, reply.Errors, 1)
	require.Equal(t, "UNSUPPORTED", reply.Errors[0].Extensions["code"])
}

func TestTeaMergeMigrationIsRepeatableAndKeepsHistoricalReceipts(t *testing.T) {
	f := newMergeFixture(t)
	source, err := os.ReadFile("../../../../db/migrations/20261012_tea_merge.sql")
	require.NoError(t, err)

	key := uuid.New()
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO catalog_operation_receipts(operation_key,actor,operation,request_hash,tea_id,resource_id,result_revision) VALUES($1,'admin','newTea','hash',$2,$2,'1')`, key, f.teaID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `DROP TABLE tea_merge_receipt_details; ALTER TABLE catalog_operation_receipts DROP CONSTRAINT catalog_operation_receipts_operation_check; ALTER TABLE catalog_operation_receipts ADD CONSTRAINT catalog_operation_receipts_operation_check CHECK(operation IN ('newTea','addTeaImage'))`)
	require.NoError(t, err)

	for range 2 {
		_, err = f.db.ExecContext(t.Context(), string(source))
		require.NoError(t, err)
	}

	store := pgadapter.NewDB(f.db, logrus.NewEntry(logrus.New()))
	receipt, err := store.CatalogueOperationReceipt(t.Context(), key)
	require.NoError(t, err)
	require.Equal(t, roleNewTea, receipt.Operation)
	require.Equal(t, f.teaID, receipt.TeaID)
	require.Equal(t, 1, mergeCommit(t, f, mergeArguments(f, mergePreview(t, f), uuid.NewString())).QRCount)
}

func TestTeaMergeRefreshesCachedTeaOfDayWithoutConsumption(t *testing.T) {
	f := newMergeFixture(t)
	date := time.Now()
	f.resolver.consumption = consumption.NewPGStore(f.db, 0)
	f.resolver.todCache.Set(f.ownerID, &model.TeaOfTheDay{Date: date, Tea: &model.QRRecord{ID: gqlCommon.ID(f.labelID), Tea: &model.Tea{ID: gqlCommon.ID(f.teaID), Name: "Old name"}}}, date)
	mergeCommit(t, f, mergeArguments(f, mergePreview(t, f), uuid.NewString()))
	value, err := f.resolver.Query().TeaOfTheDay(roleContext(t, f.auth, f.ownerToken))
	require.NoError(t, err)
	require.Equal(t, f.labelID, uuid.UUID(value.Tea.ID))
	require.Equal(t, f.targetID, uuid.UUID(value.Tea.Tea.ID))
	require.Equal(t, mergeCanonicalName, value.Tea.Tea.Name)
	require.Equal(t, date, value.Date)

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM consumptions`).Scan(&count))
	require.Zero(t, count)
}

func TestTeaMergeConcurrentSameKeyCommitsOnce(t *testing.T) {
	f := newMergeFixture(t)
	arguments := mergeArguments(f, mergePreview(t, f), uuid.NewString())
	start := make(chan struct{})
	replies := make(chan agentCallResult, 4)

	for range 4 {
		go func() {
			<-start

			replies <- concurrentAgentGraphQLCall(t.Context(), f.server, f.adminToken, mergeMutation, arguments)
		}()
	}

	close(start)

	fresh := 0

	for range 4 {
		response := <-replies
		require.NoError(t, response.Err)
		require.Empty(t, response.Reply.Errors)

		var result mergeResultWire
		require.NoError(t, json.Unmarshal(response.Reply.Data[mergeMutationField], &result))

		if !result.Replayed {
			fresh++
		}
	}

	require.Equal(t, 1, fresh)

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM tea_merge_receipt_details`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestTeaMergeWaitsForLegacyQRWriteBeforeLockingTea(t *testing.T) {
	f := newMergeFixture(t)
	preview := mergePreview(t, f)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	tx, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)

	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `UPDATE qr_records SET boiling_temp=98 WHERE id=$1`, f.labelID)
	require.NoError(t, err)

	replies := make(chan agentCallResult, 1)
	go func() {
		replies <- concurrentAgentGraphQLCall(ctx, f.server, f.adminToken, mergeMutation, mergeArguments(f, preview, uuid.NewString()))
	}()
	// A foreign-key-dependent write can finish while merge is waiting for table locks.
	_, err = tx.ExecContext(ctx, `INSERT INTO qr_records(id,tea_id,boiling_temp,expiration_date) VALUES($1,$2,90,now())`, uuid.New(), f.teaID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	response := <-replies
	require.NoError(t, response.Err)
	require.Len(t, response.Reply.Errors, 1)
	require.Equal(t, mergePreviewConflictCode, response.Reply.Errors[0].Extensions["code"])
	require.Equal(t, 2, mergePreview(t, f).QRCount)
}

func TestTeaMergePreservesSourceCoverWhenTargetGalleryIsEmpty(t *testing.T) {
	f := newMergeFixture(t)
	imageID := uuid.New()
	stamp := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO tea_images(id,tea_id,content_type,width,height,data,created_at,is_cover) VALUES($1,$2,'image/png',2,3,'immutable-original',$3,true)`, imageID, f.teaID, stamp)
	require.NoError(t, err)
	result := mergeCommit(t, f, mergeArguments(f, mergePreview(t, f), uuid.NewString()))
	require.Len(t, result.TargetTea.Images, 1)
	require.Equal(t, imageID, result.TargetTea.Images[0].ID)
	require.True(t, result.TargetTea.Images[0].IsCover)

	var (
		createdAt     time.Time
		data          []byte
		width, height int
	)
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT created_at,data,width,height FROM tea_images WHERE id=$1`, imageID).Scan(&createdAt, &data, &width, &height))
	require.Equal(t, stamp, createdAt.UTC())
	require.Equal(t, []byte("immutable-original"), data)
	require.Equal(t, 2, width)
	require.Equal(t, 3, height)
}

func TestTeaMergeRejectsEitherStaleTeaRevisionAndInvalidArguments(t *testing.T) {
	for _, sourceChanged := range []bool{true, false} {
		t.Run(fmt.Sprintf("source_changed=%t", sourceChanged), func(t *testing.T) {
			f := newMergeFixture(t)
			preview := mergePreview(t, f)

			id := f.targetID
			if sourceChanged {
				id = f.teaID
			}

			_, err := f.db.ExecContext(t.Context(), `UPDATE teas SET description='Parallel edit' WHERE id=$1`, id)
			require.NoError(t, err)
			reply := imageGraphQLCall(t, f.server, f.adminToken, mergeMutation, mergeArguments(f, preview, uuid.NewString()))
			require.Len(t, reply.Errors, 1)
			require.Equal(t, agentConflictCode, reply.Errors[0].Extensions["code"])
			require.Equal(t, "2", reply.Errors[0].Extensions["currentRevision"])
		})
	}

	f := newMergeFixture(t)

	preview := mergePreview(t, f)
	for _, field := range []string{mergeSourceVar, "sr", "tr", mergeTokenVar, agentKeyVariable} {
		arguments := mergeArguments(f, preview, uuid.NewString())

		switch field {
		case mergeSourceVar:
			arguments[field] = f.targetID.String()
		case "sr", "tr":
			arguments[field] = "0"
		case mergeTokenVar:
			arguments[field] = "bad"
		case agentKeyVariable:
			arguments[field] = uuid.Nil.String()
		}

		reply := imageGraphQLCall(t, f.server, f.adminToken, mergeMutation, arguments)
		require.Len(t, reply.Errors, 1)
		require.Equal(t, badUserInputCode, reply.Errors[0].Extensions["code"])
	}

	require.Equal(t, preview.PreviewToken, mergePreview(t, f).PreviewToken)
}

func TestTeaMergePreservesHistoricalCreationAndPhotoReceipts(t *testing.T) {
	f := newMergeFixture(t)
	createKey, photoKey := uuid.NewString(), uuid.NewString()
	create := `mutation($key:String!){newTea(tea:{name:"Old duplicate",type:tea,description:"Original"},idempotencyKey:$key){id revision}}`
	created := imageGraphQLCall(t, f.server, f.ownerToken, create, map[string]any{agentKeyVariable: createKey})
	require.Empty(t, created.Errors)

	var source mergeTeaWire
	require.NoError(t, json.Unmarshal(created.Data[roleNewTea], &source))
	f.teaID = source.ID
	upload := `mutation($tea:ID!,$image:String!,$key:String!){addTeaImage(teaID:$tea,imageBase64:$image,idempotencyKey:$key){images{id}}}`
	photoArgs := map[string]any{testTeaVariable: f.teaID.String(), testImageVariable: imageFixtureBase64(t), agentKeyVariable: photoKey}
	uploaded := imageGraphQLCall(t, f.server, f.ownerToken, upload, photoArgs)
	require.Empty(t, uploaded.Errors)

	var gallery mergeTeaWire
	require.NoError(t, json.Unmarshal(uploaded.Data[testAddImageField], &gallery))
	require.Len(t, gallery.Images, 1)

	var before string
	require.NoError(t, f.db.QueryRowContext(t.Context(), mergeHistoricalReceiptsSQL, createKey, photoKey).Scan(&before))
	mergeCommit(t, f, mergeArguments(f, mergePreview(t, f), uuid.NewString()))
	lateCreate := imageGraphQLCall(t, f.server, f.ownerToken, create, map[string]any{agentKeyVariable: createKey})
	require.Len(t, lateCreate.Errors, 1)
	require.Equal(t, roleNotFoundCode, lateCreate.Errors[0].Extensions["code"])
	latePhoto := imageGraphQLCall(t, f.server, f.ownerToken, upload, photoArgs)
	require.Len(t, latePhoto.Errors, 1)
	require.Equal(t, roleNotFoundCode, latePhoto.Errors[0].Extensions["code"])

	var after string
	require.NoError(t, f.db.QueryRowContext(t.Context(), mergeHistoricalReceiptsSQL, createKey, photoKey).Scan(&after))
	require.Equal(t, before, after)

	var parent uuid.UUID
	require.NoError(t, f.db.QueryRowContext(t.Context(), mergeImageParentSQL, gallery.Images[0].ID).Scan(&parent))
	require.Equal(t, f.targetID, parent)
	nonMerge := imageGraphQLCall(t, f.server, f.adminToken, mergeReceiptQuery, mergeArguments(f, mergePreviewWire{SourceTea: &mergeTeaWire{Revision: "1"}, TargetTea: &mergeTeaWire{Revision: "1"}, PreviewToken: strings.Repeat("0", 64)}, createKey))
	require.Len(t, nonMerge.Errors, 1)
	require.Equal(t, agentIdempotencyConflictCode, nonMerge.Errors[0].Extensions["code"])
}

func TestTeaMergePublishesCommittedUpdateAndRemovalOnlyOnce(t *testing.T) {
	f := newMergeFixture(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	updates, err := f.resolver.teaData.SubscribeOnUpdate(ctx)
	require.NoError(t, err)
	deletes, err := f.resolver.teaData.SubscribeOnDelete(ctx)
	require.NoError(t, err)

	starter, ok := f.resolver.teaData.(interface{ Start() })
	require.True(t, ok)
	starter.Start()

	arguments := mergeArguments(f, mergePreview(t, f), uuid.NewString())

	replies := make(chan agentCallResult, 1)
	go func() { replies <- concurrentAgentGraphQLCall(ctx, f.server, f.adminToken, mergeMutation, arguments) }()

	select {
	case record := <-updates:
		require.Equal(t, f.targetID, uuid.UUID(record.ID))
	case <-time.After(time.Second):
		t.Fatal("missing committed target update")
	}

	select {
	case id := <-deletes:
		require.Equal(t, f.teaID, uuid.UUID(id))
	case <-time.After(time.Second):
		t.Fatal("missing committed source removal")
	}

	response := <-replies
	require.NoError(t, response.Err)
	require.Empty(t, response.Reply.Errors)
	mergeCommit(t, f, arguments)

	select {
	case <-updates:
		t.Fatal("replay published a second update")
	case <-deletes:
		t.Fatal("replay published a second removal")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestTeaMergeImportedGalleriesPreviewExactOldestCover(t *testing.T) {
	for _, oldestOnSource := range []bool{true, false} {
		t.Run("imported", func(t *testing.T) {
			f := newMergeFixture(t)
			imageID, otherID := uuid.New(), uuid.New()

			oldestTea, newerTea := f.teaID, f.targetID
			if !oldestOnSource {
				oldestTea, newerTea = newerTea, oldestTea
			}

			_, err := f.db.ExecContext(t.Context(), `INSERT INTO tea_images(id,tea_id,content_type,width,height,data,created_at,is_cover) VALUES($1,$2,'image/png',1,1,'bytes','2020-01-01T00:00:00Z',false),($3,$4,'image/png',1,1,'bytes','2021-01-01T00:00:00Z',false)`, imageID, oldestTea, otherID, newerTea)
			require.NoError(t, err)
			preview := mergePreview(t, f)
			require.NotNil(t, preview.ResultingCoverImageID)
			require.Equal(t, imageID, *preview.ResultingCoverImageID)
			result := mergeCommit(t, f, mergeArguments(f, preview, uuid.NewString()))

			var coverID uuid.UUID
			require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT id FROM tea_images WHERE tea_id=$1 AND is_cover`, f.targetID).Scan(&coverID))
			require.Equal(t, *preview.ResultingCoverImageID, coverID)
			require.Len(t, result.TargetTea.Images, 2)
		})
	}
}

type delayedMergeAdviser struct{ ready, resume chan struct{} }

func (a delayedMergeAdviser) RecommendTea(context.Context, []common.Tea, common.Weather, string) (string, error) {
	return "", nil
}
func (a delayedMergeAdviser) RecommendTeaStream(context.Context, []common.Tea, common.Weather, string, chan<- string) error {
	return nil
}
func (a delayedMergeAdviser) ContextScores(ctx context.Context, _ []string, _ common.Weather, _ time.Weekday) (map[string]int, error) {
	close(a.ready)

	select {
	case <-a.resume:
		return map[string]int{}, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for delayed scoring: %w", ctx.Err())
	}
}

type mergeWeather struct{}

func (mergeWeather) CurrentCyprus(context.Context) (common.Weather, error) {
	return common.Weather{}, nil
}

func TestTeaMergeDuringScoringReturnsCurrentQRAndRecordsTargetOnce(t *testing.T) {
	f := newMergeFixture(t)
	delayed := delayedMergeAdviser{ready: make(chan struct{}), resume: make(chan struct{})}
	f.resolver.adviser = delayed
	f.resolver.weather = mergeWeather{}
	f.resolver.consumption = consumption.NewPGStore(f.db, 0)

	ctx, cancel := context.WithTimeout(roleContext(t, f.auth, f.ownerToken), 5*time.Second)
	defer cancel()

	type selectionReply struct {
		selection *model.TeaOfTheDay
		err       error
	}

	replies := make(chan selectionReply, 1)

	go func() {
		selection, err := f.resolver.Query().TeaOfTheDay(ctx)
		replies <- selectionReply{selection, err}
	}()

	select {
	case <-delayed.ready:
	case <-ctx.Done():
		t.Fatal("scoring was not reached")
	}

	mergeCommit(t, f, mergeArguments(f, mergePreview(t, f), uuid.NewString()))
	close(delayed.resume)

	response := <-replies
	require.NoError(t, response.err)
	require.Equal(t, f.targetID, uuid.UUID(response.selection.Tea.Tea.ID))
	require.Equal(t, f.labelID, uuid.UUID(response.selection.Tea.ID))

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), mergeCountUserHistorySQL, f.targetID, f.ownerID).Scan(&count))
	require.Equal(t, 1, count)

	cached, err := f.resolver.Query().TeaOfTheDay(ctx)
	require.NoError(t, err)
	require.Equal(t, f.targetID, uuid.UUID(cached.Tea.Tea.ID))
	require.NoError(t, f.db.QueryRowContext(t.Context(), mergeCountUserHistorySQL, f.targetID, f.ownerID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestTeaMergeReceiptAuthenticatesEveryOriginalArgument(t *testing.T) {
	f := newMergeFixture(t)
	preview := mergePreview(t, f)
	arguments := mergeArguments(f, preview, uuid.NewString())

	first := mergeCommit(t, f, arguments)
	for _, field := range []string{mergeSourceVar, mergeTargetVar, "sr", "tr", mergeTokenVar} {
		changed := mergeArguments(f, preview, first.Receipt.IdempotencyKey)

		switch field {
		case mergeSourceVar, mergeTargetVar:
			changed[field] = uuid.NewString()
		case "sr", "tr":
			changed[field] = "2"
		case mergeTokenVar:
			changed[field] = strings.Repeat("0", 64)
		}

		response := imageGraphQLCall(t, f.server, f.adminToken, mergeReceiptQuery, changed)
		require.Len(t, response.Errors, 1)
		require.Equal(t, agentIdempotencyConflictCode, response.Errors[0].Extensions["code"])
	}

	_, err := f.db.ExecContext(t.Context(), mergeDeleteTeaSQL, f.targetID)
	require.NoError(t, err)
	response := imageGraphQLCall(t, f.server, f.adminToken, mergeReceiptQuery, arguments)
	require.Empty(t, response.Errors)

	var result mergeResultWire
	require.NoError(t, json.Unmarshal(response.Data["editorTeaMergeReceipt"], &result))
	require.Nil(t, result.TargetTea)
	require.Equal(t, first.Receipt, result.Receipt)
	require.Equal(t, first.QRCount, result.QRCount)
}

func TestTeaMergeSelectionWaitsForQRRebindAndRecordsLatestTea(t *testing.T) {
	f := newMergeFixture(t)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	tx, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)

	defer tx.Rollback()

	var writerPID int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&writerPID))
	_, err = tx.ExecContext(ctx, `UPDATE qr_records SET tea_id=$2,boiling_temp=92 WHERE id=$1`, f.labelID, f.targetID)
	require.NoError(t, err)

	type recordReply struct {
		record *common.CollectionRecord
		err    error
	}

	replies := make(chan recordReply, 1)
	stamp := time.Now().UTC()

	store := consumption.NewPGStore(f.db, 0)
	go func() {
		record, err := store.RecordQR(ctx, f.ownerID, f.labelID, stamp)
		replies <- recordReply{record, err}
	}()

	require.Eventually(t, func() bool {
		var waiting bool

		queryErr := f.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND $1=ANY(pg_blocking_pids(pid)))`, writerPID).Scan(&waiting)

		return queryErr == nil && waiting
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, tx.Commit())

	response := <-replies
	require.NoError(t, response.err)
	require.Equal(t, f.targetID, response.record.Tea.ID)
	require.Equal(t, 92, response.record.BowlingTemp)

	var recordedTea uuid.UUID
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT tea_id FROM consumptions WHERE user_id=$1 AND ts=$2`, f.ownerID, stamp).Scan(&recordedTea))
	require.Equal(t, f.targetID, recordedTea)
}
