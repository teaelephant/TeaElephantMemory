package graphql

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/teaelephant/TeaElephantMemory/common"
)

type GQLErrorCode int

const (
	ErrQRRecordNotExist GQLErrorCode = iota
	ErrExpiredToken
	ErrInvalidToken
	ErrUserNotFound
	ErrCollectionNotFound
)

var errorsMap = map[error]GQLErrorCode{
	common.ErrQRRecordNotExist:   ErrQRRecordNotExist,
	common.ErrExpiredToken:       ErrExpiredToken,
	common.ErrInvalidToken:       ErrInvalidToken,
	common.ErrUserNotFound:       ErrUserNotFound,
	common.ErrCollectionNotFound: ErrCollectionNotFound,
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
