package subscribers

import (
	"context"
	"sync"

	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

type teaSubscriber struct {
	done <-chan struct{}
	ch   chan<- *model.Tea
}

type TeaSubscribers interface {
	Push(ctx context.Context, ch chan<- *model.Tea)
	SendAll(message *model.Tea)
	CleanDone()
}

type teaSubscribers struct {
	mu   sync.RWMutex
	subs []teaSubscriber
}

func (t *teaSubscribers) CleanDone() {
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

func (t *teaSubscribers) SendAll(message *model.Tea) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, el := range t.subs {
		el.ch <- message
	}
}

func (t *teaSubscribers) Push(ctx context.Context, ch chan<- *model.Tea) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.subs = append(t.subs, teaSubscriber{
		done: ctx.Done(),
		ch:   ch,
	})
}

func NewTeaSubscribers() TeaSubscribers {
	return &teaSubscribers{subs: make([]teaSubscriber, 0)}
}
