package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/teaelephant/TeaElephantMemory/internal/teaimages"
	pgadapter "github.com/teaelephant/TeaElephantMemory/pkg/pg"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const agentUpdateCase = "update"
const agentMoveCase = "move"

const agentCreateCase = "create"
const agentCategoryName = "Aroma"
const agentCategoryVariable = "category"

const agentExpectedRevisionThree = `expectedRevision:"3"`

const agentVariablesKey = "variables"
const agentAuthorizationHeader = "Authorization"

const agentConflictCode = "CONFLICT"
const agentIdempotencyConflictCode = "IDEMPOTENCY_CONFLICT"
const agentKeyVariable = "key"
const agentTeaTable = "teas"
const agentCategoryTable = "tag_categories"
const agentImageCountQuery = `SELECT count(*) FROM tea_images WHERE tea_id=$1`
const agentTeaTagCountQuery = `SELECT count(*) FROM tea_tags WHERE tea_id=$1 AND tag_id=$2`
const agentMigrationPath = "../../../../db/migrations/20261012_catalog_agent_operations.sql"

func TestCatalogueRevisionProtectsConcurrentEditorWrites(t *testing.T) {
	f := newRoleFixture(t)
	query := `query($id:ID!) { tea(id:$id) { id name description revision } }`
	reply := imageGraphQLCall(t, f.server, f.adminToken, query, map[string]any{"id": f.teaID.String()})
	require.Empty(t, reply.Errors, "catalog reads must return a revision from the same record snapshot")

	mutation := `mutation($id:ID!) { updateTea(id:$id,expectedRevision:"1",tea:{name:"First",type:tea,description:"First editor"}) { revision name } }`
	first := imageGraphQLCall(t, f.server, f.adminToken, mutation, map[string]any{"id": f.teaID.String()})
	require.Empty(t, first.Errors)
	second := imageGraphQLCall(t, f.server, f.adminToken, mutation, map[string]any{"id": f.teaID.String()})
	require.Len(t, second.Errors, 1)
	require.Equal(t, agentConflictCode, second.Errors[0].Extensions["code"])
	require.Equal(t, "2", second.Errors[0].Extensions["currentRevision"])

	var name string
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT name FROM teas WHERE id=$1`, f.teaID).Scan(&name))
	require.Equal(t, "First", name)
}

func TestCatalogueConcurrentCompareAndSwapCommitsOnlyOneWriter(t *testing.T) {
	f := newRoleFixture(t)
	query := `mutation($id:ID!,$name:String!){updateTea(id:$id,expectedRevision:"1",tea:{name:$name,type:tea,description:"Concurrent"}){name revision}}`
	start := make(chan struct{})
	results := make(chan agentCallResult, 2)

	for _, name := range []string{"Editor A", "Editor B"} {
		go func() {
			<-start

			results <- concurrentAgentGraphQLCall(t.Context(), f.server, f.adminToken, query, map[string]any{"id": f.teaID.String(), "name": name})
		}()
	}

	close(start)

	success, conflict := 0, 0

	for range 2 {
		result := <-results
		require.NoError(t, result.Err)

		reply := result.Reply
		if len(reply.Errors) == 0 {
			success++
		} else {
			require.Equal(t, agentConflictCode, reply.Errors[0].Extensions["code"])

			conflict++
		}
	}

	require.Equal(t, 1, success)
	require.Equal(t, 1, conflict)

	var revision string
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT revision::text FROM teas WHERE id=$1`, f.teaID).Scan(&revision))
	require.Equal(t, "2", revision)
}

