// Package common contains shared domain models used across the application.
//
//revive:disable:var-naming // keep package name for compatibility across modules
package common

//revive:enable:var-naming

import "github.com/google/uuid"

// NotificationType enumerates the kinds of notifications that can be sent to users.
const (
	// NotificationTypeTeaExpiration notifies about upcoming tea expiration.
	NotificationTypeTeaExpiration NotificationType = iota
	// NotificationTypeTeaRecommendation suggests a tea to drink.
	NotificationTypeTeaRecommendation
)

// NotificationType is the domain-level enum of notification categories.
type NotificationType int

// Notification describes a notification event for a user.
type Notification struct {
	UserID uuid.UUID
	Type   NotificationType
}

// Device represents a user device eligible to receive push notifications.
type Device struct {
	UserID uuid.UUID
	Token  string
}
