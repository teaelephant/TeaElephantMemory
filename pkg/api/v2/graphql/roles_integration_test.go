package graphql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/teaelephant/TeaElephantMemory/common"
	authPkg "github.com/teaelephant/TeaElephantMemory/internal/auth"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/collection"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/notification"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/qr"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/tag"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/tea"
	"github.com/teaelephant/TeaElephantMemory/internal/teaimages"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql/generated"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
	pgadapter "github.com/teaelephant/TeaElephantMemory/pkg/pg"
)

const (
	roleJarField             = "writeToQR"
	roleDeviceField          = "registerDeviceToken"
	roleCollectionField      = "createCollection"
	roleMutationData         = "New catalog entry"
	roleOwner                = "owner"
	roleOther                = "other"
	roleMissing              = "missing"
	roleAddTag               = "addTagToTea"
	roleDeleteTag            = "deleteTagFromTea"
	roleDeviceToken          = "owner-apns-token"
	roleCountQR              = `SELECT count(*) FROM qr_records WHERE id=$1`
	roleCountCollectionItems = `SELECT count(*) FROM collection_qr_items WHERE collection_id=$1`
	roleNewTea               = "newTea"
	roleUpdateTea            = "updateTea"
	roleDirectCase           = "direct=%t"
	roleBoilingVariable      = "bowlingTemp"
	roleQRBindingQuery       = `SELECT tea_id,boiling_temp FROM qr_records WHERE id=$1`
	roleDataVariable         = "data"
	roleExpirationVariable   = "expirationDate"
	roleDescriptionVariable  = "description"
	roleTagVariable          = "tag"
	roleNotFoundCode         = "NOT_FOUND"
	roleNameVariable         = "name"
	roleTypeVariable         = "type"
	roleTagsVariable         = "tags"
)

type roleFixture struct {
	db         *sql.DB
	resolver   *Resolver
	auth       authPkg.Auth
	server     *handler.Server
	ownerID    uuid.UUID
	otherID    uuid.UUID
	ownerToken string
	otherToken string
	adminToken string
	teaID      uuid.UUID
	tagID      uuid.UUID
	categoryID uuid.UUID
}

func newRoleFixture(t *testing.T) roleFixture {
	t.Helper()

	db := imageTestDatabase(t)
	log := logrus.NewEntry(logrus.New())
	store := pgadapter.NewDB(db, log)
	ownerID, err := store.GetOrCreateUser(t.Context(), roleOwner)
	require.NoError(t, err)
	otherID, err := store.GetOrCreateUser(t.Context(), roleOther)
	require.NoError(t, err)
	auth, tokens, adminToken := imageTestAuthUsers(t, store, ownerID, otherID)
	teaManager := tea.NewManager(store)
	resolver := NewResolver(log, teaManager, qr.NewManager(store), tag.NewManager(store, teaManager, log),
		collection.NewManager(store), nil, nil, notification.NewManager(store), nil, nil, nil, nil)
	server := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
	server.Use(auth.Middleware())

	record, err := store.WriteRecord(t.Context(), &common.TeaData{Name: testSharedTeaName, Type: common.TeaBeverageType, Description: "Unchanged shared description"})
	require.NoError(t, err)
	category, err := store.CreateTagCategory(t.Context(), "Aroma")
	require.NoError(t, err)
	item, err := store.CreateTag(t.Context(), "Floral", "green", category.ID)
	require.NoError(t, err)

	return roleFixture{db: db, resolver: resolver, auth: auth, server: server, ownerID: ownerID, otherID: otherID,
		ownerToken: tokens[0], otherToken: tokens[1], adminToken: adminToken, teaID: record.ID, tagID: item.ID, categoryID: category.ID}
}

func roleContext(t *testing.T, auth authPkg.Auth, token string) context.Context {
	t.Helper()

	payload := transport.InitPayload{}
	if token != "" {
		payload["Authorization"] = "Bearer " + token
	}

	ctx, _, err := auth.WsInitFunc(t.Context(), payload)
	require.NoError(t, err)

	return ctx
}

