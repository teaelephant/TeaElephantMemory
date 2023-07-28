package main

import (
	"github.com/kelseyhightower/envconfig"

	"github.com/teaelephant/TeaElephantMemory/printqr"
)

type configuration struct {
	UnidocLicenseAPIKey string `required:"true"`
}

func main() {
	cfg := new(configuration)
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}

	gen := printqr.NewGenerator(cfg.UnidocLicenseAPIKey)

	if err := gen.GenerateAndSave(10); err != nil {
		panic(err)
	}
}
