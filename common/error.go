package common

import "errors"

var (
	ErrCollectionNotFound = errors.New("collection not found")
	ErrUserNotFound       = errors.New("user not found")
	ErrExpiredToken       = errors.New("token already expired")
	ErrInvalidToken       = errors.New("token invalid")
	ErrQRRecordNotExist   = errors.New("qr record not exist")
	ErrJwtIncorrect       = errors.New("invalid jwt")
	ErrDeviceNotFound     = errors.New("device not found")
)
