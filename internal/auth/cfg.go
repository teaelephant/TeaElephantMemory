// Package auth contains configuration and authentication logic for Apple Sign In and JWT.
package auth

import (
	"os"

	"github.com/kelseyhightower/envconfig"
)

// Configuration holds Apple authentication configuration and secret key path.
type Configuration struct {
	SecretPath string
	Secret     string
	TeamID     string
	ClientID   string
	KeyID      string

	// Path to admin public key (mounted as a file)
	AdminPublicKeyPath string
}

// Config loads configuration from environment and reads the private key from SecretPath.
func Config() *Configuration {
	// Load Apple auth configuration from APPLE_AUTH_* variables only
	type appleCfg struct {
		SecretPath string `envconfig:"SECRET_PATH" default:"AuthKey_39D5B439QV.p8"`
		TeamID     string `envconfig:"TEAM_ID" required:"true"`
		ClientID   string `envconfig:"CLIENT_ID" required:"true"`
		KeyID      string `envconfig:"KEY_ID" required:"true"`
	}

	var ac appleCfg
	if err := envconfig.Process("APPLE_AUTH", &ac); err != nil {
		panic(err)
	}

	// Load non-prefixed variables like ADMIN_PUBLIC_KEY_PATH without re-processing Apple fields
	type adminCfg struct {
		AdminPublicKeyPath string `envconfig:"ADMIN_PUBLIC_KEY_PATH" default:"/keys/admin/admin_public_key.pem"`
	}

	var adc adminCfg
	if err := envconfig.Process("", &adc); err != nil {
		panic(err)
	}

	data, err := os.ReadFile(ac.SecretPath)
	if err != nil {
		panic(err)
	}

	return &Configuration{
		SecretPath:         ac.SecretPath,
		Secret:             string(data),
		TeamID:             ac.TeamID,
		ClientID:           ac.ClientID,
		KeyID:              ac.KeyID,
		AdminPublicKeyPath: adc.AdminPublicKeyPath,
	}
}
