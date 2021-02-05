package fdb

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
	common2 "github.com/teaelephant/TeaElephantMemory/pkg/fdb/common"
	dbCommon "github.com/teaelephant/TeaElephantMemory/pkg/leveldb/common"
)

type record interface {
	WriteRecord(rec *common.TeaData) (record *common.Tea, err error)
	ReadRecord(id uuid.UUID) (record *common.Tea, err error)
	ReadAllRecords(search string) ([]common.Tea, error)
	Update(id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error)
	Delete(id uuid.UUID) error
	ReadAll() ([]dbCommon.KeyValue, error)
}

func (d *db) ReadAll() ([]dbCommon.KeyValue, error) {
	res := make([]dbCommon.KeyValue, 0)
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return nil, err
	}
	kvs, err := tr.GetRange(fdb.KeyRange{Begin: fdb.Key(""), End: fdb.Key{0xFF}}, fdb.RangeOptions{}).GetSliceWithError()
	if err != nil {
		return nil, err
	}
	for _, kv := range kvs {
		key := make([]byte, len(kv.Key))
		value := make([]byte, len(kv.Value))
		copy(key, kv.Key)
		copy(value, kv.Value)
		res = append(res, dbCommon.KeyValue{
			Key:   key,
			Value: value,
		})
	}
	return res, nil
}

func (d *db) WriteRecord(rec *common.TeaData) (record *common.Tea, err error) {
	return d.writeRecord(uuid.NewV4(), rec)
}

func (d *db) ReadRecord(id uuid.UUID) (*common.Tea, error) {
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return nil, err
	}
	return d.readRecord(id, tr)
}

func (d *db) ReadAllRecords(search string) ([]common.Tea, error) {
	records := make([]common.Tea, 0)
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return nil, err
	}
	if search == "" {
		pr, err := fdb.PrefixRange(d.keyBuilder.Records())
		if err != nil {
			return nil, err
		}
		kvs, err := tr.GetRange(pr, fdb.RangeOptions{}).GetSliceWithError()
		if err != nil {
			return nil, err
		}
		for _, kv := range kvs {
			rec := new(encoder.TeaData)
			if err = rec.Decode(kv.Value); err != nil {
				return nil, err
			}
			records = append(records, common.Tea{
				ID:      uuid.FromBytesOrNil(kv.Key[1:]),
				TeaData: (*common.TeaData)(rec),
			})
		}
		return records, nil
	}
	pr, err := fdb.PrefixRange(d.keyBuilder.RecordsByName(search))
	if err != nil {
		return nil, err
	}
	kvs, err := tr.GetRange(pr, fdb.RangeOptions{}).GetSliceWithError()
	if err != nil {
		return nil, err
	}
	for _, kv := range kvs {
		id := new(uuid.UUID)
		if err = id.UnmarshalBinary(kv.Value); err != nil {
			return nil, err
		}
		rec, err := d.readRecord(*id, tr)
		if err != nil {
			return nil, err
		}
		records = append(records, *rec)
	}
	return records, nil
}

func (d *db) Update(id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error) {
	return d.writeRecord(id, rec)
}

func (d *db) Delete(id uuid.UUID) error {
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return err
	}
	rec, err := d.readRecord(id, tr)
	if err != nil {
		return err
	}
	tr.Clear(d.keyBuilder.Record(id))
	tr.Clear(d.keyBuilder.RecordsByName(rec.Name))
	return tr.Commit().Get()
}

func (d *db) writeRecord(id uuid.UUID, rec *common.TeaData) (*common.Tea, error) {
	data, err := (*encoder.TeaData)(rec).Encode()
	if err != nil {
		return nil, err
	}
	_, err = d.fdb.Transact(func(tr fdb.Transaction) (interface{}, error) {
		tr.Set(d.keyBuilder.Record(id), data)
		tr.Set(d.keyBuilder.RecordsByName(rec.Name), id.Bytes())
		return tr.Get(d.keyBuilder.Record(id)).Get()
		// db.Transact automatically commits (and if necessary,
		// retries) the transaction
	})
	if err != nil {
		return nil, err
	}
	return &common.Tea{
		ID:      id,
		TeaData: rec,
	}, nil
}

func (d *db) readRecord(id uuid.UUID, tr fdb.Transaction) (*common.Tea, error) {
	data, err := tr.Get(d.keyBuilder.Record(id)).Get()
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, common2.ErrNotFound{
			Type: "tea",
			ID:   id.String(),
		}
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
