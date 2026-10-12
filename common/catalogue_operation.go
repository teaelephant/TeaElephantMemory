package common

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrCatalogueConflict refuses stale edits without changing the catalog.
	ErrCatalogueConflict = errors.New("catalog changed; read its current revision before editing")
	// ErrIdempotencyConflict refuses reuse of a key for different arguments.
	ErrIdempotencyConflict = errors.New("idempotency key was already used for a different operation")
	// ErrInvalidCatalogueOperation identifies malformed optional mutation arguments.
	ErrInvalidCatalogueOperation = errors.New("expectedRevision must be a positive revision and idempotencyKey must be a nonzero UUID")
)

type catalogueOperationKey struct{}

// CatalogueWriteLock orders household catalog changes and selected-QR history writes.
const CatalogueWriteLock int64 = 724383810271

// CatalogueOperation carries validated mutation preconditions to the storage boundary.
type CatalogueOperation struct {
	ExpectedRevision    string
	HasExpectedRevision bool
	Key                 uuid.UUID
	HasKey              bool
	Actor               string
}

// WithCatalogueOperation validates optional, backwards-compatible mutation arguments.
func WithCatalogueOperation(ctx context.Context, revision, key *string, actor string) (context.Context, error) {
	operation := CatalogueOperation{Actor: actor}

	if revision != nil {
		number, err := strconv.ParseInt(*revision, 10, 64)
		if err != nil || number < 1 || strconv.FormatInt(number, 10) != *revision {
			return nil, ErrInvalidCatalogueOperation
		}

		operation.ExpectedRevision, operation.HasExpectedRevision = *revision, true
	}

	if key != nil {
		id, err := uuid.Parse(*key)
		if err != nil || id == uuid.Nil {
			return nil, ErrInvalidCatalogueOperation
		}

		operation.Key, operation.HasKey = id, true
	}

	return context.WithValue(ctx, catalogueOperationKey{}, operation), nil
}

// CatalogueOperationFrom returns the options supplied for this individual mutation.
func CatalogueOperationFrom(ctx context.Context) CatalogueOperation {
	result, _ := ctx.Value(catalogueOperationKey{}).(CatalogueOperation)
	return result
}

// CatalogueReceipt identifies a committed create/upload. It contains no image bytes or credentials.
type CatalogueReceipt struct {
	Key            uuid.UUID
	Actor          string
	Operation      string
	TeaID          uuid.UUID
	ResourceID     uuid.UUID
	ResultRevision string
	CompletedAt    time.Time
}

// CatalogueConflictError carries the actual version without weakening errors.Is checks.
type CatalogueConflictError struct {
	CurrentRevision string
}

func (*CatalogueConflictError) Error() string { return ErrCatalogueConflict.Error() }
func (*CatalogueConflictError) Unwrap() error { return ErrCatalogueConflict }
