package main

import (
	"github.com/teaelephant/TeaElephantMemory/internal/server"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/migrations"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/migrator"
)

func main() {
	st, err := leveldb.NewDB("./database")
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
