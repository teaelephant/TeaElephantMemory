// Package common contains shared domain models used across the application.
//
//revive:disable:var-naming // keep package name for compatibility across modules
package common

//revive:enable:var-naming

import "errors"

var (
	// ErrCollectionNotFound indicates a requested collection was not found.
	ErrCollectionNotFound = errors.New("collection not found")
	// ErrUserNotFound indicates an authenticated user was not found in context or storage.
	ErrUserNotFound = errors.New("user not found")
	// ErrExpiredToken indicates the provided JWT has already expired.
	ErrExpiredToken = errors.New("token already expired")
	// ErrInvalidToken indicates the provided JWT is malformed or invalid.
	ErrInvalidToken = errors.New("token invalid")
	// ErrQRRecordNotExist indicates a QR record does not exist.
	ErrQRRecordNotExist = errors.New("qr record not exist")
	// ErrJwtIncorrect indicates an incorrect or missing JWT in request headers.
	ErrJwtIncorrect = errors.New("invalid jwt")
	// ErrDeviceNotFound indicates a device was not found for the given identifier.
	ErrDeviceNotFound = errors.New("device not found")
	// ErrUnauthorized indicates no valid authentication was provided.
	ErrUnauthorized = errors.New("unauthenticated")
	// ErrNotAdmin indicates the authenticated principal lacks admin privileges.
	ErrNotAdmin = errors.New("forbidden: admin required")
)
