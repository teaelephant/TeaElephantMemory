package fdb

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/key_builder"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdbclient"
)

type DB interface {
	qr
	record
	version
	tag
	collection
	notification

	GetOrCreateUser(ctx context.Context, unique string) (uuid.UUID, error)
	GetUsers(ctx context.Context) ([]common.User, error)
}

type db struct {
	keyBuilder key_builder.Builder
	db         fdbclient.Database

	log *logrus.Entry
}

func (d *db) GetUsers(ctx context.Context) ([]common.User, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := fdb.PrefixRange(d.keyBuilder.Users())
	if err != nil {
		return nil, err
	}

	kvs, err := tr.GetRange(pr)
	if err != nil {
		return nil, err
	}

	users := make([]common.User, 0, len(kvs))

	for _, kv := range kvs {
		id := new(uuid.UUID)
		if err = id.UnmarshalBinary(kv.Key[1:]); err != nil {
			return nil, err
		}

		user := &encoder.User{}

		if err = user.Decode(kv.Value); err != nil {
			// FIXME
			graphql.AddError(ctx, &gqlerror.Error{
				Path:    graphql.GetPath(ctx),
				Message: err.Error(),
				Extensions: map[string]interface{}{
					"code": "-102",
					"user": id.String(),
				},
			})

			continue
		}

		users = append(users, common.User(*user))
	}

	return users, nil
}

func (d *db) GetOrCreateUser(ctx context.Context, unique string) (uuid.UUID, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	key := d.keyBuilder.UserByAppleID(unique)

	data, err := tr.Get(key)
	if err != nil {
		return uuid.Nil, err
	}

	if data != nil {
		return uuid.FromBytes(data)
	}

	user := &common.User{ID: uuid.New(), AppleID: unique}

	data, err = (*encoder.User)(user).Encode()
	if err != nil {
		return uuid.Nil, err
	}

	tr.Set(key, user.ID[:])

	tr.Set(d.keyBuilder.User(user.ID), data)

	if err = tr.Commit(); err != nil {
		return uuid.Nil, err
	}

	return user.ID, nil
}

func NewDB(fdb fdb.Database, log *logrus.Entry) DB {
	return &db{
		keyBuilder: key_builder.NewBuilder(),
		db:         fdbclient.NewDatabase(fdb),
		log:        log,
	}
}
