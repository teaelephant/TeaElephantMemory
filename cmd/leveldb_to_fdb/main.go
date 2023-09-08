package main

import (
	"context"

	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common/key_value/key_builder"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdbclient"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb"
)

const (
	recordNameIndex = 'n'
)

func main() {
	ldb, err := leveldb.NewDB("./database")
	if err != nil {
		panic(err)
	}
	foundeationDB.MustAPIVersion(620)
	db := foundeationDB.MustOpenDefault()
	pairs, err := ldb.ReadAll()
	if err != nil {
		panic(err)
	}
	wrappedDB := fdbclient.NewDatabase(db)
	tr, err := wrappedDB.NewTransaction(context.TODO())
	if err != nil {
		panic(err)
	}
	keyBuilder := key_builder.NewBuilder()
	for _, pair := range pairs {
		switch pair.Key[0] {
		case keyBuilder.Records()[0]:
			idString := string(pair.Key[1:])
			id, err := uuid.FromString(idString)
			if err != nil {
				panic(err)
			}
			if err = tr.Set(keyBuilder.Record(id), pair.Value); err != nil {
				panic(err)
			}
		case recordNameIndex:
			id, err := uuid.FromString(string(pair.Value))
			if err != nil {
				panic(err)
			}
			if err = tr.Set(pair.Key, id.Bytes()); err != nil {
				panic(err)
			}
		default:
			if err = tr.Set(pair.Key, pair.Value); err != nil {
				panic(err)
			}
			continue
		}

	}
	if err = tr.Commit(); err != nil {
		panic(err)
	}
}
