package fdb

import (
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
)

type qr interface {
	WriteQR(id uuid.UUID, data *common.QR) (err error)
	ReadQR(id uuid.UUID) (record *common.QR, err error)
}

func (d *db) WriteQR(id uuid.UUID, data *common.QR) error {
	el, err := (*encoder.QR)(data).Encode()
	if err != nil {
		return err
	}
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return err
	}
	tr.Set(d.keyBuilder.QR(id), el)
	return tr.Commit().Get()
}

func (d *db) ReadQR(id uuid.UUID) (record *common.QR, err error) {
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return nil, err
	}
	data, err := tr.Get(d.keyBuilder.QR(id)).Get()
	if err != nil {
		return nil, err
	}
	rec := new(encoder.QR)
	if err = rec.Decode(data); err != nil {
		return nil, err
	}
	return (*common.QR)(rec), nil
}
