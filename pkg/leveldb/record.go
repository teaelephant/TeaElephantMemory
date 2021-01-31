package leveldb

import (
	"encoding/json"
	"fmt"

	"github.com/satori/go.uuid"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/teaelephant/TeaElephantMemory/common"
	dbCommon "github.com/teaelephant/TeaElephantMemory/pkg/leveldb/common"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/prefix"
)

type Storage interface {
	qr
	WriteRecord(rec *common.TeaData) (record *common.Tea, err error)
	ReadRecord(id string) (record *common.Tea, err error)
	ReadAllRecords(search string) ([]common.Tea, error)
	Update(id string, rec *common.TeaData) (record *common.Tea, err error)
	Delete(id string) error
	ReadAll() ([]dbCommon.KeyValue, error)
	WriteVersion(version uint32) error
	GetVersion() (uint32, error)
}

type levelStorage struct {
	path string
	db   *leveldb.DB
}

func (D *levelStorage) Delete(id string) error {
	rec, err := D.ReadRecord(id)
	if err != nil {
		return err
	}
	if err := D.db.Delete(dbCommon.AppendPrefix(prefix.Record, []byte(id)), nil); err != nil {
		return err
	}
	if err := D.db.Delete(dbCommon.AppendPrefix(prefix.RecordNameIndex, []byte(rec.Name)), nil); err != nil {
		return err
	}
	return nil
}

func (D *levelStorage) Update(id string, rec *common.TeaData) (record *common.Tea, err error) {
	return D.writeRecord(id, rec)
}

func (D *levelStorage) ReadAllRecords(search string) ([]common.Tea, error) {
	records := make([]common.Tea, 0)
	if search == "" {
		iter := D.db.NewIterator(util.BytesPrefix([]byte{prefix.Record}), nil)
		for iter.Next() {
			rec := new(common.TeaData)
			if err := json.Unmarshal(iter.Value(), rec); err != nil {
				return nil, err
			}
			records = append(records, common.Tea{
				ID:      string(iter.Key()[1:]),
				TeaData: rec,
			})
		}
		if iter.Error() != nil {
			return nil, iter.Error()
		}
		iter.Release()
		return records, nil
	}
	iter := D.db.NewIterator(util.BytesPrefix(dbCommon.AppendPrefix(prefix.RecordNameIndex, []byte(search))), nil)
	for iter.Next() {
		rec, err := D.ReadRecord(string(iter.Value()))
		if err != nil {
			return nil, err
		}
		records = append(records, *rec)
	}
	if iter.Error() != nil {
		return nil, iter.Error()
	}
	iter.Release()
	return records, nil
}

func (D *levelStorage) WriteRecord(rec *common.TeaData) (record *common.Tea, err error) {
	id := uuid.NewV4().String()
	return D.writeRecord(id, rec)
}

func (D *levelStorage) ReadRecord(id string) (record *common.Tea, err error) {
	data, err := D.db.Get(dbCommon.AppendPrefix(prefix.Record, []byte(id)), nil)
	if err != nil {
		return nil, err
	}
	rec := new(common.TeaData)
	if err := json.Unmarshal(data, rec); err != nil {
		return nil, err
	}
	return &common.Tea{
		ID:      id,
		TeaData: rec,
	}, nil
}

func (D *levelStorage) writeRecord(id string, rec *common.TeaData) (record *common.Tea, err error) {
	data, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	if err := D.db.Put(dbCommon.AppendPrefix(prefix.Record, []byte(id)), data, nil); err != nil {
		return nil, err
	}
	if err := D.db.Put(dbCommon.AppendPrefix(prefix.RecordNameIndex, []byte(rec.Name)), []byte(id), nil); err != nil {
		return nil, err
	}
	return &common.Tea{
		ID:      id,
		TeaData: rec,
	}, nil
}

func NewDB(path string) (Storage, error) {
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
	return &levelStorage{path: path, db: db}, nil
}
