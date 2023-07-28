package main

import (
	"github.com/kelseyhightower/envconfig"

	"github.com/teaelephant/TeaElephantMemory/printqr"
)

type configuration struct {
	UnidocLicenseApiKey string `required:"true"`
}

func main() {
	cfg := new(configuration)
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}
	gen := printqr.NewGenerator(cfg.UnidocLicenseApiKey)

	if err := gen.GenerateAndSave(10); err != nil {
		panic(err)
	}
}
