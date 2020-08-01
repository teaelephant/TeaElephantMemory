package main

import "github.com/lueurxax/teaelephantmemory/pkg/server"

func main() {
	s := server.NewServer()
	if err := s.Run(); err != nil {
		panic(err)
	}
}
