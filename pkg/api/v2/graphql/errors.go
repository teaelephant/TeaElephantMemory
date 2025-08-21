// Package graphql contains GraphQL schema-related helpers and error handling utilities.
package graphql

import (
	"context"

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

	code, ok := errorsMap[err]
	if ok {
		extensions["code"] = code
	}

	return &gqlerror.Error{
		Message:    err.Error(),
		Path:       graphql.GetPath(ctx),
		Extensions: extensions,
	}
}
