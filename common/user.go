package common

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID      uuid.UUID
	AppleID string
	Session
}

type Session struct {
	JWT       string
	User      *User
	ExpiredAt time.Time
}
