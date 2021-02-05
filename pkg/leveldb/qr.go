package leveldb

import (
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
)

type qr interface {
	WriteQR(id uuid.UUID, data *common.QR) (err error)
	ReadQR(id uuid.UUID) (record *common.QR, err error)
}

func (l *levelStorage) WriteQR(id uuid.UUID, qrData *common.QR) error {
	data, err := (*encoder.QR)(qrData).Encode()
	if err != nil {
		return err
	}
	return l.db.Put(l.keyBuilder.QR(id), data, nil)
}

func (l *levelStorage) ReadQR(id uuid.UUID) (*common.QR, error) {
	data, err := l.db.Get(l.keyBuilder.QR(id), nil)
	if err != nil {
		return nil, err
	}
	rec := new(encoder.QR)
	if err = rec.Decode(data); err != nil {
		return nil, err
	}
	return (*common.QR)(rec), nil
}
