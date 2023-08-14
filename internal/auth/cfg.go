package auth

import (
	"os"

	"github.com/kelseyhightower/envconfig"
)

type Configuration struct {
	SecretPath string `envconfig:"SECRET_PATH" default:"AuthKey_39D5B439QV.p8"`
	Secret     string
	TeamID     string `envconfig:"TEAM_ID" required:"true"`
	ClientID   string `envconfig:"CLIENT_ID" required:"true"`
	KeyID      string `envconfig:"KEY_ID" required:"true"`
}

func Config() *Configuration {
	cfg := new(Configuration)
	if err := envconfig.Process("APPLE_AUTH", cfg); err != nil {
		panic(err)
	}

	data, err := os.ReadFile(cfg.SecretPath)
	if err != nil {
		panic(err)
	}

	cfg.Secret = string(data)

	return cfg
}
