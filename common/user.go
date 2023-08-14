package common

import uuid "github.com/satori/go.uuid"

type User struct {
	ID      uuid.UUID
	AppleID string
}
