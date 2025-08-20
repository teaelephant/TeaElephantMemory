package adviser

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"text/template"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
)

const (
	prompt             = "prompt.gotpl"
	contextScoringTmpl = "context_scoring.gotpl"
)

//go:embed prompt.gotpl context_scoring.gotpl
var f embed.FS

type Adviser interface {
	RecommendTea(ctx context.Context, teas []common.Tea, weather common.Weather, feelings string) (string, error)
	RecommendTeaStream(ctx context.Context, teas []common.Tea, weather common.Weather, feelings string, res chan<- string) error
	ContextScores(ctx context.Context, teas []string, weather common.Weather, day time.Weekday) (map[string]int, error)
	LoadPrompt() error
}

type service struct {
	client      *openai.Client
	log         *logrus.Entry
	tmpl        *template.Template
	contextTmpl *template.Template
}

func (s *service) RecommendTea(
	ctx context.Context, teas []common.Tea, weather common.Weather, feelings string,
) (string, error) {
	t := s.sortTeas(teas, weather, feelings)

	content, err := s.execute(t)
	if err != nil {
		return "", err
	}

	resp, err := s.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT5,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: content,
				},
			},
		},
	)

	if err != nil {
		s.log.WithError(err).Error("description generation error")
		return "", err
	}

	s.log.WithField("request", content).WithField("response", resp).Debug("recommendation result")

	return resp.Choices[0].Message.Content, nil
}

func (s *service) sortTeas(teas []common.Tea, weather common.Weather, feelings string) Template {
	t := Template{
		Teas:      make([]common.Tea, 0),
		Additives: make([]common.Tea, 0),
		Weather:   weather,
		TimeOfDay: time.Now().Add(3 * time.Hour).Format(time.TimeOnly),
		Feelings:  Feelings(feelings),
	}

	for _, tea := range teas {
		switch tea.Type {
		case common.TeaBeverageType:
			t.Teas = append(t.Teas, tea)
		case common.HerbBeverageType:
			t.Additives = append(t.Additives, tea)
		default:
		}
	}

	return t
}

func (s *service) RecommendTeaStream(
	ctx context.Context, teas []common.Tea, weather common.Weather, feelings string, res chan<- string,
) error {
	t := s.sortTeas(teas, weather, feelings)

	content, err := s.execute(t)
	if err != nil {
		return err
	}

	req := openai.ChatCompletionRequest{
		Model: openai.GPT4oLatest,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: content,
			},
		},
	}

	stream, err := s.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		s.log.WithError(err).Error("description generation error")
		return err
	}

	go s.readStream(stream, res)

	return nil
}

func (s *service) readStream(stream *openai.ChatCompletionStream, res chan<- string) {
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			s.log.Debug("stream finished")
			break
		}

		if err != nil {
			s.log.WithError(err).Debug("stream error")
			break
		}

		res <- response.Choices[0].Delta.Content
	}

	close(res)
}

func (s *service) LoadPrompt() error {
	tmpl, err := template.New("").ParseFS(f, prompt, contextScoringTmpl)
	if err != nil {
		return err
	}

	s.tmpl = tmpl.Lookup(prompt)
	s.contextTmpl = tmpl.Lookup(contextScoringTmpl)

	return nil
}

func (s *service) execute(params Template) (string, error) {
	buf := bytes.NewBuffer(make([]byte, 0))

	err := s.tmpl.ExecuteTemplate(buf, prompt, params)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (s *service) ContextScores(ctx context.Context, teas []string, weather common.Weather, day time.Weekday) (map[string]int, error) {
	type params struct {
		Teas      []string
		Weather   common.Weather
		DayOfWeek string
	}

	p := params{Teas: teas, Weather: weather, DayOfWeek: day.String()}
	var tpl bytes.Buffer
	if err := s.contextTmpl.Execute(&tpl, p); err != nil {
		return nil, err
	}

	resp, err := s.getCompletion(ctx, tpl.String())
	if err != nil {
		return nil, err
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(resp), &raw); err != nil {
		return nil, err
	}
	res := make(map[string]int, len(raw))
	for k, v := range raw {
		switch t := v.(type) {
		case float64:
			res[k] = int(t)
		case int:
			res[k] = t
		default:
			continue
		}
		if res[k] < 0 {
			res[k] = 0
		}
		if res[k] > 15 {
			res[k] = 15
		}
	}
	return res, nil
}

func (s *service) getCompletion(ctx context.Context, content string) (string, error) {
	resp, err := s.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT5,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: content,
				},
			},
		},
	)
	if err != nil {
		s.log.WithError(err).Error("tea of the day generation error")
		return "", err
	}

	s.log.WithField("request", content).WithField("response", resp).Debug("tea of the day result")
	return resp.Choices[0].Message.Content, nil
}

func NewService(client *openai.Client, log *logrus.Entry) Adviser {
	return &service{client: client, log: log}
}
