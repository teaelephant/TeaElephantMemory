package fdb

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"

	"github.com/teaelephant/TeaElephantMemory/common/key_value/key_builder"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdbclient"
)

type DB interface {
	qr
	record
	version
	tag
}

type db struct {
	keyBuilder key_builder.Builder
	db         fdbclient.Database
}

func NewDb(fdb fdb.Database) DB {
	return &db{
		keyBuilder: key_builder.NewBuilder(),
		db:         fdbclient.NewDatabase(fdb),
	}
}
