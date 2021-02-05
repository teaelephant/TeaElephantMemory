package common

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

type TeaData struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type Tea struct {
	ID uuid.UUID
	*TeaData
}

type QR struct {
	Tea            uuid.UUID
	BowlingTemp    int
	ExpirationDate time.Time
}
