package subscribers

import (
	"context"
	"testing"
	"time"

	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

func TestTeaSubscribersSendAllDoesNotBlockOnUnreadSubscriber(t *testing.T) {
	subs := NewTeaSubscribers()
	subs.Push(context.Background(), make(chan *model.Tea))

	done := make(chan struct{})
	go func() {
		subs.SendAll(&model.Tea{Name: "Cinnamon"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("SendAll blocked on a subscriber that was not reading")
	}
}
