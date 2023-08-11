package common

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type Collection struct {
	ID   uuid.UUID
	Name string
}

type CollectionRecord struct {
	ID             uuid.UUID
	Tea            *Tea
	BowlingTemp    int
	ExpirationDate time.Time
}
