package main

import (
	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common/key_value/key_builder"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdb"
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
	tr, err := db.CreateTransaction()
	if err != nil {
		panic(err)
	}
	keyBuilder := fdb.NewKeyBuilder(key_builder.NewBuilder())
	for _, pair := range pairs {
		switch pair.Key[0] {
		case keyBuilder.Records()[0]:
			idString := string(pair.Key[1:])
			id, err := uuid.FromString(idString)
			if err != nil {
				panic(err)
			}
			tr.Set(keyBuilder.Record(id), pair.Value)
		case recordNameIndex:
			id, err := uuid.FromString(string(pair.Value))
			if err != nil {
				panic(err)
			}
			tr.Set(foundeationDB.Key(pair.Key), id.Bytes())
		default:
			tr.Set(foundeationDB.Key(pair.Key), pair.Value)
			continue
		}

	}
	if err = tr.Commit().Get(); err != nil {
		panic(err)
	}
}
