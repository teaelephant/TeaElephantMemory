package fdb

import (
	"context"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdbclient"
)

type collection interface {
	CreateCollection(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error)
	AddTeaToCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error
	DeleteTeaFromCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error
	DeleteCollection(ctx context.Context, id, userID uuid.UUID) error
	Collections(ctx context.Context, userID uuid.UUID) ([]*common.Collection, error)
	Collection(ctx context.Context, id, userID uuid.UUID) (*common.Collection, error)
	CollectionTeas(ctx context.Context, id uuid.UUID) ([]*common.Tea, error)
}

func (d *db) CreateCollection(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error) {
	data, err := (&encoder.Collection{
		Name:   name,
		UserID: userID,
	}).Encode()
	if err != nil {
		return uuid.UUID{}, err
	}

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return uuid.UUID{}, err
	}

	id := uuid.NewV4()

	if err = tr.Set(d.keyBuilder.Collection(id, userID), data); err != nil {
		return uuid.UUID{}, err
	}

	if err = tr.Commit(); err != nil {
		return uuid.UUID{}, err
	}

	return id, nil
}

func (d *db) AddTeaToCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	for _, tea := range teas {
		if err = tr.Set(d.keyBuilder.CollectionsTeas(id, tea), tea.Bytes()); err != nil {
			return err
		}
	}

	return tr.Commit()
}

func (d *db) DeleteTeaFromCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	for _, tea := range teas {
		tr.Clear(d.keyBuilder.CollectionsTeas(id, tea))
	}

	return tr.Commit()
}

func (d *db) DeleteCollection(ctx context.Context, id, userID uuid.UUID) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	tr.Clear(d.keyBuilder.Collection(id, userID))

	pr, err := fdb.PrefixRange(d.keyBuilder.TeaByCollection(id))
	if err != nil {
		return err
	}

	kvs, err := tr.GetRange(pr)
	if err != nil {
		return err
	}

	for _, kv := range kvs {
		teaID := new(uuid.UUID)
		if err = id.UnmarshalBinary(kv.Value); err != nil {
			return err
		}

		tr.Clear(d.keyBuilder.CollectionsTeas(id, *teaID))
	}

	if err = tr.Commit(); err != nil {
		return err
	}

	return nil
}

func (d *db) Collections(ctx context.Context, userID uuid.UUID) ([]*common.Collection, error) {
	records := make([]*common.Collection, 0)

	pr, err := fdb.PrefixRange(d.keyBuilder.UserCollections(userID))
	if err != nil {
		return nil, err
	}

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	kvs, err := tr.GetRange(pr)
	if err != nil {
		return nil, err
	}

	for _, kv := range kvs {
		id := new(uuid.UUID)
		if err = id.UnmarshalBinary(kv.Key[uuid.Size+1:]); err != nil {
			return nil, err
		}

		col := new(encoder.Collection)
		if err = col.Decode(kv.Value); err != nil {
			return nil, err
		}

		records = append(records, &common.Collection{
			ID:   *id,
			Name: col.Name,
		})
	}

	return records, nil
}

func (d *db) Collection(ctx context.Context, id, userID uuid.UUID) (*common.Collection, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	return d.readCollection(tr, id, userID)
}

func (d *db) CollectionTeas(ctx context.Context, id uuid.UUID) ([]*common.Tea, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	return d.collectionsTeas(tr, id)
}

func (d *db) collectionsTeas(tr fdbclient.Transaction, id uuid.UUID) ([]*common.Tea, error) {
	records := make([]*common.Tea, 0)

	pr, err := fdb.PrefixRange(d.keyBuilder.TeaByCollection(id))
	if err != nil {
		return nil, err
	}

	kvs, err := tr.GetRange(pr)
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

		records = append(records, rec)
	}

	return records, nil
}

func (d *db) readCollection(tr fdbclient.Transaction, id uuid.UUID, userID uuid.UUID) (*common.Collection, error) {
	data, err := tr.Get(d.keyBuilder.Collection(id, userID))
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, common.ErrCollectionNotFound
	}

	col := new(encoder.Collection)
	if err = col.Decode(data); err != nil {
		return nil, err
	}

	return &common.Collection{
		ID:   id,
		Name: col.Name,
	}, nil
}
