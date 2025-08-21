// Package common contains shared domain models used across the application.
package common

import (
	"time"

	"github.com/google/uuid"
)

// User represents an authenticated TeaElephant user.
type User struct {
	ID      uuid.UUID
	AppleID string
	Session
}

// Session contains JWT and expiration metadata for a user session.
type Session struct {
	JWT       string
	User      *User
	ExpiredAt time.Time
}
