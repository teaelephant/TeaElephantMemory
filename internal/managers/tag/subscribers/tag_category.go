package subscribers

import (
	"context"

	commonSubs "github.com/teaelephant/TeaElephantMemory/common/subscribers"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

// TagCategorySubscribers is an interface for managing TagCategory subscribers
type TagCategorySubscribers interface {
	Push(ctx context.Context, ch chan<- *model.TagCategory)
	SendAll(message *model.TagCategory)
	CleanDone()
}

// tagCategorySubscribersImpl implements TagCategorySubscribers using the generic implementation
type tagCategorySubscribersImpl struct {
	subs commonSubs.Subscribers[*model.TagCategory]
}

// CleanDone removes subscribers whose context is done
func (t *tagCategorySubscribersImpl) CleanDone() {
	t.subs.CleanDone()
}

// SendAll sends a message to all subscribers
func (t *tagCategorySubscribersImpl) SendAll(message *model.TagCategory) {
	t.subs.SendAll(message)
}

// Push adds a new subscriber
func (t *tagCategorySubscribersImpl) Push(ctx context.Context, ch chan<- *model.TagCategory) {
	t.subs.Push(ctx, ch)
}

// NewTagCategorySubscribers creates a new TagCategorySubscribers instance
func NewTagCategorySubscribers() TagCategorySubscribers {
	return &tagCategorySubscribersImpl{
		subs: commonSubs.NewSubscribers[*model.TagCategory](),
	}
}
