package common

import uuid "github.com/satori/go.uuid"

type TagCategory struct {
	ID   uuid.UUID
	Name string
}

type Tag struct {
	ID uuid.UUID
	*TagData
}

type TagData struct {
	Name       string
	Color      string
	CategoryID uuid.UUID
}
