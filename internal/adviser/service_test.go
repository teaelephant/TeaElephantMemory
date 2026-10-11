package adviser

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teaelephant/TeaElephantMemory/common"
)

const (
	completionContentType = "application/json"
	testTeaName           = "Earl Grey"
	testFeelings          = "relaxed"
	testRecommendation    = "Try Earl Grey"
)

func Test_service_execute(t *testing.T) {
	t.Run("execute prompt", func(t *testing.T) {
		s := &service{}
		require.NoError(t, s.LoadPrompt())
		got, err := s.execute(Template{
			Teas:      []common.Tea{{TeaData: &common.TeaData{Name: "example tea"}}},
			Additives: []common.Tea{{TeaData: &common.TeaData{Name: "example additives"}}},
			Weather: common.Weather{
				Temperature: 3,
				Clouds:      4,
				Rain:        2,
				Humidity:    1,
				WindSpeed:   2,
				Visibility:  3,
			},
			TimeOfDay: "10:00",
			Feelings:  "better now",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, got)
	})
}

func TestRecommendTeaUsesCurrentModel(t *testing.T) {
	t.Parallel()

	client, requests := completionTestClient(t, completionContentType, `{"choices":[{"message":{"content":"Try Earl Grey"}}]}`)
	s := NewService(client, logrus.NewEntry(logrus.New()))
	require.NoError(t, s.LoadPrompt())

	recommendation, err := s.RecommendTea(t.Context(), nil, common.Weather{}, testFeelings)
	require.NoError(t, err)
	require.Equal(t, testRecommendation, recommendation)

	request := <-requests
	require.Equal(t, openai.GPT6Astra, request.Model)
	require.False(t, request.Stream)
	require.Empty(t, request.ReasoningEffort)
}

func TestContextScoresUsesCurrentModel(t *testing.T) {
	t.Parallel()

	client, requests := completionTestClient(t, completionContentType, `{"choices":[{"message":{"content":"{\"Earl Grey\":12}"}}]}`)
	s := NewService(client, logrus.NewEntry(logrus.New()))
	require.NoError(t, s.LoadPrompt())

	scores, err := s.ContextScores(t.Context(), []string{testTeaName}, common.Weather{}, time.Monday)
	require.NoError(t, err)
	require.Equal(t, map[string]int{testTeaName: 12}, scores)

	request := <-requests
	require.Equal(t, openai.GPT6Astra, request.Model)
	require.False(t, request.Stream)
	require.Empty(t, request.ReasoningEffort)
}

func TestRecommendTeaStreamUsesCurrentModel(t *testing.T) {
	t.Parallel()

	const response = "data: {\"choices\":[{\"delta\":{\"content\":\"Try \"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"Earl Grey\"}}]}\n\n" +
		"data: [DONE]\n\n"

	client, requests := completionTestClient(t, "text/event-stream", response)
	s := NewService(client, logrus.NewEntry(logrus.New()))
	require.NoError(t, s.LoadPrompt())

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	result := make(chan string)
	require.NoError(t, s.RecommendTeaStream(ctx, nil, common.Weather{}, testFeelings, result))

	var recommendation string

	for part := range result {
		recommendation += part
	}

	require.Equal(t, testRecommendation, recommendation)

	request := <-requests
	require.Equal(t, openai.GPT6Luna, request.Model)
	require.Equal(t, openai.ReasoningEffortNone, request.ReasoningEffort)
	require.True(t, request.Stream)
}

func completionTestClient(t *testing.T, contentType, response string) (*openai.Client, <-chan openai.ChatCompletionRequest) {
	t.Helper()

	requests := make(chan openai.ChatCompletionRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode completion request: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)

			return
		}

		requests <- request

		w.Header().Set("Content-Type", contentType)

		if _, err := io.WriteString(w, response); err != nil {
			t.Errorf("write completion response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	config := openai.DefaultConfig("test-token")
	config.BaseURL = server.URL
	config.HTTPClient = server.Client()

	return openai.NewClientWithConfig(config), requests
}
