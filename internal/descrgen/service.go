package descrgen

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

const requestTemplate = "Опиши взвешенно и информативно чай %s, чтобы помочь сделать выбор человеку который хочет попить чай"

type DescriptionGenerator interface {
	GenerateDescription(ctx context.Context, name string) (string, error)
}

type generator struct {
	client *openai.Client
}

func (g *generator) GenerateDescription(ctx context.Context, name string) (string, error) {
	resp, err := g.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(requestTemplate, name),
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

func NewGenerator(token string) DescriptionGenerator {
	return &generator{client: openai.NewClient(token)}
}
