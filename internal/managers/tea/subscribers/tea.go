package subscribers

import (
	"context"

	commonSubs "github.com/teaelephant/TeaElephantMemory/common/subscribers"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

// TeaSubscribers is an interface for managing Tea subscribers
type TeaSubscribers interface {
	Push(ctx context.Context, ch chan<- *model.Tea)
	SendAll(message *model.Tea)
	CleanDone()
}

// teaSubscribersImpl implements TeaSubscribers using the generic implementation
type teaSubscribersImpl struct {
	subs commonSubs.Subscribers[*model.Tea]
}

// CleanDone removes subscribers whose context is done
func (t *teaSubscribersImpl) CleanDone() {
	t.subs.CleanDone()
}

// SendAll sends a message to all subscribers
func (t *teaSubscribersImpl) SendAll(message *model.Tea) {
	t.subs.SendAll(message)
}

// Push adds a new subscriber
func (t *teaSubscribersImpl) Push(ctx context.Context, ch chan<- *model.Tea) {
	t.subs.Push(ctx, ch)
}

// NewTeaSubscribers creates a new TeaSubscribers instance
func NewTeaSubscribers() TeaSubscribers {
	return &teaSubscribersImpl{
		subs: commonSubs.NewSubscribers[*model.Tea](),
	}
}
