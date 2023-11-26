package fdb

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/google/uuid"
	"github.com/vektah/gqlparser/v2/gqlerror"

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
	CollectionRecords(ctx context.Context, id uuid.UUID) ([]*common.CollectionRecord, error)
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

	id := uuid.New()

	tr.Set(d.keyBuilder.Collection(id, userID), data)

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
		data, err := tr.Get(d.keyBuilder.QR(tea))
		if err != nil {
			return err
		}

		if data == nil {
			continue
		}

		tr.Set(d.keyBuilder.CollectionsTeas(id, tea), tea[:])
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

	pr, err := fdb.PrefixRange(d.keyBuilder.RecordsByCollection(id))
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

	return tr.Commit()
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
		if err = id.UnmarshalBinary(kv.Key[len(id)+1:]); err != nil {
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

func (d *db) CollectionRecords(ctx context.Context, id uuid.UUID) ([]*common.CollectionRecord, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	return d.collectionRecords(ctx, tr, id)
}

func (d *db) collectionRecords(ctx context.Context, tr fdbclient.Transaction, id uuid.UUID) ([]*common.CollectionRecord, error) {
	records := make([]*common.CollectionRecord, 0)

	pr, err := fdb.PrefixRange(d.keyBuilder.RecordsByCollection(id))
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

		rec, err := d.readQR(*id, tr)
		if err != nil {
			// FIXME
			graphql.AddError(ctx, &gqlerror.Error{
				Path:    graphql.GetPath(ctx),
				Message: err.Error(),
				Extensions: map[string]interface{}{
					"code":   "-101",
					"record": id.String(),
				},
			})

			continue
		}

		tea, err := d.readRecord(rec.Tea, tr)
		if err != nil {
			// FIXME
			graphql.AddError(ctx, &gqlerror.Error{
				Path:    graphql.GetPath(ctx),
				Message: err.Error(),
				Extensions: map[string]interface{}{
					"code":   "-100",
					"record": id.String(),
				},
			})

			continue
		}

		records = append(records, &common.CollectionRecord{
			ID:             *id,
			Tea:            tea,
			BowlingTemp:    rec.BowlingTemp,
			ExpirationDate: rec.ExpirationDate,
		})
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