func TestCatalogueLegacyTagAndPhotoWritesInvalidateTeaRevision(t *testing.T) {
	f := newAgentFixture(t)
	addTag := `mutation($tea:ID!,$tag:ID!){addTagToTea(teaID:$tea,tagID:$tag,expectedRevision:"1"){revision tags{id revision}}}`
	reply := imageGraphQLCall(t, f.server, f.adminToken, addTag, map[string]any{testTeaVariable: f.teaID.String(), roleTagVariable: f.tagID.String()})
	require.Empty(t, reply.Errors)
	require.Equal(t, "2", agentRevision(t, f, agentTeaTable, f.teaID))

	upload := `mutation($tea:ID!,$image:String!){addTeaImage(teaID:$tea,imageBase64:$image){revision images{id}}}`
	reply = imageGraphQLCall(t, f.server, f.ownerToken, upload, map[string]any{testTeaVariable: f.teaID.String(), testImageVariable: imageFixtureBase64(t)})
	require.Empty(t, reply.Errors)
	require.Equal(t, "3", agentRevision(t, f, agentTeaTable, f.teaID))

	stale := `mutation($id:ID!){updateTea(id:$id,expectedRevision:"2",tea:{name:"Stale",type:tea,description:"Stale"}){revision}}`
	reply = imageGraphQLCall(t, f.server, f.adminToken, stale, map[string]any{"id": f.teaID.String()})
	require.Len(t, reply.Errors, 1)
	require.Equal(t, agentConflictCode, reply.Errors[0].Extensions["code"])

	legacy := `mutation($id:ID!){updateTea(id:$id,tea:{name:"Legacy Editor",type:tea,description:"Preserved"}){name revision}}`
	require.Empty(t, imageGraphQLCall(t, f.server, f.adminToken, legacy, map[string]any{"id": f.teaID.String()}).Errors)
	require.Equal(t, "4", agentRevision(t, f, agentTeaTable, f.teaID))

	remove := `mutation($tea:ID!,$tag:ID!){deleteTagFromTea(teaID:$tea,tagID:$tag,expectedRevision:"4"){revision}}`
	require.Empty(t, imageGraphQLCall(t, f.server, f.adminToken, remove, map[string]any{testTeaVariable: f.teaID.String(), roleTagVariable: f.tagID.String()}).Errors)
	require.Equal(t, "5", agentRevision(t, f, agentTeaTable, f.teaID))
}

