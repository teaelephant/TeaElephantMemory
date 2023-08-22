package fdb

import (
	"context"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"

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

	GetOrCreateUser(ctx context.Context, unique string) (uuid.UUID, error)
}

type db struct {
	keyBuilder key_builder.Builder
	db         fdbclient.Database

	log *logrus.Entry
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

	if data == nil {
		user := &common.User{ID: uuid.NewV4(), AppleID: unique}

		data, err = (*encoder.User)(user).Encode()
		if err != nil {
			return uuid.Nil, err
		}

		if err = tr.Set(key, user.ID.Bytes()); err != nil {
			return uuid.Nil, err
		}

		if err = tr.Set(d.keyBuilder.User(user.ID), data); err != nil {
			return uuid.Nil, err
		}

		if err = tr.Commit(); err != nil {
			return uuid.Nil, err
		}

		return user.ID, nil
	}

	return uuid.FromBytes(data)
}

func NewDB(fdb fdb.Database, log *logrus.Entry) DB {
	return &db{
		keyBuilder: key_builder.NewBuilder(),
		db:         fdbclient.NewDatabase(fdb),
		log:        log,
	}
}
