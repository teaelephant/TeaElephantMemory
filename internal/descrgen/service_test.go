package descrgen

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eko/gocache/lib/v4/store"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const teaName = "Earl Grey"

func TestStartGenerateDescriptionReturnsCachedValue(t *testing.T) {
	t.Parallel()

	g, ok := NewGenerator("", logrus.NewEntry(logrus.New())).(*generator)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, g.cacheManager.Close()) })

	const cachedDescription = "Cached description"

	ctx := t.Context()
	require.NoError(t, g.cacheManager.Set(ctx, teaName, cachedDescription, store.WithSynchronousSet()))

	result := make(chan string, 1)
	require.NoError(t, g.StartGenerateDescription(ctx, teaName, result))

	description, open := <-result
	require.True(t, open)
	require.Equal(t, cachedDescription, description)

	_, open = <-result
	require.False(t, open)
}

func TestDescriptionCacheMissMatchesNotFound(t *testing.T) {
	t.Parallel()

	g, ok := NewGenerator("", logrus.NewEntry(logrus.New())).(*generator)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, g.cacheManager.Close()) })

	_, err := g.cacheManager.Get(t.Context(), "Missing tea")
	require.ErrorIs(t, err, store.NotFound{})
}

func TestGenerateDescriptionUsesCurrentModel(t *testing.T) {
	t.Parallel()

	requests := make(chan openai.ChatCompletionRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode completion request: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)

			return
		}

		requests <- request

		w.Header().Set("Content-Type", "application/json")

		if _, err := io.WriteString(w, `{"choices":[{"message":{"content":"A balanced tea description"}}]}`); err != nil {
			t.Errorf("write completion response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	config := openai.DefaultConfig("test-token")
	config.BaseURL = server.URL
	config.HTTPClient = server.Client()
	g := &generator{client: openai.NewClientWithConfig(config), log: logrus.NewEntry(logrus.New())}

	description, err := g.GenerateDescription(t.Context(), teaName)
	require.NoError(t, err)
	require.Equal(t, "A balanced tea description", description)

	request := <-requests
	require.Equal(t, openai.GPT6Astra, request.Model)
	require.Contains(t, request.Messages[0].Content, teaName)
	require.Empty(t, request.ReasoningEffort)
	require.Zero(t, request.Temperature)
}
