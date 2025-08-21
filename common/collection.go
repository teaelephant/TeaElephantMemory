// Package common contains shared domain models used across the application.
package common

import (
	"time"

	"github.com/google/uuid"
)

// Collection represents a user-defined grouping of QR tea records.
type Collection struct {
	ID   uuid.UUID
	Name string
}

// CollectionRecord represents a QR-coded tea item tracked in a Collection.
type CollectionRecord struct {
	ID             uuid.UUID
	Tea            *Tea
	BowlingTemp    int
	ExpirationDate time.Time
}