func TestCatalogueCreationIdempotencyIsAtomicAndAuthorized(t *testing.T) {
	f := newAgentFixture(t)
	key := uuid.NewString()
	query := `mutation($key:String!,$tags:[ID!]!){newTea(tea:{name:"Agent Jasmine",type:tea,description:"Known source"},tagIDs:$tags,idempotencyKey:$key){id revision tags{id}}}`
	start := make(chan struct{})
	results := make(chan agentCallResult, 6)

	for range 6 {
		go func() {
			<-start

			results <- concurrentAgentGraphQLCall(t.Context(), f.server, f.adminToken, query, map[string]any{agentKeyVariable: key, roleTagsVariable: []string{f.tagID.String(), f.tagID.String()}})
		}()
	}

	close(start)

	id := ""

	for range 6 {
		result := <-results
		require.NoError(t, result.Err)
		reply := result.Reply
		require.Empty(t, reply.Errors)

		var record agentTeaRecord
		require.NoError(t, json.Unmarshal(reply.Data["newTea"], &record))

		if id == "" {
			id = record.ID
		} else {
			require.Equal(t, id, record.ID)
		}
	}

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM teas WHERE name='Agent Jasmine'`).Scan(&count))
	require.Equal(t, 1, count)

	changed := strings.Replace(query, "Known source", "Different source", 1)
	mismatch := imageGraphQLCall(t, f.server, f.adminToken, changed, map[string]any{agentKeyVariable: key, roleTagsVariable: []string{f.tagID.String()}})
	require.Len(t, mismatch.Errors, 1)
	require.Equal(t, agentIdempotencyConflictCode, mismatch.Errors[0].Extensions["code"])
	foreign := imageGraphQLCall(t, f.server, f.otherToken, query, map[string]any{agentKeyVariable: key, roleTagsVariable: []string{f.tagID.String()}})
	require.Len(t, foreign.Errors, 1)
	require.Equal(t, agentIdempotencyConflictCode, foreign.Errors[0].Extensions["code"])
	anonymous := imageGraphQLCall(t, f.server, "", query, map[string]any{agentKeyVariable: key, roleTagsVariable: []string{f.tagID.String()}})
	require.Len(t, anonymous.Errors, 1)
	require.Equal(t, testImageUnauthorizedCode, anonymous.Errors[0].Extensions["code"])
	receipt := imageGraphQLCall(t, f.server, f.adminToken, `query($key:String!){editorOperationReceipt(idempotencyKey:$key){operation teaID resourceID resultRevision completedAt}}`, map[string]any{agentKeyVariable: key})
	require.Empty(t, receipt.Errors)

	var result agentReceiptRecord
	require.NoError(t, json.Unmarshal(receipt.Data["editorOperationReceipt"], &result))
	require.Equal(t, "newTea", result.Operation)
	require.Equal(t, id, result.TeaID)
	require.Equal(t, id, result.ResourceID)
}

func TestCatalogueImageReplayRecoversDespiteStaleRevision(t *testing.T) {
	f := newAgentFixture(t)
	key := uuid.NewString()
	image := imageFixtureBase64(t)
	query := `mutation($tea:ID!,$image:String!,$key:String!){addTeaImage(teaID:$tea,imageBase64:$image,expectedRevision:"1",idempotencyKey:$key){revision images{id}}}`
	variables := map[string]any{testTeaVariable: f.teaID.String(), testImageVariable: image, agentKeyVariable: key}
	first := imageGraphQLCall(t, f.server, f.adminToken, query, variables)
	require.Empty(t, first.Errors)

	legacy := `mutation($tea:ID!,$image:String!){addTeaImage(teaID:$tea,imageBase64:$image){revision}}`
	require.Empty(t, imageGraphQLCall(t, f.server, f.ownerToken, legacy, map[string]any{testTeaVariable: f.teaID.String(), testImageVariable: image}).Errors)
	replay := imageGraphQLCall(t, f.server, f.adminToken, query, variables)
	require.Empty(t, replay.Errors)

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), agentImageCountQuery, f.teaID).Scan(&count))
	require.Equal(t, 2, count)
	require.Equal(t, "3", agentRevision(t, f, agentTeaTable, f.teaID))

	different := strings.Replace(query, `expectedRevision:"1"`, agentExpectedRevisionThree, 1)
	mismatch := imageGraphQLCall(t, f.server, f.adminToken, different, variables)
	require.Len(t, mismatch.Errors, 1)
	require.Equal(t, agentIdempotencyConflictCode, mismatch.Errors[0].Extensions["code"])

	unknownKey := uuid.NewString()
	variables[agentKeyVariable] = unknownKey
	stale := imageGraphQLCall(t, f.server, f.adminToken, query, variables)
	require.Len(t, stale.Errors, 1)
	require.Equal(t, agentConflictCode, stale.Errors[0].Extensions["code"])
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM catalog_operation_receipts WHERE operation_key=$1`, unknownKey).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, f.db.QueryRowContext(t.Context(), agentImageCountQuery, f.teaID).Scan(&count))
	require.Equal(t, 2, count)
}

func TestCatalogueAdministrativeCapabilitiesAndReceiptsAuthorizeBeforeLookup(t *testing.T) {
	f := newAgentFixture(t)
	capabilities := `query{editorCapabilities{protocolVersion supportsRevisions supportsIdempotency}}`
	reply := imageGraphQLCall(t, f.server, f.adminToken, capabilities, nil)
	require.Empty(t, reply.Errors)

	var result agentCapabilitiesRecord
	require.NoError(t, json.Unmarshal(reply.Data["editorCapabilities"], &result))
	require.Equal(t, "1", result.ProtocolVersion)
	require.True(t, result.SupportsRevisions)
	require.True(t, result.SupportsIdempotency)

	for _, query := range []string{capabilities, `query{editorOperationReceipt(idempotencyKey:"invalid"){resourceID}}`} {
		for token, code := range map[string]string{"": testImageUnauthorizedCode, f.ownerToken: testImageForbiddenCode} {
			reply := imageGraphQLCall(t, f.server, token, query, nil)
			require.Len(t, reply.Errors, 1)
			require.Equal(t, code, reply.Errors[0].Extensions["code"])
		}
	}

	missing := imageGraphQLCall(t, f.server, f.adminToken, `query($key:String!){editorOperationReceipt(idempotencyKey:$key){resourceID}}`, map[string]any{agentKeyVariable: uuid.NewString()})
	require.Empty(t, missing.Errors)
	require.Equal(t, "null", string(missing.Data["editorOperationReceipt"]))
	_, err := f.db.ExecContext(t.Context(), `DROP TABLE tea_merge_receipt_details; DROP TABLE catalog_operation_receipts`)
	require.NoError(t, err)
	absent := imageGraphQLCall(t, f.server, f.adminToken, capabilities, nil)
	require.Empty(t, absent.Errors)
	require.NoError(t, json.Unmarshal(absent.Data["editorCapabilities"], &result))
	require.False(t, result.SupportsRevisions)
	require.False(t, result.SupportsIdempotency)
}

