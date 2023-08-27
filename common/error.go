package common

import "errors"

type Error struct {
	Code int
	Msg  error
}

type HttpError struct {
	Error string `json:"error"`
}

var (
	ErrCollectionNotFound = errors.New("collection not found")
	ErrUserNotFound       = errors.New("user not found")
	ErrExpiredToken       = errors.New("token already expired")
	ErrInvalidToken       = errors.New("token invalid")
	ErrQRRecordNotExist   = errors.New("qr record not exist")
	ErrJwtIncorrect       = errors.New("invalid jwt")
	ErrDeviceNotFound     = errors.New("device not found")
)
