// Package apns provides Apple Push Notification sending.
package apns

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/sirupsen/logrus"
)

const apnsIDField = "apns id"

// Sender delivers APNS notifications for a user's devices.
type Sender interface {
	Send(ctx context.Context, userID, itemID uuid.UUID, title, body string) error
}

type userIDMapper interface {
	MapUserIDToDeviceID(ctx context.Context, userID uuid.UUID) ([]string, error)
}

type sender struct {
	client *apns2.Client
	userIDMapper

	log   *logrus.Entry
	topic string
}

func (s *sender) Send(ctx context.Context, userID, itemID uuid.UUID, title, body string) error {
	deviceTokens, err := s.MapUserIDToDeviceID(ctx, userID)
	if err != nil {
		return fmt.Errorf("map user id to device id: %w", err)
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
			return fmt.Errorf("push apns: %w", err)
		}

		if res.Sent() {
			s.log.WithField(apnsIDField, res.ApnsID).Debug("Sent signal")
		} else {
			s.log.
				WithField("host", s.client.Host).
				WithField("status code", res.StatusCode).
				WithField(apnsIDField, res.ApnsID).
				WithField("reason", res.Reason).
				Warn("Notification not Sent")
		}
	}

	return nil
}

// NewSender constructs a new APNS sender with a client, topic, mapper and logger.
func NewSender(client *apns2.Client, topic string, mapper userIDMapper, log *logrus.Entry) Sender {
	return &sender{
		topic:        topic,
		client:       client,
		userIDMapper: mapper,
		log:          log,
	}
}
