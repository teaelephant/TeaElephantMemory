package notification

import (
	"context"

	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

type Manager interface {
	RegisterDeviceToken(ctx context.Context, userID, deviceID uuid.UUID, deviceToken string) error
	Notifications(ctx context.Context, userID uuid.UUID) ([]common.Notification, error)
}

type repository interface {
	CreateOrUpdateDeviceToken(ctx context.Context, userID, deviceID uuid.UUID, deviceToken string) error
	Notifications(ctx context.Context, userID uuid.UUID) ([]common.Notification, error)
}

type manager struct {
	repository
}

func (m *manager) RegisterDeviceToken(ctx context.Context, userID, deviceID uuid.UUID, deviceToken string) error {
	return m.repository.CreateOrUpdateDeviceToken(ctx, userID, deviceID, deviceToken)
}

func NewManager(repository repository) Manager {
	return &manager{repository: repository}
}
