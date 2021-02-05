package leveldb

import (
	"fmt"

	"github.com/satori/go.uuid"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/key_builder"
	dbCommon "github.com/teaelephant/TeaElephantMemory/pkg/leveldb/common"
)

type Storage interface {
	qr
	WriteRecord(rec *common.TeaData) (record *common.Tea, err error)
	ReadRecord(id uuid.UUID) (record *common.Tea, err error)
	ReadAllRecords(search string) ([]common.Tea, error)
	Update(id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error)
	Delete(id uuid.UUID) error
	ReadAll() ([]dbCommon.KeyValue, error)
	WriteVersion(version uint32) error
	GetVersion() (uint32, error)
}

type levelStorage struct {
	path       string
	db         *leveldb.DB
	keyBuilder key_builder.Builder
}

func (l *levelStorage) Delete(id uuid.UUID) error {
	rec, err := l.ReadRecord(id)
	if err != nil {
		return err
	}
	if err = l.db.Delete(l.keyBuilder.Record(id), nil); err != nil {
		return err
	}
	if err = l.db.Delete(l.keyBuilder.RecordsByName(rec.Name), nil); err != nil {
		return err
	}
	return nil
}

func (l *levelStorage) Update(id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error) {
	return l.writeRecord(id, rec)
}

func (l *levelStorage) ReadAllRecords(search string) ([]common.Tea, error) {
	records := make([]common.Tea, 0)
	if search == "" {
		iter := l.db.NewIterator(util.BytesPrefix(l.keyBuilder.Records()), nil)
		for iter.Next() {
			rec := new(encoder.TeaData)
			if err := rec.Decode(iter.Value()); err != nil {
				return nil, err
			}
			records = append(records, common.Tea{
				ID:      uuid.FromStringOrNil(string(iter.Key()[1:])),
				TeaData: (*common.TeaData)(rec),
			})
		}
		if iter.Error() != nil {
			return nil, iter.Error()
		}
		iter.Release()
		return records, nil
	}
	iter := l.db.NewIterator(util.BytesPrefix(l.keyBuilder.RecordsByName(search)), nil)
	defer iter.Release()
	for iter.Next() {
		id := new(uuid.UUID)
		if err := id.UnmarshalText(iter.Value()); err != nil {
			return nil, err
		}
		rec, err := l.ReadRecord(*id)
		if err != nil {
			return nil, err
		}
		records = append(records, *rec)
	}
	if iter.Error() != nil {
		return nil, iter.Error()
	}
	return records, nil
}

func (l *levelStorage) WriteRecord(rec *common.TeaData) (record *common.Tea, err error) {
	id := uuid.NewV4()
	return l.writeRecord(id, rec)
}

func (l *levelStorage) ReadRecord(id uuid.UUID) (record *common.Tea, err error) {
	data, err := l.db.Get(l.keyBuilder.Record(id), nil)
	if err != nil {
		return nil, err
	}
	rec := new(encoder.TeaData)
	if err = rec.Decode(data); err != nil {
		return nil, err
	}
	return &common.Tea{
		ID:      id,
		TeaData: (*common.TeaData)(rec),
	}, nil
}

func (l *levelStorage) writeRecord(id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error) {
	data, err := (*encoder.TeaData)(rec).Encode()
	if err != nil {
		return nil, err
	}
	if err := l.db.Put(l.keyBuilder.Record(id), data, nil); err != nil {
		return nil, err
	}
	if err := l.db.Put(l.keyBuilder.RecordsByName(rec.Name), id.Bytes(), nil); err != nil {
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
	return &levelStorage{
		path:       path,
		db:         db,
		keyBuilder: key_builder.NewBuilder(),
	}, nil
}
