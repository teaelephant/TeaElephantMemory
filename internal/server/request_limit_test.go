package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGraphQLRequestLimitRejectsOversizedBodyBeforeExecution(t *testing.T) {
	t.Parallel()

	for _, unknownLength := range []bool{false, true} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, v2QueryPath, strings.NewReader(strings.Repeat("x", 8*1024*1024+1)))
		if unknownLength {
			request.ContentLength = -1
		}

		executed := false
		next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { executed = true })
		response := httptest.NewRecorder()
		boundedGraphQLBody(next).ServeHTTP(response, request)
		require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
		require.False(t, executed, "oversized bodies must not reach authentication or mutation execution")
	}
}

func TestGraphQLRequestLimitPreservesValidBody(t *testing.T) {
	t.Parallel()

	const source = `{"query":"query { __typename }"}`

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, v2QueryPath, strings.NewReader(source))

	var received []byte

	var readErr error

	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		received, readErr = io.ReadAll(r.Body)
	})
	boundedGraphQLBody(next).ServeHTTP(httptest.NewRecorder(), request)
	require.NoError(t, readErr)
	require.JSONEq(t, source, string(received))
}
