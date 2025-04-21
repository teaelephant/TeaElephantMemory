package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

// CreateCollection creates a new collection for a user
func (d *pgDB) CreateCollection(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error) {
	id := uuid.New()

	idBytes, err := id.MarshalBinary()
	if err != nil {
		return uuid.UUID{}, err
	}

	userIDBytes, err := userID.MarshalBinary()
	if err != nil {
		return uuid.UUID{}, err
	}

	_, err = d.dbConn.ExecContext(ctx, `
		INSERT INTO collections (id, name, user_id)
		VALUES ($1, $2, $3)
	`, idBytes, name, userIDBytes)

	if err != nil {
		return uuid.UUID{}, err
	}

	return id, nil
}

// AddTeaToCollection adds tea records to a collection
func (d *pgDB) AddTeaToCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return err
	}

	tx, err := d.dbConn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO collection_teas (collection_id, tea_id)
		VALUES ($1, $2)
		ON CONFLICT (collection_id, tea_id) DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, tea := range teas {
		// Verify tea exists
		teaIDBytes, err := tea.MarshalBinary()
		if err != nil {
			return err
		}

		var exists bool

		err = tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM tea_records WHERE id = $1)
		`, teaIDBytes).Scan(&exists)
		if err != nil {
			return err
		}

		if !exists {
			continue
		}

		_, err = stmt.ExecContext(ctx, idBytes, teaIDBytes)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteTeaFromCollection removes tea records from a collection
func (d *pgDB) DeleteTeaFromCollection(ctx context.Context, id uuid.UUID, teas []uuid.UUID) error {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return err
	}

	tx, err := d.dbConn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		DELETE FROM collection_teas
		WHERE collection_id = $1 AND tea_id = $2
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, tea := range teas {
		teaIDBytes, err := tea.MarshalBinary()
		if err != nil {
			return err
		}

		_, err = stmt.ExecContext(ctx, idBytes, teaIDBytes)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteCollection deletes a collection
func (d *pgDB) DeleteCollection(ctx context.Context, id, userID uuid.UUID) error {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return err
	}

	userIDBytes, err := userID.MarshalBinary()
	if err != nil {
		return err
	}

	result, err := d.dbConn.ExecContext(ctx, `
		DELETE FROM collections
		WHERE id = $1 AND user_id = $2
	`, idBytes, userIDBytes)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return common.ErrCollectionNotFound
	}

	return nil
}

// Collections returns all collections for a user
func (d *pgDB) Collections(ctx context.Context, userID uuid.UUID) ([]*common.Collection, error) {
	userIDBytes, err := userID.MarshalBinary()
	if err != nil {
		return nil, err
	}

	rows, err := d.dbConn.QueryContext(ctx, `
		SELECT id, name
		FROM collections
		WHERE user_id = $1
	`, userIDBytes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	collections := make([]*common.Collection, 0)

	for rows.Next() {
		var idBytes []byte

		var name string

		if err := rows.Scan(&idBytes, &name); err != nil {
			return nil, err
		}

		var id uuid.UUID
		if err := id.UnmarshalBinary(idBytes); err != nil {
			return nil, err
		}

		collections = append(collections, &common.Collection{
			ID:   id,
			Name: name,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return collections, nil
}

// Collection returns a specific collection for a user
func (d *pgDB) Collection(ctx context.Context, id, userID uuid.UUID) (*common.Collection, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	userIDBytes, err := userID.MarshalBinary()
	if err != nil {
		return nil, err
	}

	var name string
	err = d.dbConn.QueryRowContext(ctx, `
		SELECT name
		FROM collections
		WHERE id = $1 AND user_id = $2
	`, idBytes, userIDBytes).Scan(&name)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, common.ErrCollectionNotFound
	} else if err != nil {
		return nil, err
	}

	return &common.Collection{
		ID:   id,
		Name: name,
	}, nil
}

// CollectionRecords returns all tea records in a collection
func (d *pgDB) CollectionRecords(ctx context.Context, id uuid.UUID) ([]*common.CollectionRecord, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	rows, err := d.dbConn.QueryContext(ctx, `
		SELECT qr.id, tr.id, tr.name, tr.type, tr.description, qr.bowling_temp, qr.expiration_date
		FROM collection_teas ct
		JOIN tea_records tr ON ct.tea_id = tr.id
		LEFT JOIN qr_codes qr ON tr.id = qr.tea_id
		WHERE ct.collection_id = $1
	`, idBytes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]*common.CollectionRecord, 0)

	for rows.Next() {
		var qrIDBytes, teaIDBytes []byte

		var name, description string

		var teaType int

		var bowlingTemp sql.NullInt32

		var expirationDate sql.NullTime

		if err := rows.Scan(&qrIDBytes, &teaIDBytes, &name, &teaType, &description, &bowlingTemp, &expirationDate); err != nil {
			return nil, err
		}

		var qrID, teaID uuid.UUID
		if err := qrID.UnmarshalBinary(qrIDBytes); err != nil {
			return nil, fmt.Errorf("invalid QR ID: %w", err)
		}

		if err := teaID.UnmarshalBinary(teaIDBytes); err != nil {
			return nil, fmt.Errorf("invalid tea ID: %w", err)
		}

		tea := &common.Tea{
			ID: teaID,
			TeaData: &common.TeaData{
				Name:        name,
				Type:        common.BeverageType(teaType),
				Description: description,
			},
		}

		bTemp := 0
		if bowlingTemp.Valid {
			bTemp = int(bowlingTemp.Int32)
		}

		expDate := expirationDate.Time
		if !expirationDate.Valid {
			expDate = expirationDate.Time
		}

		records = append(records, &common.CollectionRecord{
			ID:             qrID,
			Tea:            tea,
			BowlingTemp:    bTemp,
			ExpirationDate: expDate,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}
