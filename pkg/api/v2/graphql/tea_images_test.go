package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/teaelephant/TeaElephantMemory/common"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql/generated"
)

const (
	testImageUnauthorizedCode = "UNAUTHENTICATED"
	testImageForbiddenCode    = "FORBIDDEN"
	testAdminRole             = "admin"
	testAnonymousRole         = "anonymous"
	testMobileUserRole        = "mobile user"
	testSharedTeaName         = "Shared tea"
	testDeleteImageField      = "deleteTeaImage"
)

func TestTeaImageMutationsRequireAuthentication(t *testing.T) {
	t.Parallel()

	for _, mutation := range []string{
		`addTeaImage(teaID:"11111111-1111-1111-1111-111111111111",imageBase64:"AA==")`,
		`deleteTeaImage(teaID:"11111111-1111-1111-1111-111111111111",imageID:"22222222-2222-2222-2222-222222222222")`,
		`setTeaCoverImage(teaID:"11111111-1111-1111-1111-111111111111",imageID:"22222222-2222-2222-2222-222222222222")`,
	} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()

			resolver := NewResolver(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			server := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
			body, err := json.Marshal(map[string]string{testGraphQLQueryKey: "mutation { " + mutation + " { id } }"})
			require.NoError(t, err)

			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, testGraphQLPath, bytes.NewReader(body))
			request.Header.Set(testContentTypeHeader, testJSONContentType)

			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)

			var result struct {
				Errors []struct {
					Extensions map[string]any `json:"extensions"`
				} `json:"errors"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
			require.Len(t, result.Errors, 1)
			require.Equal(t, testImageUnauthorizedCode, result.Errors[0].Extensions["code"])
		})
	}
}

// imageDeletionCatalogue models persistence and update notifications without an external database.
type imageDeletionCatalogue struct {
	teaData
	teaImages
	teaID       uuid.UUID
	imageID     uuid.UUID
	present     bool
	updateCount int
}

func (s *imageDeletionCatalogue) GetOrCreateUser(context.Context, string) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (s *imageDeletionCatalogue) DeleteTeaImage(_ context.Context, teaID, imageID uuid.UUID) error {
	if teaID != s.teaID || imageID != s.imageID || !s.present {
		return common.ErrTeaImageNotFound
	}

	s.present = false

	return nil
}

func (s *imageDeletionCatalogue) NotifyUpdate(context.Context, uuid.UUID) (*common.Tea, error) {
	s.updateCount++

	return &common.Tea{ID: s.teaID, TeaData: &common.TeaData{Name: testSharedTeaName, Type: common.TeaBeverageType}}, nil
}

func TestDeleteTeaImageRequiresAdmin(t *testing.T) {
	t.Parallel()

	for _, direct := range []bool{false, true} {
		for _, role := range []string{testAnonymousRole, testMobileUserRole, testAdminRole} {
			t.Run(role+"/"+map[bool]string{false: "GraphQL", true: "direct resolver"}[direct], func(t *testing.T) {
				t.Parallel()

				store := &imageDeletionCatalogue{teaID: uuid.New(), imageID: uuid.New(), present: true}
				auth, userToken, adminToken := imageTestAuth(t, store)
				resolver := NewResolver(nil, store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, store)
				token := map[string]string{testAnonymousRole: "", testMobileUserRole: userToken, testAdminRole: adminToken}[role]
				allowed := role == testAdminRole

				denialCode := testImageForbiddenCode
				if role == testAnonymousRole {
					denialCode = testImageUnauthorizedCode
				}

				if direct {
					payload := transport.InitPayload{}
					if token != "" {
						payload["Authorization"] = "Bearer " + token
					}

					ctx, _, err := auth.WsInitFunc(t.Context(), payload)
					require.NoError(t, err)

					result, err := resolver.Mutation().DeleteTeaImage(ctx, gqlCommon.ID(store.teaID), gqlCommon.ID(store.imageID), nil)
					if allowed {
						require.NoError(t, err)
						require.Equal(t, gqlCommon.ID(store.teaID), result.ID)
					} else {
						require.Nil(t, result)

						var gqlErr *gqlerror.Error
						require.ErrorAs(t, err, &gqlErr)
						require.Equal(t, denialCode, gqlErr.Extensions["code"])
					}
				} else {
					server := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolver}))
					server.Use(auth.Middleware())

					query := `mutation($tea:ID!,$image:ID!) { deleteTeaImage(teaID:$tea,imageID:$image) { id } }`
					vars := map[string]any{testTeaVariable: store.teaID.String(), testImageVariable: store.imageID.String()}

					reply := imageGraphQLCall(t, server, token, query, vars)
					if allowed {
						require.Equal(t, store.teaID.String(), decodeTeaImageReply(t, reply, testDeleteImageField).ID)
					} else {
						require.Len(t, reply.Errors, 1)
						require.Equal(t, denialCode, reply.Errors[0].Extensions["code"])
						require.Empty(t, reply.Data)
					}
				}

				require.Equal(t, !allowed, store.present, "denied deletion must preserve the shared image")

				if allowed {
					require.Equal(t, 1, store.updateCount)
				} else {
					require.Zero(t, store.updateCount, "denied deletion must not publish a catalog update")
				}
			})
		}
	}
}
