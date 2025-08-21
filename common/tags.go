// Package common contains shared domain models used across the application.
package common

import (
	"github.com/google/uuid"
)

// TagCategory represents a category to which tags belong (e.g., flavor, origin).
type TagCategory struct {
	ID   uuid.UUID
	Name string
}

// Tag describes a label that can be attached to a tea (e.g., "bergamot", "green").
type Tag struct {
	ID uuid.UUID
	*TagData
}

// TagData holds mutable tag fields.
type TagData struct {
	Name       string
	Color      string
	CategoryID uuid.UUID
}