func TestCatalogueTaxonomyRevisionsAndCategoryFailureAreAtomic(t *testing.T) {
	f := newAgentFixture(t)
	tagQuery := `mutation($tag:ID!){updateTag(id:$tag,name:"Jasmine",color:"green",expectedRevision:"1"){revision name}}`
	require.Empty(t, imageGraphQLCall(t, f.server, f.adminToken, tagQuery, map[string]any{roleTagVariable: f.tagID.String()}).Errors)
	require.Equal(t, "2", agentRevision(t, f, roleTagsVariable, f.tagID))
	conflict := imageGraphQLCall(t, f.server, f.adminToken, tagQuery, map[string]any{roleTagVariable: f.tagID.String()})
	require.Len(t, conflict.Errors, 1)
	require.Equal(t, agentConflictCode, conflict.Errors[0].Extensions["code"])

	add := `mutation($tea:ID!,$tag:ID!){addTagToTea(teaID:$tea,tagID:$tag){revision}}`
	require.Empty(t, imageGraphQLCall(t, f.server, f.adminToken, add, map[string]any{testTeaVariable: f.teaID.String(), roleTagVariable: f.tagID.String()}).Errors)

	categoryUpdate := `mutation($id:ID!){updateTagCategory(id:$id,name:"Flavor",expectedRevision:"1"){revision name}}`
	require.Empty(t, imageGraphQLCall(t, f.server, f.adminToken, categoryUpdate, map[string]any{"id": f.categoryID.String()}).Errors)
	require.Equal(t, "2", agentRevision(t, f, agentCategoryTable, f.categoryID))
	_, err := f.db.ExecContext(t.Context(), `CREATE FUNCTION reject_category_delete() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'test category delete failure'; END $$ LANGUAGE plpgsql; CREATE TRIGGER reject_category_delete BEFORE DELETE ON tag_categories FOR EACH ROW EXECUTE FUNCTION reject_category_delete()`)
	require.NoError(t, err)

	deleteQuery := `mutation($id:ID!){deleteTagCategory(id:$id,expectedRevision:"2")}`
	failed := imageGraphQLCall(t, f.server, f.adminToken, deleteQuery, map[string]any{"id": f.categoryID.String()})
	require.NotEmpty(t, failed.Errors)

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), agentTeaTagCountQuery, f.teaID, f.tagID).Scan(&count))
	require.Equal(t, 1, count)
	require.Equal(t, "2", agentRevision(t, f, agentTeaTable, f.teaID))
	_, err = f.db.ExecContext(t.Context(), `DROP TRIGGER reject_category_delete ON tag_categories`)
	require.NoError(t, err)
	require.Empty(t, imageGraphQLCall(t, f.server, f.adminToken, deleteQuery, map[string]any{"id": f.categoryID.String()}).Errors)
	require.Equal(t, "3", agentRevision(t, f, agentTeaTable, f.teaID))
}

