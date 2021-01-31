package qr_manager

import (
	"github.com/teaelephant/TeaElephantMemory/common"
	model "github.com/teaelephant/TeaElephantMemory/internal/server/api/v2/models"
)

type Manager interface {
	Set(id string, data *model.QRRecordData) (err error)
	Get(id string) (*model.QRRecordData, error)
}

type storage interface {
	WriteQR(id string, data *common.QR) (err error)
	ReadQR(id string) (record *common.QR, err error)
}

type manager struct {
	storage
}

func (m *manager) Set(id string, data *model.QRRecordData) (err error) {
	return m.storage.WriteQR(id, &common.QR{
		Tea:            data.Tea,
		BowlingTemp:    data.BowlingTemp,
		ExpirationDate: data.ExpirationDate,
	})
}

func (m *manager) Get(id string) (*model.QRRecordData, error) {
	data, err := m.storage.ReadQR(id)
	if err != nil {
		return nil, err
	}
	return &model.QRRecordData{
		Tea:            data.Tea,
		BowlingTemp:    data.BowlingTemp,
		ExpirationDate: data.ExpirationDate,
	}, nil
}

func NewManager(storage storage) Manager {
	return &manager{storage: storage}
}
