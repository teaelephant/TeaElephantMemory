package notification

import (
	"context"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

type Manager interface {
	BindDevice(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) error
	RegisterDeviceToken(ctx context.Context, deviceID uuid.UUID, deviceToken string) error
	Notifications(ctx context.Context, userID uuid.UUID) ([]common.Notification, error)
}

type repository interface {
	CreateOrUpdateDeviceToken(ctx context.Context, deviceID uuid.UUID, deviceToken string) error
	AddDeviceForUser(ctx context.Context, userID, deviceID uuid.UUID) error
	Notifications(ctx context.Context, userID uuid.UUID) ([]common.Notification, error)
}

type manager struct {
	repository
}

func (m *manager) BindDevice(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) error {
	return m.AddDeviceForUser(ctx, userID, deviceID)
}

func (m *manager) RegisterDeviceToken(ctx context.Context, deviceID uuid.UUID, deviceToken string) error {
	return m.repository.CreateOrUpdateDeviceToken(ctx, deviceID, deviceToken)
}

func NewManager(repository repository) Manager {
	return &manager{repository: repository}
}
