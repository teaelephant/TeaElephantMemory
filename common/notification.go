package common

import "github.com/google/uuid"

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
