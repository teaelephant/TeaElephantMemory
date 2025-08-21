package fdb

import (
	"context"
	"fmt"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
	common2 "github.com/teaelephant/TeaElephantMemory/pkg/fdb/common"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdbclient"
)

const iteratorGetFmt = "iterator get: %w"

type record interface {
	WriteRecord(ctx context.Context, rec *common.TeaData) (record *common.Tea, err error)
	ReadRecord(ctx context.Context, id uuid.UUID) (record *common.Tea, err error)
	ReadAllRecords(ctx context.Context, search string) ([]common.Tea, error)
	Update(ctx context.Context, id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error)
	Delete(ctx context.Context, id uuid.UUID) error
}

func (d *db) WriteRecord(ctx context.Context, rec *common.TeaData) (record *common.Tea, err error) {
	return d.writeRecord(ctx, uuid.New(), rec)
}

func (d *db) ReadRecord(ctx context.Context, id uuid.UUID) (*common.Tea, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	return d.readRecord(id, tr)
}

func (d *db) ReadAllRecords(ctx context.Context, search string) ([]common.Tea, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	if search == "" {
		return d.readAllWithoutSearch(tr)
	}

	return d.readAllByPrefix(tr, search)
}

func (d *db) readAllWithoutSearch(tr fdbclient.Transaction) ([]common.Tea, error) {
	records := make([]common.Tea, 0)

	pr, err := fdb.PrefixRange(d.keyBuilder.Records())
	if err != nil {
		return nil, fmt.Errorf("records prefix range: %w", err)
	}

	it := tr.GetIterator(pr)
	for it.Advance() {
		kv, err := it.Get()
		if err != nil {
			return nil, fmt.Errorf(iteratorGetFmt, err)
		}

		rec := new(encoder.TeaData)
		if err = rec.Decode(kv.Value); err != nil {
			return nil, fmt.Errorf("decode tea data: %w", err)
		}

		id, err := uuid.FromBytes(kv.Key[1:])
		if err != nil {
			id = uuid.Nil
		}

		records = append(records, common.Tea{ID: id, TeaData: rec.ToCommonTeaData()})
	}

	return records, nil
}

func (d *db) readAllByPrefix(tr fdbclient.Transaction, search string) ([]common.Tea, error) {
	prefix := d.keyBuilder.RecordsByName(search)
	d.log.WithField("prefix", string(prefix)).Debug("search by prefix")
	pr, err := fdb.PrefixRange(prefix)
	if err != nil {
		return nil, err
	}
	it := tr.GetIterator(pr)
	records := make([]common.Tea, 0)
	for it.Advance() {
		kv, err := it.Get()
		if err != nil {
			return nil, fmt.Errorf(iteratorGetFmt, err)
		}
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

func (d *db) Update(ctx context.Context, id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error) {
	return d.writeRecord(ctx, id, rec)
}

func (d *db) Delete(ctx context.Context, id uuid.UUID) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	rec, err := d.readRecord(id, tr)
	if err != nil {
		return err
	}

	tr.Clear(d.keyBuilder.Record(id))
	tr.Clear(d.keyBuilder.RecordsByName(rec.Name))

	return tr.Commit()
}

func (d *db) writeRecord(ctx context.Context, id uuid.UUID, rec *common.TeaData) (*common.Tea, error) {
	data, err := encoder.FromCommonTeaData(rec).Encode()
	if err != nil {
		return nil, err
	}

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	tr.Set(d.keyBuilder.Record(id), data)

	tr.Set(d.keyBuilder.RecordsByName(rec.Name), id[:])

	if err = tr.Commit(); err != nil {
		return nil, err
	}

	return &common.Tea{
		ID:      id,
		TeaData: rec,
	}, nil
}

func (d *db) readRecord(id uuid.UUID, tr fdbclient.Transaction) (*common.Tea, error) {
	data, err := tr.Get(d.keyBuilder.Record(id))
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
		TeaData: rec.ToCommonTeaData(),
	}, nil
}
