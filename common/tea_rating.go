package common

import (
	"errors"

	"github.com/google/uuid"
)

// Rating errors distinguish invalid votes, membership denial and an unavailable adapter.
var (
	ErrInvalidTeaRating      = errors.New("Tea rating must be an integer from 1 to 5")
	ErrTeaRatingIneligible   = errors.New("add this Tea to one of your collections before setting a rating")
	ErrTeaRatingsUnavailable = errors.New("Tea ratings are unavailable")
)

// TeaRating contains public aggregates and only the current viewer's personal vote.
type TeaRating struct {
	TeaID    uuid.UUID
	Average  *float64
	Count    int
	MyRating *int
	CanRate  bool
}
