package main

import (
	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"

	"github.com/teaelephant/TeaElephantMemory/internal/server"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdb"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/migrations"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/migrator"
)

func main() {
	foundeationDB.MustAPIVersion(620)
	db := foundeationDB.MustOpenDefault()
	st := fdb.NewDb(db)
	mig := migrator.NewManager(migrations.Migrations, st)
	if err := mig.Migrate(); err != nil {
		panic(err)
	}
	s := server.NewServer(st)
	if err := s.Run(); err != nil {
		panic(err)
	}
}
