package subscribers

import (
	"context"

	commonSubs "github.com/teaelephant/TeaElephantMemory/common/subscribers"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
)

// IDSubscribers is an interface for managing ID subscribers
type IDSubscribers interface {
	Push(ctx context.Context, ch chan<- common.ID)
	SendAll(message common.ID)
	CleanDone()
}

// idSubscribersImpl implements IDSubscribers using the generic implementation
type idSubscribersImpl struct {
	subs commonSubs.Subscribers[common.ID]
}

// CleanDone removes subscribers whose context is done
func (t *idSubscribersImpl) CleanDone() {
	t.subs.CleanDone()
}

// SendAll sends a message to all subscribers
func (t *idSubscribersImpl) SendAll(message common.ID) {
	t.subs.SendAll(message)
}

// Push adds a new subscriber
func (t *idSubscribersImpl) Push(ctx context.Context, ch chan<- common.ID) {
	t.subs.Push(ctx, ch)
}

// NewIDSubscribers creates a new IDSubscribers instance
func NewIDSubscribers() IDSubscribers {
	return &idSubscribersImpl{
		subs: commonSubs.NewSubscribers[common.ID](),
	}
}
