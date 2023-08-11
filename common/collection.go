package common

import uuid "github.com/satori/go.uuid"

type Collection struct {
	ID   uuid.UUID
	Name string
}
