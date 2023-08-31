package adviser

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
)

const template = "I have teas %scurrent weather is %s, can you recommend tea for me?"

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
	for _, tea := range teas {
		teaString += fmt.Sprintf("%s, ", tea.Name)
	}

	ifeel := ""
	if feelings != "" {
		ifeel = fmt.Sprintf(" and I feel %s", feelings)
	}

	resp, err := s.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(template, teaString, weather.String()+ifeel),
				},
			},
		},
	)

	if err != nil {
		s.log.WithError(err).Error("description generation error")
		return "", err
	}

	s.log.WithField("response", resp).Debug("recommendation result")

	return resp.Choices[0].Message.Content, nil
}

func NewService(client *openai.Client, log *logrus.Entry) Adviser {
	return &service{client: client, log: log}
}
