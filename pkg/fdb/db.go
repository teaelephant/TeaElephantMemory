package fdb

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"

	"github.com/teaelephant/TeaElephantMemory/common/key_value/key_builder"
)

type DB interface {
	qr
	record
	version
	tag
}

type db struct {
	fdb        fdb.Database
	keyBuilder KeyBuilder
}

func NewDb(fdb fdb.Database) DB {
	return &db{
		fdb:        fdb,
		keyBuilder: NewKeyBuilder(key_builder.NewBuilder()),
	}
}
