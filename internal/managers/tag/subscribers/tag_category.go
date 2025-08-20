package subscribers

import (
	"context"
	"sync"

	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

type tagCategorySubscriber struct {
	done <-chan struct{}
	ch   chan<- *model.TagCategory
}

type TagCategorySubscribers interface {
	Push(ctx context.Context, ch chan<- *model.TagCategory)
	SendAll(message *model.TagCategory)
	CleanDone()
}

type tagCategorySubscribers struct {
	mu   sync.RWMutex
	subs []tagCategorySubscriber
}

func (t *tagCategorySubscribers) CleanDone() {
	t.mu.Lock()
	defer t.mu.Unlock()

	forRemove := make([]int, 0)

	for i, sub := range t.subs {
		select {
		case <-sub.done:
			close(sub.ch)

			forRemove = append(forRemove, i)
		default:
		}
	}

	for j := len(forRemove) - 1; j >= 0; j-- {
		t.subs[forRemove[j]] = t.subs[len(t.subs)-len(forRemove)+j]
	}

	t.subs = t.subs[:len(t.subs)-len(forRemove)]
}

func (t *tagCategorySubscribers) SendAll(message *model.TagCategory) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, el := range t.subs {
		el.ch <- message
	}
}

func (t *tagCategorySubscribers) Push(ctx context.Context, ch chan<- *model.TagCategory) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.subs = append(t.subs, tagCategorySubscriber{
		done: ctx.Done(),
		ch:   ch,
	})
}

func NewTagCategorySubscribers() TagCategorySubscribers {
	return &tagCategorySubscribers{subs: make([]tagCategorySubscriber, 0)}
}
