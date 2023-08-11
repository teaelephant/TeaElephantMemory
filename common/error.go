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
)
