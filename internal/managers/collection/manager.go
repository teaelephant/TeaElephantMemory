package collection

import (
	"context"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

type Manager interface {
	Create(ctx context.Context, userID uuid.UUID, name string) (*model.Collection, error)
	AddRecords(ctx context.Context, userID uuid.UUID, id uuid.UUID, teas []uuid.UUID) (*model.Collection, error)
	DeleteRecords(ctx context.Context, userID uuid.UUID, id uuid.UUID, teas []uuid.UUID) (*model.Collection, error)
	Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	List(ctx context.Context, userID uuid.UUID) ([]*model.Collection, error)
	ListRecords(ctx context.Context, id, userID uuid.UUID) ([]*model.QRRecord, error)
}

type storage interface {
	CreateCollection(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error)
	AddTeaToCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error
	DeleteTeaFromCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error
	DeleteCollection(ctx context.Context, id, userID uuid.UUID) error
	Collections(ctx context.Context, userID uuid.UUID) ([]*common.Collection, error)
	Collection(ctx context.Context, id, userID uuid.UUID) (*common.Collection, error)
	CollectionRecords(ctx context.Context, id uuid.UUID) ([]*common.CollectionRecord, error)
}

type manager struct {
	storage
}

func (m *manager) ListRecords(ctx context.Context, id, userID uuid.UUID) ([]*model.QRRecord, error) {
	if _, err := m.Collection(ctx, id, userID); err != nil {
		return nil, err
	}

	records, err := m.CollectionRecords(ctx, id)
	if err != nil {
		return nil, err
	}

	list := make([]*model.QRRecord, len(records))
	for i, record := range records {
		list[i] = &model.QRRecord{
			ID:             gqlCommon.ID(record.ID),
			Tea:            model.FromCommonTea(record.Tea),
			BowlingTemp:    record.BowlingTemp,
			ExpirationDate: record.ExpirationDate,
		}
	}

	return list, err
}

func (m *manager) Create(ctx context.Context, userID uuid.UUID, name string) (*model.Collection, error) {
	id, err := m.CreateCollection(ctx, userID, name)
	if err != nil {
		return nil, err
	}

	return &model.Collection{
		ID:     gqlCommon.ID(id),
		Name:   name,
		UserID: gqlCommon.ID(userID),
	}, nil
}

func (m *manager) AddRecords(ctx context.Context, userID, id uuid.UUID, teas []uuid.UUID) (*model.Collection, error) {
	if _, err := m.Collection(ctx, id, userID); err != nil {
		return nil, err
	}

	if err := m.AddTeaToCollection(ctx, id, teas); err != nil {
		return nil, err
	}

	collection, err := m.Collection(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	return &model.Collection{
		ID:     gqlCommon.ID(id),
		Name:   collection.Name,
		UserID: gqlCommon.ID(userID),
	}, nil
}

func (m *manager) DeleteRecords(ctx context.Context, userID uuid.UUID, id uuid.UUID, teas []uuid.UUID) (*model.Collection, error) {
	if _, err := m.Collection(ctx, id, userID); err != nil {
		return nil, err
	}

	if err := m.DeleteTeaFromCollection(ctx, id, teas); err != nil {
		return nil, err
	}

	collection, err := m.Collection(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	return &model.Collection{
		ID:     gqlCommon.ID(id),
		Name:   collection.Name,
		UserID: gqlCommon.ID(userID),
	}, nil
}

func (m *manager) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	return m.DeleteCollection(ctx, id, userID)
}

func (m *manager) List(ctx context.Context, userID uuid.UUID) ([]*model.Collection, error) {
	list, err := m.Collections(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]*model.Collection, len(list))
	for i, col := range list {
		result[i] = &model.Collection{
			ID:     gqlCommon.ID(col.ID),
			Name:   col.Name,
			UserID: gqlCommon.ID(userID),
		}
	}

	return result, nil
}

func NewManager(storage storage) Manager {
	return &manager{storage: storage}
}
