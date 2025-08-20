package subscribers

import (
	"context"
	"sync"

	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

type tagSubscriber struct {
	done <-chan struct{}
	ch   chan<- *model.Tag
}

type TagSubscribers interface {
	Push(ctx context.Context, ch chan<- *model.Tag)
	SendAll(message *model.Tag)
	CleanDone()
}

type tagSubscribers struct {
	mu   sync.RWMutex
	subs []tagSubscriber
}

func (t *tagSubscribers) CleanDone() {
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

func (t *tagSubscribers) SendAll(message *model.Tag) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, el := range t.subs {
		el.ch <- message
	}
}

func (t *tagSubscribers) Push(ctx context.Context, ch chan<- *model.Tag) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.subs = append(t.subs, tagSubscriber{
		done: ctx.Done(),
		ch:   ch,
	})
}

func NewTagSubscribers() TagSubscribers {
	return &tagSubscribers{subs: make([]tagSubscriber, 0)}
}
