package fdb

import (
	"context"
	"errors"

	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdbclient"
)

type qr interface {
	WriteQR(ctx context.Context, id uuid.UUID, data *common.QR) (err error)
	ReadQR(ctx context.Context, id uuid.UUID) (record *common.QR, err error)
}

func (d *db) WriteQR(ctx context.Context, id uuid.UUID, data *common.QR) error {
	el, err := (*encoder.QR)(data).Encode()
	if err != nil {
		return err
	}

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	if err := tr.Set(d.keyBuilder.QR(id), el); err != nil {
		return err
	}

	return tr.Commit()
}

func (d *db) ReadQR(ctx context.Context, id uuid.UUID) (record *common.QR, err error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	return d.readQR(id, tr)
}

func (d *db) readQR(id uuid.UUID, tr fdbclient.Transaction) (*common.QR, error) {
	data, err := tr.Get(d.keyBuilder.QR(id))
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, errors.New("data not exist")
	}

	rec := new(encoder.QR)
	if err = rec.Decode(data); err != nil {
		return nil, err
	}

	return (*common.QR)(rec), nil
}
