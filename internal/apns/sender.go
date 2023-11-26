package apns

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/sirupsen/logrus"
)

type Sender interface {
	Send(ctx context.Context, userID, itemID uuid.UUID, title, body string) error
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

func (s *sender) Send(ctx context.Context, userID, itemID uuid.UUID, title, body string) error {
	deviceTokens, err := s.userIdMapper.MapUserIdToDeviceID(ctx, userID)
	if err != nil {
		return err
	}
	for _, device := range deviceTokens {
		notification := &apns2.Notification{
			DeviceToken: device,
			Topic:       s.topic,
			Expiration:  time.Now().Add(time.Minute * 15),
			Payload: payload.NewPayload().
				AlertTitle(title).
				AlertBody(body).
				Badge(1).
				Category("showCard").
				ThreadID(itemID.String()),
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
