package graphql

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/internal/teaimages"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
	pgadapter "github.com/teaelephant/TeaElephantMemory/pkg/pg"
)

const initialTagMutation = `mutation($tea:TeaData!,$tags:[ID!]!) { newTea(tea:$tea,tagIDs:$tags) { id tags { id } } }`

func TestTeaCreationAcceptsInitialExistingTagsAtomically(t *testing.T) {
	for _, direct := range []bool{false, true} {
		for _, role := range []string{testMobileUserRole, testAdminRole, testAnonymousRole} {
			for _, invalid := range []uuid.UUID{uuid.New(), uuid.Nil} {
				for _, valid := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/valid=%t/invalid=%s/direct=%t", role, valid, invalid, direct), func(t *testing.T) {
						checkInitialTagCreation(t, direct, role, valid, invalid)
					})
				}
			}
		}
	}
}

func checkInitialTagCreation(t *testing.T, direct bool, role string, valid bool, invalid uuid.UUID) {
	t.Helper()

	f := newRoleFixture(t)
	secondID := seedInitialTagResources(t, f)
	before := catalogState(t, f.db)
	existingTea := existingTeaState(t, f.db, f.teaID)
	definitions := initialTagDefinitions(t, f)

	tagIDs := []uuid.UUID{f.tagID, secondID, f.tagID}
	if !valid {
		tagIDs = append(tagIDs, invalid)
	}

	token := map[string]string{testAnonymousRole: "", testMobileUserRole: f.ownerToken, testAdminRole: f.adminToken}[role]

	id, err := createTeaWithInitialTags(t, f, direct, token, tagIDs)
	if role == testAnonymousRole || !valid {
		assertInitialTagCreationDenied(t, err, role)
		require.Equal(t, before, catalogState(t, f.db), "refused creation must leave no Tea or initial tag links")

		return
	}

	require.NoError(t, err)
	assertInitialTeaPreservedResources(t, f, id, []uuid.UUID{f.tagID, secondID}, existingTea, definitions)
	assertUserCannotAddTagAfterCreation(t, f, direct, id)
}

func seedInitialTagResources(t *testing.T, f roleFixture) uuid.UUID {
	t.Helper()

	store := pgadapter.NewDB(f.db, logrus.NewEntry(logrus.New()))
	images := teaimages.NewService(store)
	f.resolver.images = images
	require.NoError(t, images.Add(t.Context(), f.teaID, imageFixtureBase64(t)))
	require.NoError(t, store.AddTagToTea(t.Context(), f.teaID, f.tagID))
	second, err := store.CreateTag(t.Context(), "Earthy", "brown", f.categoryID)
	require.NoError(t, err)

	return second.ID
}

func createTeaWithInitialTags(t *testing.T, f roleFixture, direct bool, token string, tagIDs []uuid.UUID) (uuid.UUID, error) {
	t.Helper()

	data := model.TeaData{Name: testSharedTeaName, Type: model.TypeTea, Description: roleMutationData}

	if direct {
		ids := make([]gqlCommon.ID, len(tagIDs))
		for i, id := range tagIDs {
			ids[i] = gqlCommon.ID(id)
		}

		created, err := f.resolver.Mutation().NewTea(roleContext(t, f.auth, token), data, ids, nil)
		if err != nil {
			return uuid.Nil, fmt.Errorf("create tea with tags: %w", err)
		}

		return uuid.UUID(created.ID), nil
	}

	vars := map[string]any{testTeaVariable: map[string]any{roleNameVariable: data.Name, roleTypeVariable: data.Type.String(), roleDescriptionVariable: data.Description}, roleTagsVariable: tagIDs}

	reply := imageGraphQLCall(t, f.server, token, initialTagMutation, vars)
	if len(reply.Errors) > 0 {
		require.Len(t, reply.Errors, 1)

		return uuid.Nil, &gqlerror.Error{Message: reply.Errors[0].Message, Extensions: reply.Errors[0].Extensions}
	}

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(reply.Data[roleNewTea], &created))

	id, err := uuid.Parse(created.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse created tea identity: %w", err)
	}

	return id, nil
}

func assertInitialTagCreationDenied(t *testing.T, err error, role string) {
	t.Helper()

	var gqlErr *gqlerror.Error
	require.ErrorAs(t, err, &gqlErr)

	code := roleNotFoundCode
	if role == testAnonymousRole {
		code = testImageUnauthorizedCode
	}

	require.Equal(t, code, gqlErr.Extensions["code"])
}