func catalogState(t *testing.T, db *sql.DB) string {
	t.Helper()

	var snapshot string

	err := db.QueryRowContext(t.Context(), `SELECT jsonb_build_object(
        'teas', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM teas t),
        'categories', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM tag_categories t),
        'tags', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM tags t),
        'assignments', (SELECT jsonb_agg(to_jsonb(t) ORDER BY tea_id, tag_id) FROM tea_tags t),
        'images', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM tea_images t))::text`).Scan(&snapshot)
	require.NoError(t, err)

	return snapshot
}

func assertRoleDenied(t *testing.T, reply imageGraphQLReply, code any) {
	t.Helper()

	require.Len(t, reply.Errors, 1)
	require.Equal(t, code, reply.Errors[0].Extensions["code"])
}

func TestCatalogueWritePermissionsPreserveUserCreationAndAdminEditing(t *testing.T) {
	allowedRoles := map[string]map[string]bool{
		roleNewTea:    {testMobileUserRole: true, testAdminRole: true},
		roleUpdateTea: {testAdminRole: true},
		roleAddTag:    {testAdminRole: true},
		roleDeleteTag: {testAdminRole: true},
	}

	for _, operation := range []string{roleNewTea, roleUpdateTea, roleAddTag, roleDeleteTag} {
		for _, direct := range []bool{false, true} {
			for _, role := range []string{testAnonymousRole, testMobileUserRole, testAdminRole} {
				t.Run(fmt.Sprintf("%s/%s/direct=%t", operation, role, direct), func(t *testing.T) {
					f := newRoleFixture(t)
					if operation == roleDeleteTag {
						_, err := f.db.ExecContext(t.Context(), `INSERT INTO tea_tags(tea_id,tag_id) VALUES($1,$2)`, f.teaID, f.tagID)
						require.NoError(t, err)
					}

					before := catalogState(t, f.db)
					token := map[string]string{testAnonymousRole: "", testMobileUserRole: f.ownerToken, testAdminRole: f.adminToken}[role]
					allowed := allowedRoles[operation][role]

					code := testImageForbiddenCode
					if role == testAnonymousRole {
						code = testImageUnauthorizedCode
					}

					if direct {
						ctx := roleContext(t, f.auth, token)

						_, err := callCatalogueResolver(ctx, f.resolver, operation, f.teaID, f.tagID)
						if allowed {
							require.NoError(t, err)
						} else {
							var gqlErr *gqlerror.Error
							require.ErrorAs(t, err, &gqlErr)
							require.Equal(t, code, gqlErr.Extensions["code"])
						}
					} else {
						reply := imageGraphQLCall(t, f.server, token, catalogMutation(operation), map[string]any{testTeaVariable: f.teaID.String(), roleTagVariable: f.tagID.String()})
						if allowed {
							require.Empty(t, reply.Errors)
							require.NotEmpty(t, reply.Data[operation])
						} else {
							assertRoleDenied(t, reply, code)
						}
					}

					after := catalogState(t, f.db)
					if allowed {
						require.NotEqual(t, before, after, "allowed operation must change actual persisted catalog data")
					} else {
						require.Equal(t, before, after, "refusal must preserve all shared catalog data")
					}
				})
			}
		}
	}
}

