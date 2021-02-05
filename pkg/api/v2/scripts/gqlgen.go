// +build tools

package main

import (
	"github.com/99designs/gqlgen/cmd"
	_ "github.com/99designs/gqlgen/cmd"
)

// dirty huck https://github.com/99designs/gqlgen/issues/800
func main() {
	cmd.Execute()
}
