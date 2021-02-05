package qr_manager

import (
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

type Manager interface {
	Set(id uuid.UUID, data *model.QRRecordData) (err error)
	Get(id uuid.UUID) (*model.QRRecordData, error)
}

type storage interface {
	WriteQR(id uuid.UUID, data *common.QR) (err error)
	ReadQR(id uuid.UUID) (record *common.QR, err error)
}

type manager struct {
	storage
}

func (m *manager) Set(id uuid.UUID, data *model.QRRecordData) (err error) {
	return m.storage.WriteQR(id, &common.QR{
		Tea:            uuid.UUID(data.Tea),
		BowlingTemp:    data.BowlingTemp,
		ExpirationDate: data.ExpirationDate,
	})
}

func (m *manager) Get(id uuid.UUID) (*model.QRRecordData, error) {
	data, err := m.storage.ReadQR(id)
	if err != nil {
		return nil, err
	}
	return &model.QRRecordData{
		Tea:            gqlCommon.ID(data.Tea),
		BowlingTemp:    data.BowlingTemp,
		ExpirationDate: data.ExpirationDate,
	}, nil
}

func NewManager(storage storage) Manager {
	return &manager{storage: storage}
}
