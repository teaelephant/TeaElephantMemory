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
	ErrUnknownSession     = errors.New("unknown session")
	ErrExpiredToken       = errors.New("token already expired")
	ErrInvalidToken       = errors.New("token invalid")
)