func initialTagDefinitions(t *testing.T, f roleFixture) string {
	t.Helper()

	var state string
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT jsonb_build_object(
        'categories',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM tag_categories t),
        'tags',(SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM tags t))::text`).Scan(&state))

	return state
}

func assertInitialTeaPreservedResources(t *testing.T, f roleFixture, id uuid.UUID, tagIDs []uuid.UUID, previousTea, previousDefinitions string) {
	t.Helper()

	require.NotEqual(t, f.teaID, id)
	require.Equal(t, previousTea, existingTeaState(t, f.db, f.teaID), "creation must preserve existing same-name Tea and gallery")
	require.Equal(t, previousDefinitions, initialTagDefinitions(t, f), "creation must preserve shared tag definitions")

	query := `query($tea:ID!) { tea(id:$tea) { id name description tags { id } images { id } } }`
	reply := imageGraphQLCall(t, f.server, "", query, map[string]any{testTeaVariable: id})
	require.Empty(t, reply.Errors)

	var created struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Tags        []struct {
			ID uuid.UUID `json:"id"`
		} `json:"tags"`
		Images []struct {
			ID uuid.UUID `json:"id"`
		} `json:"images"`
	}
	require.NoError(t, json.Unmarshal(reply.Data["tea"], &created))
	require.Equal(t, testSharedTeaName, created.Name)
	require.Equal(t, roleMutationData, created.Description)
	require.Empty(t, created.Images)

	actualTags := make([]uuid.UUID, len(created.Tags))
	for i, item := range created.Tags {
		actualTags[i] = item.ID
	}

	require.ElementsMatch(t, tagIDs, actualTags, "initial tags must be deduplicated and readable after commit")

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM tea_tags WHERE tea_id=$1`, id).Scan(&count))
	require.Equal(t, len(tagIDs), count)
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM tea_tags WHERE tea_id=$1 AND tag_id=$2`, f.teaID, f.tagID).Scan(&count))
	require.Equal(t, 1, count, "existing tag associations must survive new Tea creation")
}

func assertUserCannotAddTagAfterCreation(t *testing.T, f roleFixture, direct bool, id uuid.UUID) {
	t.Helper()

	before := catalogState(t, f.db)
	if direct {
		_, err := f.resolver.Mutation().AddTagToTea(roleContext(t, f.auth, f.ownerToken), gqlCommon.ID(id), gqlCommon.ID(f.tagID), nil)

		var gqlErr *gqlerror.Error
		require.ErrorAs(t, err, &gqlErr)
		require.Equal(t, testImageForbiddenCode, gqlErr.Extensions["code"])
	} else {
		vars := map[string]any{testTeaVariable: id, roleTagVariable: f.tagID}
		assertRoleDenied(t, imageGraphQLCall(t, f.server, f.ownerToken, catalogMutation(roleAddTag), vars), testImageForbiddenCode)
	}

	require.Equal(t, before, catalogState(t, f.db), "creation must not grant a window of ownership over an existing Tea")
}

func TestTeaCreationRollsBackInitialTagInsertAndCommitFailures(t *testing.T) {
	for _, direct := range []bool{false, true} {
		for _, atCommit := range []bool{false, true} {
			t.Run(fmt.Sprintf("commit=%t/direct=%t", atCommit, direct), func(t *testing.T) {
				f := newRoleFixture(t)
				secondID := seedInitialTagResources(t, f)
				before := catalogState(t, f.db)
				installInitialTagFailure(t, f, atCommit)
				_, err := createTeaWithInitialTags(t, f, direct, f.ownerToken, []uuid.UUID{secondID, f.tagID})
				require.Error(t, err)
				require.Equal(t, before, catalogState(t, f.db), "failed link insert or commit must roll back Tea and every initial association")
			})
		}
	}
}

func installInitialTagFailure(t *testing.T, f roleFixture, atCommit bool) {
	t.Helper()

	statement := fmt.Sprintf(`CREATE FUNCTION reject_initial_tag() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
        IF NEW.tag_id = '%s'::uuid THEN RAISE EXCEPTION 'test initial tag failure'; END IF; RETURN NEW; END $$`, f.tagID)
	_, err := f.db.ExecContext(t.Context(), statement)
	require.NoError(t, err)

	trigger := `CREATE TRIGGER reject_initial_tag BEFORE INSERT ON tea_tags FOR EACH ROW EXECUTE PROCEDURE reject_initial_tag()`
	if atCommit {
		trigger = `CREATE CONSTRAINT TRIGGER reject_initial_tag AFTER INSERT ON tea_tags DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE PROCEDURE reject_initial_tag()`
	}

	_, err = f.db.ExecContext(t.Context(), trigger)
	require.NoError(t, err)
}

func TestTeaCreationRejectsMalformedInitialTagIDWithoutWrites(t *testing.T) {
	f := newRoleFixture(t)
	before := catalogState(t, f.db)
	vars := map[string]any{testTeaVariable: map[string]any{roleNameVariable: roleMutationData, roleTypeVariable: model.TypeTea.String()}, roleTagsVariable: []string{"invalid-tag-id"}}
	reply := imageGraphQLCall(t, f.server, f.ownerToken, initialTagMutation, vars)
	require.NotEmpty(t, reply.Errors)
	require.Equal(t, before, catalogState(t, f.db))
}

func TestMissingInitialTagReportsDomainError(t *testing.T) {
	f := newRoleFixture(t)
	store := pgadapter.NewDB(f.db, logrus.NewEntry(logrus.New()))
	before := catalogState(t, f.db)
	_, err := store.WriteRecordWithTags(t.Context(), &common.TeaData{Name: roleMutationData, Type: common.TeaBeverageType}, []uuid.UUID{uuid.New()})
	require.ErrorIs(t, err, common.ErrTagNotFound)
	require.Equal(t, before, catalogState(t, f.db))
}
