package auth

import (
	"context"

	uuid "github.com/satori/go.uuid"
)

var uid = [uuid.Size]byte{}

type Auth interface {
	CheckToken(ctx context.Context, token string) (uuid.UUID, error)
}

type auth struct {
}

func (a *auth) CheckToken(ctx context.Context, token string) (uuid.UUID, error) {
	return uid, nil
}

func NewAuth() Auth {
	return &auth{}
}