func newAgentFixture(t *testing.T) roleFixture {
	t.Helper()
	f := newRoleFixture(t)
	store := pgadapter.NewDB(f.db, logrus.NewEntry(logrus.New()))
	f.resolver.images = teaimages.NewService(store)

	return f
}
func agentRevision(t *testing.T, f roleFixture, table string, id uuid.UUID) string {
	t.Helper()

	var result string
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT revision::text FROM `+table+` WHERE id=$1`, id).Scan(&result))

	return result
}

func TestCatalogueMigrationPreservesExistingLibraryAndQRMembership(t *testing.T) {
	f := newAgentFixture(t)
	label := uuid.New()
	collectionID := uuid.New()
	_, err := f.db.ExecContext(t.Context(), `INSERT INTO qr_records(id,tea_id,boiling_temp,expiration_date) VALUES($1,$2,85,now())`, label, f.teaID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO collections(id,user_id,name) VALUES($1,$2,'Family shelf')`, collectionID, f.ownerID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `INSERT INTO collection_qr_items(collection_id,qr_id) VALUES($1,$2)`, collectionID, label)
	require.NoError(t, err)
	image := imageGraphQLCall(t, f.server, f.ownerToken, `mutation($tea:ID!,$image:String!){addTeaImage(teaID:$tea,imageBase64:$image){images{id}}}`, map[string]any{testTeaVariable: f.teaID.String(), testImageVariable: imageFixtureBase64(t)})
	require.Empty(t, image.Errors)
	_, err = f.db.ExecContext(t.Context(), `DROP TRIGGER catalog_tea_revision ON teas; DROP TRIGGER catalog_tag_revision ON tags; DROP TRIGGER catalog_category_revision ON tag_categories; DROP TRIGGER catalog_tea_tags_revision ON tea_tags; DROP TRIGGER catalog_tea_images_revision ON tea_images; ALTER TABLE teas DROP COLUMN revision; ALTER TABLE tags DROP COLUMN revision; ALTER TABLE tag_categories DROP COLUMN revision; DROP TABLE tea_merge_receipt_details; DROP TABLE catalog_operation_receipts`)
	require.NoError(t, err)
	source, err := os.ReadFile(agentMigrationPath)
	require.NoError(t, err)

	for range 2 {
		_, err = f.db.ExecContext(t.Context(), string(source))
		require.NoError(t, err)
	}

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM collection_qr_items WHERE collection_id=$1 AND qr_id=$2`, collectionID, label).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, f.db.QueryRowContext(t.Context(), agentImageCountQuery, f.teaID).Scan(&count))
	require.Equal(t, 1, count)
	require.Equal(t, "1", agentRevision(t, f, agentTeaTable, f.teaID))
	require.Equal(t, "1", agentRevision(t, f, roleTagsVariable, f.tagID))
	require.Equal(t, "1", agentRevision(t, f, agentCategoryTable, f.categoryID))
}

type agentCallResult struct {
	Reply imageGraphQLReply
	Err   error
}
type agentTeaRecord struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
}

//nolint:tagliatelle // GraphQL response fields use the published camelCase wire names.
type agentReceiptRecord struct {
	Operation      string `json:"operation"`
	TeaID          string `json:"teaID"`
	ResourceID     string `json:"resourceID"`
	ResultRevision string `json:"resultRevision"`
}

//nolint:tagliatelle // GraphQL response fields use the published camelCase wire names.
type agentCapabilitiesRecord struct {
	ProtocolVersion     string `json:"protocolVersion"`
	SupportsRevisions   bool   `json:"supportsRevisions"`
	SupportsIdempotency bool   `json:"supportsIdempotency"`
}

// Concurrent callers collect errors; only the main test goroutine makes assertions.
func concurrentAgentGraphQLCall(ctx context.Context, server http.Handler, token, query string, variables map[string]any) agentCallResult {
	body, err := json.Marshal(map[string]any{"query": query, agentVariablesKey: variables})
	if err != nil {
		return agentCallResult{Err: fmt.Errorf("marshal request: %w", err)}
	}

	request := httptest.NewRequestWithContext(ctx, http.MethodPost, testGraphQLPath, bytes.NewReader(body))
	request.Header.Set(testContentTypeHeader, testJSONContentType)
	request.Header.Set(agentAuthorizationHeader, "Bearer "+token)

	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	var reply imageGraphQLReply
	if err := json.Unmarshal(response.Body.Bytes(), &reply); err != nil {
		return agentCallResult{Err: fmt.Errorf("decode concurrent reply: %w", err)}
	}

	return agentCallResult{Reply: reply}
}

func TestCataloguePhotoMutationsCompareWholeGalleryRevision(t *testing.T) {
	f := newAgentFixture(t)
	upload := `mutation($tea:ID!,$image:String!){addTeaImage(teaID:$tea,imageBase64:$image){images{id isCover}}}`
	vars := map[string]any{testTeaVariable: f.teaID.String(), testImageVariable: imageFixtureBase64(t)}
	require.Empty(t, imageGraphQLCall(t, f.server, f.ownerToken, upload, vars).Errors)
	second := imageGraphQLCall(t, f.server, f.ownerToken, upload, vars)
	require.Empty(t, second.Errors)

	var result struct {
		Images []struct {
			ID string `json:"id"`
		} `json:"images"`
	}
	require.NoError(t, json.Unmarshal(second.Data[testAddImageField], &result))
	require.Len(t, result.Images, 2)

	cover := `mutation($tea:ID!,$image:ID!){setTeaCoverImage(teaID:$tea,imageID:$image,expectedRevision:"3"){revision}}`
	coverVars := map[string]any{testTeaVariable: f.teaID.String(), testImageVariable: result.Images[1].ID}
	require.Empty(t, imageGraphQLCall(t, f.server, f.ownerToken, cover, coverVars).Errors)
	current := agentRevision(t, f, agentTeaTable, f.teaID)
	staleDelete := `mutation($tea:ID!,$image:ID!){deleteTeaImage(teaID:$tea,imageID:$image,expectedRevision:"3"){revision}}`
	conflict := imageGraphQLCall(t, f.server, f.adminToken, staleDelete, coverVars)
	require.Len(t, conflict.Errors, 1)
	require.Equal(t, agentConflictCode, conflict.Errors[0].Extensions["code"])
	require.Equal(t, current, agentRevision(t, f, agentTeaTable, f.teaID))
	validDelete := strings.Replace(staleDelete, agentExpectedRevisionThree, `expectedRevision:"`+current+`"`, 1)
	require.Empty(t, imageGraphQLCall(t, f.server, f.adminToken, validDelete, coverVars).Errors)

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), agentImageCountQuery, f.teaID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestCatalogueConcurrentPhotoReplayAndFreshKeys(t *testing.T) {
	for _, sameKey := range []bool{true, false} {
		t.Run(fmt.Sprintf("same_key=%t", sameKey), func(t *testing.T) {
			f := newAgentFixture(t)
			upload := `mutation($tea:ID!,$image:String!,$key:String!){addTeaImage(teaID:$tea,imageBase64:$image,expectedRevision:"1",idempotencyKey:$key){revision}}`
			image, key := imageFixtureBase64(t), uuid.NewString()
			start := make(chan struct{})
			results := make(chan agentCallResult, 4)

			for range 4 {
				operationKey := key
				if !sameKey {
					operationKey = uuid.NewString()
				}

				go func() {
					<-start

					results <- concurrentAgentGraphQLCall(t.Context(), f.server, f.adminToken, upload, map[string]any{testTeaVariable: f.teaID.String(), testImageVariable: image, agentKeyVariable: operationKey})
				}()
			}

			close(start)

			successes := 0

			for range 4 {
				result := <-results
				require.NoError(t, result.Err)

				if len(result.Reply.Errors) == 0 {
					successes++
				} else {
					require.Equal(t, agentConflictCode, result.Reply.Errors[0].Extensions["code"])
				}
			}

			if sameKey {
				require.Equal(t, 4, successes)
			} else {
				require.Equal(t, 1, successes)
			}

			var count int
			require.NoError(t, f.db.QueryRowContext(t.Context(), agentImageCountQuery, f.teaID).Scan(&count))
			require.Equal(t, 1, count)
			require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM catalog_operation_receipts`).Scan(&count))
			require.Equal(t, 1, count)
			require.Equal(t, "2", agentRevision(t, f, agentTeaTable, f.teaID))
		})
	}
}

