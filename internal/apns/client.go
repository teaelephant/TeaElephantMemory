package apns

import (
	"errors"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
)

var errInvalidEnvironment = errors.New("APNS_ENVIRONMENT must be sandbox or production")

// NewClient creates a token-authenticated APNs client for the selected environment.
func NewClient(auth *token.Token, environment string) (*apns2.Client, error) {
	switch environment {
	case "sandbox":
		return apns2.NewTokenClient(auth).Development(), nil
	case "production":
		return apns2.NewTokenClient(auth).Production(), nil
	default:
		return nil, errInvalidEnvironment
	}
}