func callCatalogueResolver(ctx context.Context, resolver *Resolver, operation string, teaID, tagID uuid.UUID) (*model.Tea, error) {
	data := model.TeaData{Name: roleMutationData, Type: model.TypeTea, Description: "Updated catalog description"}

	var result *model.Tea

	var err error

	switch operation {
	case roleNewTea:
		result, err = resolver.Mutation().NewTea(ctx, data, nil)
	case roleUpdateTea:
		result, err = resolver.Mutation().UpdateTea(ctx, gqlCommon.ID(teaID), data)
	case roleAddTag:
		result, err = resolver.Mutation().AddTagToTea(ctx, gqlCommon.ID(teaID), gqlCommon.ID(tagID))
	default:
		result, err = resolver.Mutation().DeleteTagFromTea(ctx, gqlCommon.ID(teaID), gqlCommon.ID(tagID))
	}

	if err != nil {
		return nil, fmt.Errorf("catalog mutation %s: %w", operation, err)
	}

	return result, nil
}

func catalogMutation(operation string) string {
	switch operation {
	case roleNewTea:
		return `mutation { newTea(tea:{name:"New catalog entry",type:tea,description:"Updated catalog description"}) { id } }`
	case roleUpdateTea:
		return `mutation($tea:ID!) { updateTea(id:$tea,tea:{name:"New catalog entry",type:tea,description:"Updated catalog description"}) { id } }`
	default:
		return `mutation($tea:ID!,$tag:ID!) { ` + operation + `(teaID:$tea,tagID:$tag) { id } }`
	}
}

func TestSharedQRWritesRequireAuthenticationAndRemainUsableAcrossUsers(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(fmt.Sprintf(roleDirectCase, direct), func(t *testing.T) {
			f := newRoleFixture(t)
			id := uuid.New()
			expiration := time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC)
			data := model.QRRecordData{Tea: gqlCommon.ID(f.teaID), BowlingTemp: 85, ExpirationDate: expiration}
			before := catalogState(t, f.db)
			query := `mutation($id:ID!,$data:QRRecordData!) { writeToQR(id:$id,data:$data) { id bowlingTemp expirationDate tea { id name description } } }`
			persistedTemp := 0

			for _, role := range []string{testAnonymousRole, roleOwner, testAnonymousRole, roleOther, testAdminRole} {
				token := map[string]string{testAnonymousRole: "", roleOwner: f.ownerToken, roleOther: f.otherToken, testAdminRole: f.adminToken}[role]

				data.BowlingTemp++
				if direct {
					result, err := f.resolver.Mutation().WriteToQR(roleContext(t, f.auth, token), gqlCommon.ID(id), data)

					if role == testAnonymousRole {
						var gqlErr *gqlerror.Error
						require.ErrorAs(t, err, &gqlErr)
						require.Equal(t, testImageUnauthorizedCode, gqlErr.Extensions["code"])
					} else {
						require.NoError(t, err)
						require.Equal(t, id, uuid.UUID(result.ID))
						require.Equal(t, data.BowlingTemp, result.BowlingTemp)
					}
				} else {
					vars := map[string]any{"id": id.String(), roleDataVariable: map[string]any{testTeaVariable: f.teaID.String(), roleBoilingVariable: data.BowlingTemp, roleExpirationVariable: expiration.Format(time.RFC3339)}}

					reply := imageGraphQLCall(t, f.server, token, query, vars)
					if role == testAnonymousRole {
						assertRoleDenied(t, reply, testImageUnauthorizedCode)
					} else {
						require.Empty(t, reply.Errors)
						require.NotEmpty(t, reply.Data[roleJarField])
					}
				}

				var count int
				require.NoError(t, f.db.QueryRowContext(t.Context(), roleCountQR, id).Scan(&count))

				if persistedTemp == 0 && role == testAnonymousRole {
					require.Zero(t, count, "anonymous QR write must not persist a record")
				} else {
					require.Equal(t, 1, count, "same shared QR ID must update, not create one record per user")

					if role != testAnonymousRole {
						persistedTemp = data.BowlingTemp
					}

					var actualTemp int

					var actualTea uuid.UUID
					require.NoError(t, f.db.QueryRowContext(t.Context(), roleQRBindingQuery, id).Scan(&actualTea, &actualTemp))
					require.Equal(t, persistedTemp, actualTemp, "anonymous overwrites must preserve the existing jar metadata")
					require.Equal(t, f.teaID, actualTea)
				}

				require.Equal(t, before, catalogState(t, f.db), "jar writes must not modify the shared Tea")
			}

			reply := imageGraphQLCall(t, f.server, "", `query($id:ID!) { qrRecord(id:$id) { id bowlingTemp } }`, map[string]any{"id": id.String()})
			require.Empty(t, reply.Errors, "reading a jar still works without authentication")
		})
	}
}