func TestCatalogueCapabilitiesFailClosedOnDisabledTriggersOrMissingRevision(t *testing.T) {
	f := newAgentFixture(t)
	query := `query{editorCapabilities{supportsRevisions supportsIdempotency}}`
	_, err := f.db.ExecContext(t.Context(), `ALTER TABLE tea_images DISABLE TRIGGER catalog_tea_images_revision`)
	require.NoError(t, err)
	reply := imageGraphQLCall(t, f.server, f.adminToken, query, nil)
	require.Empty(t, reply.Errors)

	var result agentCapabilitiesRecord
	require.NoError(t, json.Unmarshal(reply.Data["editorCapabilities"], &result))
	require.False(t, result.SupportsRevisions)
	require.False(t, result.SupportsIdempotency)
	_, err = f.db.ExecContext(t.Context(), `ALTER TABLE tea_images ENABLE TRIGGER catalog_tea_images_revision; ALTER TABLE tags DROP COLUMN revision`)
	require.NoError(t, err)
	reply = imageGraphQLCall(t, f.server, f.adminToken, query, nil)
	require.Empty(t, reply.Errors)
	require.NoError(t, json.Unmarshal(reply.Data["editorCapabilities"], &result))
	require.False(t, result.SupportsRevisions)
	require.False(t, result.SupportsIdempotency)
}

