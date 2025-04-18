package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

// CreateOrUpdateDeviceToken creates or updates a device token
func (d *pgDB) CreateOrUpdateDeviceToken(ctx context.Context, deviceID uuid.UUID, deviceToken string) error {
	idBytes, err := deviceID.MarshalBinary()
	if err != nil {
		return err
	}

	_, err = d.dbConn.ExecContext(ctx, `
		INSERT INTO devices (id, token)
		VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE
		SET token = $2
	`, idBytes, deviceToken)

	return err
}

// Notifications retrieves all notifications for a user
func (d *pgDB) Notifications(ctx context.Context, userID uuid.UUID) ([]common.Notification, error) {
	userIDBytes, err := userID.MarshalBinary()
	if err != nil {
		return nil, err
	}

	rows, err := d.dbConn.QueryContext(ctx, `
		SELECT id, user_id, type
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userIDBytes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notifications := make([]common.Notification, 0)
	for rows.Next() {
		var idBytes, userIDBytes []byte
		var notificationType int

		if err := rows.Scan(&idBytes, &userIDBytes, &notificationType); err != nil {
			return nil, err
		}

		var userID uuid.UUID
		if err := userID.UnmarshalBinary(userIDBytes); err != nil {
			return nil, err
		}

		notifications = append(notifications, common.Notification{
			UserID: userID,
			Type:   common.NotificationType(notificationType),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notifications, nil
}

// AddDeviceForUser associates a device with a user
func (d *pgDB) AddDeviceForUser(ctx context.Context, userID, deviceID uuid.UUID) error {
	userIDBytes, err := userID.MarshalBinary()
	if err != nil {
		return err
	}

	deviceIDBytes, err := deviceID.MarshalBinary()
	if err != nil {
		return err
	}

	// Check if device exists
	var token string
	err = d.dbConn.QueryRowContext(ctx, `
		SELECT token FROM devices WHERE id = $1
	`, deviceIDBytes).Scan(&token)

	if err == sql.ErrNoRows {
		return common.ErrDeviceNotFound
	} else if err != nil {
		return err
	}

	// Update device with user ID
	_, err = d.dbConn.ExecContext(ctx, `
		UPDATE devices
		SET user_id = $1
		WHERE id = $2
	`, userIDBytes, deviceIDBytes)

	return err
}

// MapUserIdToDeviceID maps a user ID to device tokens
func (d *pgDB) MapUserIdToDeviceID(ctx context.Context, userID uuid.UUID) ([]string, error) {
	userIDBytes, err := userID.MarshalBinary()
	if err != nil {
		return nil, err
	}

	rows, err := d.dbConn.QueryContext(ctx, `
		SELECT token
		FROM devices
		WHERE user_id = $1
	`, userIDBytes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := make([]string, 0)
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tokens, nil
}
