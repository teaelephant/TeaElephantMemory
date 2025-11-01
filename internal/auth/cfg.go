// Package auth contains configuration and authentication logic for Apple Sign In and JWT.
package auth

import (
	"os"

	"github.com/kelseyhightower/envconfig"
)

// Configuration holds Apple authentication configuration and secret key path.
type Configuration struct {
	SecretPath string `envconfig:"SECRET_PATH" default:"AuthKey_39D5B439QV.p8"`
	Secret     string
	TeamID     string `envconfig:"TEAM_ID" required:"true"`
	ClientID   string `envconfig:"CLIENT_ID" required:"true"`
	KeyID      string `envconfig:"KEY_ID" required:"true"`

	// Path to admin public key (mounted as a file)
	AdminPublicKeyPath string `envconfig:"ADMIN_PUBLIC_KEY_PATH" default:"/keys/admin/admin_public_key.pem"`
}

// Config loads configuration from environment and reads the private key from SecretPath.
func Config() *Configuration {
	cfg := new(Configuration)
	// Load Apple auth configuration from APPLE_AUTH_* variables
	if err := envconfig.Process("APPLE_AUTH", cfg); err != nil {
		panic(err)
	}
	// Load non-prefixed variables like ADMIN_PUBLIC_KEY_PATH
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}

	data, err := os.ReadFile(cfg.SecretPath)
	if err != nil {
		panic(err)
	}

	cfg.Secret = string(data)

	return cfg
}
