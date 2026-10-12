package consumption

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/teaelephant/TeaElephantMemory/common"
)

// RecordQR reads the selected jar and records its current Tea in a short transaction.
// The catalog lock prevents a merge between selection and the history insert;
// the QR share lock also preserves concurrent user jar edits until commit.
func (s *PGStore) RecordQR(ctx context.Context, userID, qrID uuid.UUID, ts time.Time) (*common.CollectionRecord, error) {
	if s.pg == nil {
		return nil, ErrNilDB
	}

	tx, err := s.pg.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin selected QR consumption: %w", err)
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, common.CatalogueWriteLock); err != nil {
		return nil, fmt.Errorf("lock selected QR consumption: %w", err)
	}

	record := &common.CollectionRecord{Tea: &common.Tea{TeaData: &common.TeaData{}}}

	var kind string

	// Lock the label first, then read its Tea in a fresh statement snapshot. A
	// waiting JOIN/FOR SHARE can discard its row when the Tea reference changes.
	err = tx.QueryRowContext(ctx, `SELECT id,boiling_temp,expiration_date,tea_id FROM qr_records WHERE id=$1 FOR SHARE`, qrID).
		Scan(&record.ID, &record.BowlingTemp, &record.ExpirationDate, &record.Tea.ID)
	if err != nil {
		return nil, fmt.Errorf("read selected QR consumption: %w", err)
	}

	err = tx.QueryRowContext(ctx, `SELECT name,type,COALESCE(description,''),revision::text FROM teas WHERE id=$1`, record.Tea.ID).
		Scan(&record.Tea.Name, &kind, &record.Tea.Description, &record.Tea.Revision)
	if err != nil {
		return nil, fmt.Errorf("read selected QR Tea: %w", err)
	}

	record.Tea.Type = common.StringToBeverageType(kind)
	if _, err = tx.ExecContext(ctx, `INSERT INTO consumptions(user_id,ts,tea_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, userID, ts.UTC(), record.Tea.ID); err != nil {
		return nil, fmt.Errorf("record selected QR consumption: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM consumptions WHERE user_id=$1 AND ts<$2`, userID, ts.UTC().Add(-s.retention)); err != nil {
		return nil, fmt.Errorf("prune selected QR consumption: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit selected QR consumption: %w", err)
	}

	return record, nil
}
