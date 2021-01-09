package main

import (
	"github.com/lueurxax/teaelephantmemory/internal/server"
	"github.com/lueurxax/teaelephantmemory/pkg/db"
	"github.com/lueurxax/teaelephantmemory/pkg/db/migrations"
	"github.com/lueurxax/teaelephantmemory/pkg/db/migrator"
)

func main() {
	st, err := db.NewDB("./database")
	if err != nil {
		panic(err)
	}
	mig := migrator.NewManager(migrations.Migrations, st)
	if err = mig.Migrate(); err != nil {
		panic(err)
	}
	s := server.NewServer(st)
	if err := s.Run(); err != nil {
		panic(err)
	}
}
