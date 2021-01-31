package fdb

import "github.com/teaelephant/TeaElephantMemory/common"

type qr interface {
	WriteQR(id string, data *common.QR) (err error)
	ReadQR(id string) (record *common.QR, err error)
}
