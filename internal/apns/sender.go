package apns

import (
	"context"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/sirupsen/logrus"
)

type Sender interface {
	Send(ctx context.Context, userID uuid.UUID) error
}

type userIdMapper interface {
	MapUserIdToDeviceID(ctx context.Context, userID uuid.UUID) ([]string, error)
}

type sender struct {
	client *apns2.Client
	userIdMapper

	log   *logrus.Entry
	topic string
}

func (s *sender) Send(ctx context.Context, userID uuid.UUID) error {
	deviceTokens, err := s.userIdMapper.MapUserIdToDeviceID(ctx, userID)
	if err != nil {
		return err
	}
	for _, device := range deviceTokens {
		notification := &apns2.Notification{
			DeviceToken: device,
			Topic:       s.topic,
			Expiration:  time.Now().Add(time.Minute * 15), // TODO validate and
			Payload:     payload.NewPayload().AlertTitle("TEST").AlertBody("TEST").Badge(1),
		}
		res, err := s.client.PushWithContext(ctx, notification)
		if err != nil {
			return err
		}
		if res.Sent() {
			s.log.WithField("apns id", res.ApnsID).Debug("Sent signal")
		} else {
			s.log.
				WithField("host", s.client.Host).
				WithField("status code", res.StatusCode).
				WithField("apns id", res.ApnsID).
				WithField("reason", res.Reason).
				Warn("Notification not Sent")
		}
	}
	return nil
}

func NewSender(client *apns2.Client, topic string, mapper userIdMapper, log *logrus.Entry) Sender {
	return &sender{
		topic:        topic,
		client:       client,
		userIdMapper: mapper,
		log:          log,
	}
}
