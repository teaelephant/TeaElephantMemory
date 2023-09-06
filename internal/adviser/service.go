package adviser

import (
	"context"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
)

const template = "I can choose only this teas: %s and mix them only with this herbs: %s\n I have some criteria for choosing:\n 1. Current weather: %s\n2. Current time of day: %s, can you recommend tea for me?"

type Adviser interface {
	RecommendTea(ctx context.Context, teas []common.Tea, weather common.Weather, feelings string) (string, error)
}

type service struct {
	client *openai.Client
	log    *logrus.Entry
}

func (s *service) RecommendTea(
	ctx context.Context, teas []common.Tea, weather common.Weather, feelings string,
) (string, error) {
	teaString := ""
	herbsString := ""

	for _, tea := range teas {
		data := fmt.Sprintf("%s, ", tea.Name)
		switch tea.Type {
		case common.TeaBeverageType:
			teaString += data
		case common.HerbBeverageType:
			herbsString += data
		}
	}

	ifeel := ""
	if feelings != "" {
		ifeel = fmt.Sprintf(" 2. My feelings: %s", feelings)
	}

	content := fmt.Sprintf(template, teaString, herbsString, weather.String(), time.Now().Add(3*time.Hour).Format(time.TimeOnly)+ifeel)

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

func NewService(client *openai.Client, log *logrus.Entry) Adviser {
	return &service{client: client, log: log}
}
