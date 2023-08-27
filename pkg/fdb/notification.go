package fdb

import (
	"context"

	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
)

type notification interface {
	CreateOrUpdateDeviceToken(ctx context.Context, deviceID uuid.UUID, deviceToken string) error
	Notifications(ctx context.Context, userID uuid.UUID) ([]common.Notification, error)
	AddDeviceForUser(ctx context.Context, userID, deviceID uuid.UUID) error
}

func (d *db) AddDeviceForUser(ctx context.Context, userID, deviceID uuid.UUID) error {
	key := d.keyBuilder.Device(deviceID)

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	data, err := tr.Get(key)
	if err != nil {
		return err
	}

	el := &encoder.Device{}
	if err = el.Decode(data); err != nil {
		return err
	}

	if el.UserID != userID {
		index := d.keyBuilder.DevicesByUserID(userID)

		data, err = tr.Get(index)
		if err != nil {
			return err
		}

		devices := make([]uuid.UUID, 0)

		if err = encoder.Decode(data, devices); err != nil {
			return err
		}

		for _, device := range devices {
			if device == deviceID {
				return nil
			}
		}
		devices = append(devices, deviceID)

		data, err = encoder.Encode(devices)
		if err != nil {
			return err
		}

		if err = tr.Set(index, data); err != nil {
			return err
		}
	}

	return tr.Commit()
}

func (d *db) CreateOrUpdateDeviceToken(ctx context.Context, deviceID uuid.UUID, deviceToken string) error {
	key := d.keyBuilder.Device(deviceID)

	el, err := (*encoder.Device)(&common.Device{
		Token: deviceToken,
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
