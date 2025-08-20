package subscribers

import (
	"context"
	"sync"
)

// Subscriber represents a single subscriber with a context and a channel
type Subscriber[T any] struct {
	ctx context.Context //nolint:containedctx
	ch  chan<- T
}

// Subscribers is the interface for managing subscribers of type T
type Subscribers[T any] interface {
	// Push adds a new subscriber
	Push(ctx context.Context, ch chan<- T)
	// SendAll sends a message to all subscribers
	SendAll(message T)
	// CleanDone removes subscribers whose context is done
	CleanDone()
}

// subscribersImpl is the implementation of Subscribers
type subscribersImpl[T any] struct {
	mu   sync.RWMutex
	subs []Subscriber[T]
}

// CleanDone removes subscribers whose context is done
func (s *subscribersImpl[T]) CleanDone() {
	s.mu.Lock()
	defer s.mu.Unlock()

	forRemove := make([]int, 0)

	for i, sub := range s.subs {
		select {
		case <-sub.ctx.Done():
			close(sub.ch)

			forRemove = append(forRemove, i)
		default:
		}
	}

	for j := len(forRemove) - 1; j >= 0; j-- {
		s.subs[forRemove[j]] = s.subs[len(s.subs)-len(forRemove)+j]
	}

	s.subs = s.subs[:len(s.subs)-len(forRemove)]
}

// SendAll sends a message to all subscribers
func (s *subscribersImpl[T]) SendAll(message T) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, el := range s.subs {
		el.ch <- message
	}
}

// Push adds a new subscriber
func (s *subscribersImpl[T]) Push(ctx context.Context, ch chan<- T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs = append(s.subs, Subscriber[T]{
		ctx: ctx,
		ch:  ch,
	})
}

// NewSubscribers creates a new Subscribers instance
func NewSubscribers[T any]() Subscribers[T] {
	return &subscribersImpl[T]{subs: make([]Subscriber[T], 0)}
}
