package adviser

import (
	"bytes"
	"context"
	"embed"
	"text/template"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
)

const prompt = "prompt.gotpl"

//go:embed prompt.gotpl
var f embed.FS

type Adviser interface {
	RecommendTea(ctx context.Context, teas []common.Tea, weather common.Weather, feelings string) (string, error)
	LoadPrompt() error
}

type service struct {
	client *openai.Client
	log    *logrus.Entry
	tmpl   *template.Template
}

func (s *service) RecommendTea(
	ctx context.Context, teas []common.Tea, weather common.Weather, feelings string,
) (string, error) {
	t := Template{
		Teas:      make([]common.Tea, 0),
		Additives: make([]common.Tea, 0),
		Weather:   weather,
		TimeOfDay: time.Now().Add(3 * time.Hour).Format(time.TimeOnly),
		Feelings:  Feelings(feelings),
	}

	for _, tea := range teas {
		switch tea.Type { //nolint:exhaustive
		case common.TeaBeverageType:
			t.Teas = append(t.Teas, tea)
		case common.HerbBeverageType:
			t.Additives = append(t.Additives, tea)
		}
	}

	content, err := s.execute(t)
	if err != nil {
		return "", err
	}

	resp, err := s.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
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

func (s *service) LoadPrompt() error {
	tmpl, err := template.New("").ParseFS(f, prompt)
	if err != nil {
		return err
	}

	s.tmpl = tmpl

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

func NewService(client *openai.Client, log *logrus.Entry) Adviser {
	return &service{client: client, log: log}
}
