package common

import "time"

type TeaData struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type Tea struct {
	ID string
	*TeaData
}

type QR struct {
	Tea            string
	BowlingTemp    int
	ExpirationDate time.Time
}
