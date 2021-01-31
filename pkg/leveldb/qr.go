package leveldb

import (
	"encoding/json"

	"github.com/teaelephant/TeaElephantMemory/common"
	dbCommon "github.com/teaelephant/TeaElephantMemory/pkg/leveldb/common"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/prefix"
)

type qr interface {
	WriteQR(id string, data *common.QR) (err error)
	ReadQR(id string) (record *common.QR, err error)
}

func (D *levelStorage) WriteQR(id string, qrData *common.QR) error {
	data, err := json.Marshal(qrData)
	if err != nil {
		return err
	}
	return D.db.Put(dbCommon.AppendPrefix(prefix.QR, []byte(id)), data, nil)
}

func (D *levelStorage) ReadQR(id string) (*common.QR, error) {
	data, err := D.db.Get(dbCommon.AppendPrefix(prefix.QR, []byte(id)), nil)
	if err != nil {
		return nil, err
	}
	rec := new(common.QR)
	if err = json.Unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}
