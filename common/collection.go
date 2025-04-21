package common

import (
	"time"

	"github.com/google/uuid"
)

// Collection tea collection struct
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
