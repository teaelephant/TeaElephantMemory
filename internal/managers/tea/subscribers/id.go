package subscribers

import (
	"context"
	"sync"

	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
)

type idSubscriber struct {
	done <-chan struct{}
	ch   chan<- common.ID
}

type IDSubscribers interface {
	Push(ctx context.Context, ch chan<- common.ID)
	SendAll(message common.ID)
	CleanDone()
}

type idSubscribers struct {
	mu   sync.RWMutex
	subs []idSubscriber
}

func (t *idSubscribers) CleanDone() {
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

func (t *idSubscribers) SendAll(message common.ID) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, el := range t.subs {
		el.ch <- message
	}
}

func (t *idSubscribers) Push(ctx context.Context, ch chan<- common.ID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.subs = append(t.subs, idSubscriber{
		done: ctx.Done(),
		ch:   ch,
	})
}

func NewIDSubscribers() IDSubscribers {
	return &idSubscribers{subs: make([]idSubscriber, 0)}
}
