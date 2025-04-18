package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

func (d *pgDB) WriteQR(ctx context.Context, id uuid.UUID, data *common.QR) error {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return err
	}

	teaIDBytes, err := data.Tea.MarshalBinary()
	if err != nil {
		return err
	}

	_, err = d.dbConn.ExecContext(ctx, `
		INSERT INTO qr_codes (id, tea_id, bowling_temp, expiration_date)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		SET tea_id = $2, bowling_temp = $3, expiration_date = $4
	`, idBytes, teaIDBytes, data.BowlingTemp, data.ExpirationDate)

	return err
}

func (d *pgDB) ReadQR(ctx context.Context, id uuid.UUID) (*common.QR, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	var teaIDBytes []byte
	var bowlingTemp int
	var expirationDate time.Time
	err = d.dbConn.QueryRowContext(ctx, `
		SELECT tea_id, bowling_temp, expiration_date
		FROM qr_codes
		WHERE id = $1
	`, idBytes).Scan(&teaIDBytes, &bowlingTemp, &expirationDate)

	if err == sql.ErrNoRows {
		return nil, common.ErrQRRecordNotExist
	} else if err != nil {
		return nil, err
	}

	var teaID uuid.UUID
	if err := teaID.UnmarshalBinary(teaIDBytes); err != nil {
		return nil, fmt.Errorf("invalid tea ID in QR record: %w", err)
	}

	return &common.QR{
		Tea:            teaID,
		BowlingTemp:    bowlingTemp,
		ExpirationDate: expirationDate,
	}, nil
}