func TestCatalogueTagMutationsReturnHydratedCategory(t *testing.T) {
	for _, operation := range []string{agentCreateCase, agentUpdateCase, agentMoveCase} {
		t.Run(operation, func(t *testing.T) {
			f := newAgentFixture(t)
			categoryID, categoryName := f.categoryID.String(), agentCategoryName
			field, query := "createTag", `mutation($category:ID!){createTag(name:"Agent Rose",color:"green",category:$category){id revision name category{id revision name __typename}}}`
			variables := map[string]any{agentCategoryVariable: categoryID}

			switch operation {
			case agentUpdateCase:
				field, query = "updateTag", `mutation($id:ID!){updateTag(id:$id,name:"Agent renamed",color:"green",expectedRevision:"1"){id revision name category{id revision name __typename}}}`
				variables = map[string]any{"id": f.tagID.String()}
			case agentMoveCase:
				created := imageGraphQLCall(t, f.server, f.adminToken, `mutation{createTagCategory(name:"Agent Origin"){id revision name}}`, nil)
				require.Empty(t, created.Errors)

				var category agentTeaRecord
				require.NoError(t, json.Unmarshal(created.Data["createTagCategory"], &category))
				categoryID, categoryName = category.ID, "Agent Origin"
				field, query = "changeTagCategory", `mutation($id:ID!,$category:ID!){changeTagCategory(id:$id,category:$category,expectedRevision:"1"){id revision name category{id revision name __typename}}}`
				variables = map[string]any{"id": f.tagID.String(), agentCategoryVariable: categoryID}
			}

			reply := imageGraphQLCall(t, f.server, f.adminToken, query, variables)
			require.Empty(t, reply.Errors, "a committed tag write must not turn into an unknown outcome because its non-null category is absent")

			var result struct {
				ID       string `json:"id"`
				Revision string `json:"revision"`
				Category struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					Revision string `json:"revision"`
				} `json:"category"`
			}
			require.NoError(t, json.Unmarshal(reply.Data[field], &result))
			require.NotEmpty(t, result.ID)
			require.Equal(t, categoryID, result.Category.ID)
			require.Equal(t, categoryName, result.Category.Name)
			require.Equal(t, "1", result.Category.Revision)

			if operation == agentCreateCase {
				require.Equal(t, "1", result.Revision)
			} else {
				require.Equal(t, "2", result.Revision)
			}
		})
	}
}