func TestRegisterDeviceTokenRequiresAuthenticatedDeviceOwner(t *testing.T) {
	for _, direct := range []bool{false, true} {
		for _, role := range []string{testAnonymousRole, roleOwner, roleOther, testAdminRole, roleMissing} {
			t.Run(fmt.Sprintf("%s/direct=%t", role, direct), func(t *testing.T) {
				f := newRoleFixture(t)
				deviceID := uuid.New()
				store := pgadapter.NewDB(f.db, logrus.NewEntry(logrus.New()))
				require.NoError(t, store.AddDeviceForUser(t.Context(), f.ownerID, deviceID))
				require.NoError(t, store.AddDeviceForUser(t.Context(), f.otherID, deviceID), "repeat binding must not transfer ownership")

				requestedID := deviceID
				if role == roleMissing {
					requestedID = uuid.New()
				}

				token := map[string]string{testAnonymousRole: "", roleOwner: f.ownerToken, roleOther: f.otherToken, testAdminRole: f.adminToken, roleMissing: f.ownerToken}[role]
				allowed := role == roleOwner

				var code any = float64(ErrDeviceNotFound)
				if role == testAnonymousRole || role == testAdminRole {
					code = float64(ErrUserNotFound)
				}

				for range 2 {
					assertDeviceRegistration(t, f, direct, token, requestedID, allowed, code)

					var actualUser uuid.UUID

					var actualToken string
					require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT user_id,token FROM devices WHERE id=$1`, deviceID).Scan(&actualUser, &actualToken))
					require.Equal(t, f.ownerID, actualUser, "authentication/binding collisions must not transfer a device")

					if allowed {
						require.Equal(t, roleDeviceToken, actualToken)
					} else {
						require.Equal(t, deviceID.String(), actualToken, "refused registration must preserve existing token")
					}
				}
			})
		}
	}
}

func assertDeviceRegistration(t *testing.T, f roleFixture, direct bool, token string, deviceID uuid.UUID, allowed bool, code any) {
	t.Helper()

	if direct {
		result, err := f.resolver.Mutation().RegisterDeviceToken(roleContext(t, f.auth, token), gqlCommon.ID(deviceID), roleDeviceToken)
		if allowed {
			require.NoError(t, err)
			require.True(t, result)
		} else {
			var gqlErr *gqlerror.Error
			require.ErrorAs(t, err, &gqlErr)
			require.EqualValues(t, code, gqlErr.Extensions["code"])
		}
	} else {
		reply := imageGraphQLCall(t, f.server, token, `mutation($id:ID!,$token:String!) { registerDeviceToken(deviceID:$id,deviceToken:$token) }`, map[string]any{"id": deviceID.String(), "token": roleDeviceToken})
		if allowed {
			require.Empty(t, reply.Errors)
			require.JSONEq(t, "true", string(reply.Data[roleDeviceField]))
		} else {
			assertRoleDenied(t, reply, code)
		}
	}
}

func TestPersonalCollectionsRemainOwnerScopedAndPreserveSharedResources(t *testing.T) {
	f := newRoleFixture(t)
	store := pgadapter.NewDB(f.db, logrus.NewEntry(logrus.New()))
	qrID := uuid.New()
	require.NoError(t, store.WriteQR(t.Context(), qrID, &common.QR{Tea: f.teaID, BowlingTemp: 90, ExpirationDate: time.Now().Add(time.Hour)}))
	before := catalogState(t, f.db)
	created := imageGraphQLCall(t, f.server, f.ownerToken, `mutation { createCollection(name:"Kitchen") { id } }`, nil)
	require.Empty(t, created.Errors)

	var result struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(created.Data[roleCollectionField], &result))
	vars := map[string]any{"id": result.ID, "records": []string{qrID.String()}}
	add := `mutation($id:ID!,$records:[ID!]!) { addRecordsToCollection(id:$id,records:$records) { id records { id } } }`
	remove := `mutation($id:ID!,$records:[ID!]!) { deleteRecordsFromCollection(id:$id,records:$records) { id } }`
	deleted := `mutation($id:ID!) { deleteCollection(id:$id) }`

	for _, token := range []string{"", f.adminToken} {
		denied := imageGraphQLCall(t, f.server, token, `mutation { createCollection(name:"Foreign") { id } }`, nil)
		require.NotEmpty(t, denied.Errors)
	}

	for _, token := range []string{"", f.otherToken, f.adminToken} {
		denied := imageGraphQLCall(t, f.server, token, add, vars)
		require.NotEmpty(t, denied.Errors)

		var count int
		require.NoError(t, f.db.QueryRowContext(t.Context(), roleCountCollectionItems, result.ID).Scan(&count))
		require.Zero(t, count)
	}

	reply := imageGraphQLCall(t, f.server, f.ownerToken, add, vars)
	require.Empty(t, reply.Errors)

	for _, token := range []string{f.otherToken, f.adminToken} {
		listed := imageGraphQLCall(t, f.server, token, `query { collections { id records { id } } }`, nil)
		if token == f.otherToken {
			require.Empty(t, listed.Errors)
			require.JSONEq(t, "[]", string(listed.Data["collections"]))
		} else {
			require.NotEmpty(t, listed.Errors)
		}

		imageGraphQLCall(t, f.server, token, remove, vars)
		imageGraphQLCall(t, f.server, token, deleted, vars)

		var count int
		require.NoError(t, f.db.QueryRowContext(t.Context(), roleCountCollectionItems, result.ID).Scan(&count))
		require.Equal(t, 1, count, "foreign removal/deletion must preserve owner's collection")
	}

	reply = imageGraphQLCall(t, f.server, f.ownerToken, remove, vars)
	require.Empty(t, reply.Errors)
	assertCollectionHasNoItems(t, f, result.ID)
	reply = imageGraphQLCall(t, f.server, f.ownerToken, deleted, vars)
	require.Empty(t, reply.Errors)
	assertCollectionRemovalPreservesSharedJar(t, f, qrID, result.ID, before)
}

func assertCollectionRemovalPreservesSharedJar(t *testing.T, f roleFixture, qrID uuid.UUID, collectionID, before string) {
	t.Helper()

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), roleCountQR, qrID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM collections WHERE id=$1`, collectionID).Scan(&count))
	require.Zero(t, count, "owner must be able to delete the personal collection")
	require.Equal(t, before, catalogState(t, f.db))
}

func assertCollectionHasNoItems(t *testing.T, f roleFixture, collectionID string) {
	t.Helper()

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), roleCountCollectionItems, collectionID).Scan(&count))
	require.Zero(t, count, "owner must be able to remove a jar from the personal collection")
}

