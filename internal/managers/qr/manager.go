package qr

import (
	"context"

	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

type Manager interface {
	Set(ctx context.Context, id uuid.UUID, data *model.QRRecordData) (err error)
	Get(ctx context.Context, id uuid.UUID) (*model.QRRecordData, error)
}

type storage interface {
	WriteQR(ctx context.Context, id uuid.UUID, data *common.QR) (err error)
	ReadQR(ctx context.Context, id uuid.UUID) (record *common.QR, err error)
}

type manager struct {
	storage
}

func (m *manager) Set(ctx context.Context, id uuid.UUID, data *model.QRRecordData) (err error) {
	return m.storage.WriteQR(ctx, id, &common.QR{
		Tea:            uuid.UUID(data.Tea),
		BowlingTemp:    data.BowlingTemp,
		ExpirationDate: data.ExpirationDate,
	})
}

func (m *manager) Get(ctx context.Context, id uuid.UUID) (*model.QRRecordData, error) {
	data, err := m.storage.ReadQR(ctx, id)
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
