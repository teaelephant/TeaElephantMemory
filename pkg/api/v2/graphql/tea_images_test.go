package graphql

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/stretchr/testify/require"

	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql/generated"
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
			require.Equal(t, "UNAUTHENTICATED", result.Errors[0].Extensions["code"])
		})
	}
}
