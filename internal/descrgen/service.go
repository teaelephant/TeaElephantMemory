package descrgen

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	ristrettoStore "github.com/eko/gocache/store/ristretto/v4"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

const (
	asyncGenerateTimeout       = 5 * time.Minute
	requestTemplate            = "Опиши взвешенно и информативно без маркетинга напиток %s, чтобы помочь сделать выбор человеку на основании вкусовых качеств, пользы для организма, стимуляции к деятельности" //nolint:lll
	descriptionGenerationError = "description generation error"
	descriptionModel           = openai.GPT5
)

type DescriptionGenerator interface {
	GenerateDescription(ctx context.Context, name string) (string, error)
	StartGenerateDescription(ctx context.Context, name string, res chan<- string) error
}

type generator struct {
	client       *openai.Client
	cacheManager *cache.Cache[string]

	log *logrus.Entry
}

func (g *generator) GenerateDescription(ctx context.Context, productName string) (string, error) {
	request := g.createChatCompletionRequest(productName)
	descriptionResponse, err := g.client.CreateChatCompletion(ctx, request)

	if err != nil {
		g.log.WithError(err).Error(descriptionGenerationError)
		return "", err
	}

	g.log.WithField("response", descriptionResponse).Debug("description generation result")

	return descriptionResponse.Choices[0].Message.Content, nil
}

func (g *generator) createChatCompletionRequest(name string) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model: descriptionModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf(requestTemplate, name),
			},
		},
	}
}

// StartGenerateDescription generates a description for a given name.
// It first checks if the description is available in the cache.
// If not, it starts a goroutine to generate the description
func (g *generator) StartGenerateDescription(ctx context.Context, name string, result chan<- string) error {
	res, err := g.cacheManager.Get(ctx, name)
	if err != nil {
		if errors.Is(err, store.NotFound{}) {
			go g.generateDescription(name, result) //nolint:contextcheck

			return nil
		}

		g.log.WithError(err).Error("cache get error")

		return err
	}

	result <- res

	close(result)

	return nil
}

func (g *generator) generateDescription(name string, result chan<- string) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, asyncGenerateTimeout)

	g.log.WithField("name", name).Debug("start generate description")

	res, err := g.GenerateDescription(ctx, name)
	if err != nil {
		g.log.WithError(err).Error(descriptionGenerationError)
	}

	if err = g.cacheManager.Set(ctx, name, res); err != nil {
		g.log.WithError(err).Error("cache set error")
	}

	cancel()

	result <- res

	close(result)
}

func NewGenerator(token string, log *logrus.Entry) DescriptionGenerator {
	ristrettoCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000,
		MaxCost:     100000000,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}

	return &generator{
		client:       openai.NewClient(token),
		cacheManager: cache.New[string](ristrettoStore.NewRistretto(ristrettoCache)),
		log:          log,
	}
}
