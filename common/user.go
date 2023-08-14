package common

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type User struct {
	ID      uuid.UUID
	AppleID string
}

type Session struct {
	JWT       string
	User      *User
	ExpiredAt time.Time
}
