package fdb

import (
	"context"

	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
)

type notification interface {
	CreateOrUpdateDeviceToken(ctx context.Context, userID, deviceID uuid.UUID, deviceToken string) error
	Notifications(ctx context.Context, userID uuid.UUID) ([]common.Notification, error)
}

func (d *db) CreateOrUpdateDeviceToken(ctx context.Context, userID, deviceID uuid.UUID, deviceToken string) error {
	key := d.keyBuilder.Device(deviceID)

	index := d.keyBuilder.DevicesByUserID(userID)

	el, err := (*encoder.Device)(&common.Device{
		UserID: userID,
		Token:  deviceToken,
	}).Encode()
	if err != nil {
		return err
	}

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	if err = tr.Set(key, el); err != nil {
		return err
	}

	if err = tr.Set(index, deviceID.Bytes()); err != nil {
		return err
	}

	return tr.Commit()
}

func (d *db) Notifications(ctx context.Context, userID uuid.UUID) ([]common.Notification, error) {
	key := d.keyBuilder.NotificationByUserID(userID)

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	data, err := tr.Get(key)
	if err != nil {
		return nil, err
	}
	nt := make([]uuid.UUID, 0)

	if err = encoder.Decode(data, nt); err != nil {
		return nil, err
	}

	res := make([]common.Notification, len(nt))

	for i, id := range nt {
		key = d.keyBuilder.Notification(id)

		data, err := tr.Get(key)
		if err != nil {
			return nil, err
		}

		el := &encoder.Notification{}
		if err = el.Decode(data); err != nil {
			return nil, err
		}

		res[i] = common.Notification(*el)
	}

	return res, nil
}
