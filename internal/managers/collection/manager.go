package collection

import (
	"context"

	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

type Manager interface {
	Create(ctx context.Context, userID uuid.UUID, name string) (*model.Collection, error)
	AddTea(ctx context.Context, userID uuid.UUID, id uuid.UUID, teas []uuid.UUID) (*model.Collection, error)
	DeleteTea(ctx context.Context, userID uuid.UUID, id uuid.UUID, teas []uuid.UUID) (*model.Collection, error)
	Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	List(ctx context.Context, userID uuid.UUID) ([]*model.Collection, error)
	ListTeas(ctx context.Context, id, userID uuid.UUID) ([]*model.Tea, error)
}

type storage interface {
	CreateCollection(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error)
	AddTeaToCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error
	DeleteTeaFromCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error
	DeleteCollection(ctx context.Context, id, userID uuid.UUID) error
	Collections(ctx context.Context, userID uuid.UUID) ([]*common.Collection, error)
	Collection(ctx context.Context, id, userID uuid.UUID) (*common.Collection, error)
	CollectionTeas(ctx context.Context, id uuid.UUID) ([]*common.Tea, error)
}

type manager struct {
	storage
}

func (m *manager) ListTeas(ctx context.Context, id, userID uuid.UUID) ([]*model.Tea, error) {
	if _, err := m.storage.Collection(ctx, id, userID); err != nil {
		return nil, err
	}

	teas, err := m.storage.CollectionTeas(ctx, id)
	if err != nil {
		return nil, err
	}

	list := make([]*model.Tea, len(teas))
	for i, tea := range teas {
		list[i] = &model.Tea{
			ID:          gqlCommon.ID(tea.ID),
			Name:        tea.Name,
			Type:        model.Type(tea.Type),
			Description: tea.Description,
		}
	}

	return list, err
}

func (m *manager) Create(ctx context.Context, userID uuid.UUID, name string) (*model.Collection, error) {
	id, err := m.storage.CreateCollection(ctx, userID, name)
	if err != nil {
		return nil, err
	}

	return &model.Collection{
		ID:     gqlCommon.ID(id),
		Name:   name,
		UserID: gqlCommon.ID(userID),
	}, nil
}

func (m *manager) AddTea(ctx context.Context, userID, id uuid.UUID, teas []uuid.UUID) (*model.Collection, error) {
	if _, err := m.storage.Collection(ctx, id, userID); err != nil {
		return nil, err
	}

	if err := m.storage.AddTeaToCollection(ctx, id, teas); err != nil {
		return nil, err
	}

	collection, err := m.storage.Collection(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	return &model.Collection{
		ID:     gqlCommon.ID(id),
		Name:   collection.Name,
		UserID: gqlCommon.ID(userID),
	}, nil
}

func (m *manager) DeleteTea(ctx context.Context, userID uuid.UUID, id uuid.UUID, teas []uuid.UUID) (*model.Collection, error) {
	if _, err := m.storage.Collection(ctx, id, userID); err != nil {
		return nil, err
	}

	if err := m.storage.DeleteTeaFromCollection(ctx, id, teas); err != nil {
		return nil, err
	}

	collection, err := m.storage.Collection(ctx, id, userID)
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
	return m.storage.DeleteCollection(ctx, id, userID)
}

func (m *manager) List(ctx context.Context, userID uuid.UUID) ([]*model.Collection, error) {
	list, err := m.storage.Collections(ctx, userID)
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
