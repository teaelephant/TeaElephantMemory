package openweather

import (
	"github.com/kelseyhightower/envconfig"
)

type Configuration struct {
	ApiKey string `require:"true"`
}

func Config() *Configuration {
	cfg := new(Configuration)
	if err := envconfig.Process("OPENWEATHER", cfg); err != nil {
		panic(err)
	}

	return cfg
}