func TestSharedQRCanBeReassignedToExistingTeaWithoutCollectionMembership(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(fmt.Sprintf(roleDirectCase, direct), func(t *testing.T) {
			f := newRoleFixture(t)
			store := pgadapter.NewDB(f.db, logrus.NewEntry(logrus.New()))
			f.resolver.images = teaimages.NewService(store)
			second, err := store.WriteRecord(t.Context(), &common.TeaData{Name: "Second shared Tea", Type: common.TeaBeverageType, Description: "Another unchanged description"})
			require.NoError(t, err)
			secondImageID := seedReassignmentGalleries(t, f, second.ID)
			before := catalogState(t, f.db)
			id := uuid.New()
			expiration := time.Date(2030, 1, 2, 0, 0, 0, 0, time.UTC)
			data := model.QRRecordData{Tea: gqlCommon.ID(f.teaID), BowlingTemp: 90, ExpirationDate: expiration}
			assertQRReassignment(t, f, direct, f.ownerToken, id, data, true)

			var collections int
			require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM collections WHERE user_id=$1`, f.otherID).Scan(&collections))
			require.Zero(t, collections, "second user must have no collection authorizing this label change")

			data.Tea = gqlCommon.ID(second.ID)
			data.BowlingTemp = 85
			assertQRReassignment(t, f, direct, f.otherToken, id, data, true)
			assertReboundQR(t, f, id, second.ID, secondImageID)
			require.Equal(t, before, catalogState(t, f.db), "reassigning a label must preserve both shared Teas and their galleries")

			data.Tea = gqlCommon.ID(f.teaID)
			data.BowlingTemp = 55
			assertQRReassignment(t, f, direct, "", id, data, false)
			assertReboundQR(t, f, id, second.ID, secondImageID)
			require.Equal(t, before, catalogState(t, f.db), "refused reassignment must preserve both shared Teas and their galleries")
		})
	}
}

func seedReassignmentGalleries(t *testing.T, f roleFixture, secondTeaID uuid.UUID) string {
	t.Helper()

	var secondImageID string

	query := `mutation($tea:ID!,$image:String!) { addTeaImage(teaID:$tea,imageBase64:$image) { ` + imageSelection + ` } }`

	for _, teaID := range []uuid.UUID{f.teaID, secondTeaID} {
		vars := map[string]any{testTeaVariable: teaID.String(), testImageVariable: imageFixtureBase64(t)}
		reply := decodeTeaImageReply(t, imageGraphQLCall(t, f.server, f.ownerToken, query, vars), testAddImageField)
		require.Len(t, reply.Images, 1)

		if teaID == secondTeaID {
			secondImageID = reply.Images[0].ID
		}
	}

	return secondImageID
}

func assertQRReassignment(t *testing.T, f roleFixture, direct bool, token string, id uuid.UUID, data model.QRRecordData, allowed bool) {
	t.Helper()

	if direct {
		record, err := f.resolver.Mutation().WriteToQR(roleContext(t, f.auth, token), gqlCommon.ID(id), data)
		if allowed {
			require.NoError(t, err)
			require.Equal(t, data.Tea, record.Tea.ID)
		} else {
			var gqlErr *gqlerror.Error
			require.ErrorAs(t, err, &gqlErr)
			require.Equal(t, testImageUnauthorizedCode, gqlErr.Extensions["code"])
		}
	} else {
		vars := map[string]any{"id": id.String(), roleDataVariable: map[string]any{testTeaVariable: uuid.UUID(data.Tea).String(), roleBoilingVariable: data.BowlingTemp, roleExpirationVariable: data.ExpirationDate.Format(time.RFC3339)}}
		query := `mutation($id:ID!,$data:QRRecordData!) { writeToQR(id:$id,data:$data) { id tea { id } } }`

		reply := imageGraphQLCall(t, f.server, token, query, vars)
		if allowed {
			require.Empty(t, reply.Errors)
		} else {
			assertRoleDenied(t, reply, testImageUnauthorizedCode)
		}
	}
}

func assertReboundQR(t *testing.T, f roleFixture, id, teaID uuid.UUID, imageID string) {
	t.Helper()

	var count int
	require.NoError(t, f.db.QueryRowContext(t.Context(), roleCountQR, id).Scan(&count))
	require.Equal(t, 1, count, "the shared label must remain one row")

	var actualTea uuid.UUID

	var actualTemp int
	require.NoError(t, f.db.QueryRowContext(t.Context(), roleQRBindingQuery, id).Scan(&actualTea, &actualTemp))
	require.Equal(t, teaID, actualTea)
	require.Equal(t, 85, actualTemp, "anonymous attempts must not overwrite existing metadata")

	query := `query($id:ID!) { qrRecord(id:$id) { tea { id images { id } } } }`
	reply := imageGraphQLCall(t, f.server, "", query, map[string]any{"id": id.String()})
	require.Empty(t, reply.Errors)

	var record struct {
		Tea struct {
			ID     string `json:"id"`
			Images []struct {
				ID string `json:"id"`
			} `json:"images"`
		} `json:"tea"`
	}
	require.NoError(t, json.Unmarshal(reply.Data["qrRecord"], &record))
	require.Equal(t, teaID.String(), record.Tea.ID, "public label reads must reflect the latest Tea association")
	require.Len(t, record.Tea.Images, 1)
	require.Equal(t, imageID, record.Tea.Images[0].ID, "label reads must expose the newly associated Tea gallery")
}

func TestUserTeaCreationWithExistingNamePreservesSharedTeaAndGallery(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(fmt.Sprintf(roleDirectCase, direct), func(t *testing.T) {
			f := newRoleFixture(t)
			store := pgadapter.NewDB(f.db, logrus.NewEntry(logrus.New()))
			images := teaimages.NewService(store)
			f.resolver.images = images
			require.NoError(t, images.Add(t.Context(), f.teaID, imageFixtureBase64(t)))
			before := existingTeaState(t, f.db, f.teaID)
			data := model.TeaData{Name: testSharedTeaName, Type: model.TypeOther, Description: "Independent user-created drink"}
			createdID := createTeaAsUser(t, f, direct, data)
			require.NotEqual(t, f.teaID, createdID, "creation must allocate a new identity even when the name exists")
			created, err := store.ReadRecord(t.Context(), createdID)
			require.NoError(t, err)
			require.Equal(t, testSharedTeaName, created.Name)
			require.Equal(t, data.Description, created.Description)
			require.Equal(t, common.OtherBeverageType, created.Type)
			require.Equal(t, before, existingTeaState(t, f.db, f.teaID), "user creation must preserve the existing Tea and its image bytes/cover")

			var count int
			require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM teas WHERE name=$1`, testSharedTeaName).Scan(&count))
			require.Equal(t, 2, count)
			require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM tea_images WHERE tea_id=$1`, createdID).Scan(&count))
			require.Zero(t, count, "a new Tea must not alias the pre-existing image gallery")
		})
	}
}

func createTeaAsUser(t *testing.T, f roleFixture, direct bool, data model.TeaData) uuid.UUID {
	t.Helper()

	if direct {
		created, err := f.resolver.Mutation().NewTea(roleContext(t, f.auth, f.ownerToken), data, nil)
		require.NoError(t, err)

		return uuid.UUID(created.ID)
	}

	query := `mutation($tea:TeaData!) { newTea(tea:$tea) { ` + imageSelection + ` } }`
	vars := map[string]any{testTeaVariable: map[string]any{roleNameVariable: data.Name, roleTypeVariable: data.Type.String(), roleDescriptionVariable: data.Description}}
	created := decodeTeaImageReply(t, imageGraphQLCall(t, f.server, f.ownerToken, query, vars), roleNewTea)
	id, err := uuid.Parse(created.ID)
	require.NoError(t, err)

	return id
}

func existingTeaState(t *testing.T, db *sql.DB, id uuid.UUID) string {
	t.Helper()

	var snapshot string

	err := db.QueryRowContext(t.Context(), `SELECT jsonb_build_object(
        'tea', (SELECT to_jsonb(t) FROM teas t WHERE id=$1),
        'images', (SELECT jsonb_agg(to_jsonb(t) ORDER BY id) FROM tea_images t WHERE tea_id=$1))::text`, id).Scan(&snapshot)
	require.NoError(t, err)

	return snapshot
}
