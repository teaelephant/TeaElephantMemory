package db

import (
	"encoding/json"
	"fmt"
	"github.com/lueurxax/teaelephantmemory/common"
	"github.com/satori/go.uuid"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

type DB struct {
	path string
	db   *leveldb.DB
}

func (D *DB) Update(id string, rec *common.Record) (record *common.RecordWithID, err error) {
	data, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	if err := D.db.Put([]byte(id), data, nil); err != nil {
		return nil, err
	}
	return &common.RecordWithID{
		ID:     id,
		Record: rec,
	}, nil
}

func (D *DB) ReadAll() ([]common.RecordWithID, error) {
	var records []common.RecordWithID
	iter := D.db.NewIterator(nil, nil)
	for iter.Next() {
		rec := new(common.Record)
		if err := json.Unmarshal(iter.Value(), rec); err != nil {
			return nil, err
		}
		records = append(records, common.RecordWithID{
			ID:     string(iter.Key()),
			Record: rec,
		})
	}
	if iter.Error() != nil {
		return nil, iter.Error()
	}
	iter.Release()
	return records, nil
}

func (D *DB) Write(rec *common.Record) (record *common.RecordWithID, err error) {
	id := uuid.NewV4().String()
	data, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	if err := D.db.Put([]byte(id), data, nil); err != nil {
		return nil, err
	}
	return &common.RecordWithID{
		ID:     id,
		Record: rec,
	}, nil
}

func (D *DB) Read(id string) (record *common.RecordWithID, err error) {
	data, err := D.db.Get([]byte(id), nil)
	if err != nil {
		return nil, err
	}
	rec := new(common.Record)
	if err := json.Unmarshal(data, rec); err != nil {
		return nil, err
	}
	return &common.RecordWithID{
		ID:     id,
		Record: rec,
	}, nil
}

func NewDB(path string) (*DB, error) {
	opts := &opt.Options{
		OpenFilesCacheCapacity: 16,
		BlockCacheCapacity:     16 / 2 * opt.MiB,
		WriteBuffer:            16 / 4 * opt.MiB, // Two of these are used internally
		Filter:                 filter.NewBloomFilter(10),
	}
	db, err := leveldb.OpenFile(path, opts)
	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(path, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("path: %s, %s", path, err.Error())
	}
	return &DB{path: path, db: db}, nil
}
