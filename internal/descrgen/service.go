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

func (g *generator) GenerateDescription(ctx context.Context, name string) (string, error) {
	resp, err := g.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4TurboPreview,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf(requestTemplate, name),
				},
			},
		},
	)

	if err != nil {
		g.log.WithError(err).Error(descriptionGenerationError)
		return "", err
	}

	g.log.WithField("response", resp).Debug("description generation result")

	return resp.Choices[0].Message.Content, nil
}

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
}

func NewGenerator(token string, log *logrus.Entry) DescriptionGenerator {
	ristrettoCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000,
		MaxCost:     100,
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
