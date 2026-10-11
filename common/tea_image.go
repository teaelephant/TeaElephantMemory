package common

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Image limits apply to the shared tea catalog, independently of QR packages.
const (
	MaxTeaImages      = 10
	MaxTeaImageBytes  = 5 * 1024 * 1024
	MaxTeaImagePixels = 16_000_000
	MaxTeaImageEdge   = 8192
)

// ErrInvalidTeaImage indicates invalid bytes or dimensions.
var (
	ErrInvalidTeaImage = errors.New("image must be a valid JPEG or PNG within the image limits")
	// ErrTeaImageLimit indicates a full image gallery.
	ErrTeaImageLimit = errors.New("a tea can have at most 10 images")
	// ErrTeaImageNotFound indicates an image outside the requested tea.
	ErrTeaImageNotFound = errors.New("image does not belong to this tea")
	// ErrTeaNotFound indicates a missing catalog entry.
	ErrTeaNotFound = errors.New("tea not found")
)

// TeaImage is metadata for an immutable image owned by a catalog tea.
type TeaImage struct {
	ID          uuid.UUID
	TeaID       uuid.UUID
	ContentType string
	Width       int
	Height      int
	CreatedAt   time.Time
	IsCover     bool
}

// TeaImageContent includes the original validated bytes for HTTP delivery.
type TeaImageContent struct {
	TeaImage
	Data []byte
}
