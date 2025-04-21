package subscribers

import (
	"context"

	commonSubs "github.com/teaelephant/TeaElephantMemory/common/subscribers"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

// TagSubscribers is an interface for managing Tag subscribers
type TagSubscribers interface {
	Push(ctx context.Context, ch chan<- *model.Tag)
	SendAll(message *model.Tag)
	CleanDone()
}

// tagSubscribersImpl implements TagSubscribers using the generic implementation
type tagSubscribersImpl struct {
	subs commonSubs.Subscribers[*model.Tag]
}

// CleanDone removes subscribers whose context is done
func (t *tagSubscribersImpl) CleanDone() {
	t.subs.CleanDone()
}

// SendAll sends a message to all subscribers
func (t *tagSubscribersImpl) SendAll(message *model.Tag) {
	t.subs.SendAll(message)
}

// Push adds a new subscriber
func (t *tagSubscribersImpl) Push(ctx context.Context, ch chan<- *model.Tag) {
	t.subs.Push(ctx, ch)
}

// NewTagSubscribers creates a new TagSubscribers instance
func NewTagSubscribers() TagSubscribers {
	return &tagSubscribersImpl{
		subs: commonSubs.NewSubscribers[*model.Tag](),
	}
}
