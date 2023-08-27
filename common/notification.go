package common

import uuid "github.com/satori/go.uuid"

const (
	NotificationTypeTeaExpiration NotificationType = iota
	NotificationTypeTeaRecommendation
)

type NotificationType int

type Notification struct {
	UserID uuid.UUID
	Type   NotificationType
}

type Device struct {
	UserID uuid.UUID
	Token  string
}
