// Package graphql contains GraphQL schema-related helpers and error handling utilities.
package graphql

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/teaelephant/TeaElephantMemory/common"
)

// GQLErrorCode provides machine-readable codes for GraphQL errors returned to clients.
type GQLErrorCode int

// Predefined GraphQL error codes for commonly encountered errors.
const (
	ErrQRRecordNotExist GQLErrorCode = iota
	ErrExpiredToken
	ErrInvalidToken
	ErrUserNotFound
	ErrCollectionNotFound
	ErrDeviceNotFound
)

const badUserInputCode = "BAD_USER_INPUT"

var errorsMap = map[error]GQLErrorCode{
	common.ErrQRRecordNotExist:   ErrQRRecordNotExist,
	common.ErrExpiredToken:       ErrExpiredToken,
	common.ErrInvalidToken:       ErrInvalidToken,
	common.ErrUserNotFound:       ErrUserNotFound,
	common.ErrCollectionNotFound: ErrCollectionNotFound,
	common.ErrDeviceNotFound:     ErrDeviceNotFound,
}

func castGQLError(ctx context.Context, err error) error {
	extensions := map[string]interface{}{}

	// Standardize auth-related error codes for clients
	if errors.Is(err, common.ErrUnauthorized) {
		extensions["code"] = "UNAUTHENTICATED"
	} else if errors.Is(err, common.ErrNotAdmin) || errors.Is(err, common.ErrTeaRatingIneligible) {
		extensions["code"] = "FORBIDDEN"
	} else if errors.Is(err, common.ErrCatalogueConflict) {
		extensions["code"] = "CONFLICT"

		var conflict *common.CatalogueConflictError
		if errors.As(err, &conflict) {
			extensions["currentRevision"] = conflict.CurrentRevision
		}
	} else if errors.Is(err, common.ErrIdempotencyConflict) {
		extensions["code"] = "IDEMPOTENCY_CONFLICT"
	} else if errors.Is(err, common.ErrTeaMergePreviewConflict) {
		extensions["code"] = "PREVIEW_CONFLICT"
	} else if errors.Is(err, common.ErrTeaMergeBlocked) {
		extensions["code"] = "MERGE_BLOCKED"

		var blocked *common.TeaMergeBlockedError
		if errors.As(err, &blocked) {
			extensions["blockers"] = blocked.Blockers
		}
	} else if errors.Is(err, common.ErrTeaMergeUnavailable) || errors.Is(err, common.ErrTeaRatingsUnavailable) {
		extensions["code"] = "UNSUPPORTED"
	} else if errors.Is(err, common.ErrInvalidTeaMerge) || errors.Is(err, common.ErrInvalidTeaRating) {
		extensions["code"] = badUserInputCode
	} else if errors.Is(err, common.ErrInvalidCatalogueOperation) {
		extensions["code"] = badUserInputCode
	} else if errors.Is(err, common.ErrInvalidTeaImage) || errors.Is(err, common.ErrTeaImageLimit) {
		extensions["code"] = badUserInputCode
	} else if errors.Is(err, common.ErrTeaImageNotFound) || errors.Is(err, common.ErrTeaNotFound) || errors.Is(err, common.ErrTagNotFound) {
		extensions["code"] = "NOT_FOUND"
	} else if code, ok := errorsMap[err]; ok {
		extensions["code"] = code
	}

	return &gqlerror.Error{
		Message:    err.Error(),
		Path:       graphql.GetPath(ctx),
		Extensions: extensions,
	}
}
