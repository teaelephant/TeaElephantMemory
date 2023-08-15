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
)

var errorsMap = map[error]GQLErrorCode{
	common.ErrQRRecordNotExist: ErrQRRecordNotExist,
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
