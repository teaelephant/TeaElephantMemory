package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

func (d *pgDB) WriteRecord(ctx context.Context, rec *common.TeaData) (*common.Tea, error) {
	id := uuid.New()
	return d.writeRecord(ctx, id, rec)
}

func (d *pgDB) ReadRecord(ctx context.Context, id uuid.UUID) (*common.Tea, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	var name, description string
	var teaType int
	err = d.dbConn.QueryRowContext(ctx, `
		SELECT name, type, description 
		FROM tea_records 
		WHERE id = $1
	`, idBytes).Scan(&name, &teaType, &description)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tea record not found: %s", id)
	} else if err != nil {
		return nil, err
	}

	return &common.Tea{
		ID: id,
		TeaData: &common.TeaData{
			Name:        name,
			Type:        common.BeverageType(teaType),
			Description: description,
		},
	}, nil
}

func (d *pgDB) ReadAllRecords(ctx context.Context, search string) ([]common.Tea, error) {
	var rows *sql.Rows
	var err error

	if search == "" {
		rows, err = d.dbConn.QueryContext(ctx, `
			SELECT id, name, type, description 
			FROM tea_records
		`)
	} else {
		searchPattern := "%" + strings.ToLower(search) + "%"
		rows, err = d.dbConn.QueryContext(ctx, `
			SELECT id, name, type, description 
			FROM tea_records 
			WHERE LOWER(name) LIKE $1
		`, searchPattern)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]common.Tea, 0)
	for rows.Next() {
		var idBytes []byte
		var name, description string
		var teaType int

		if err := rows.Scan(&idBytes, &name, &teaType, &description); err != nil {
			return nil, err
		}

		var id uuid.UUID
		if err := id.UnmarshalBinary(idBytes); err != nil {
			return nil, err
		}

		records = append(records, common.Tea{
			ID: id,
			TeaData: &common.TeaData{
				Name:        name,
				Type:        common.BeverageType(teaType),
				Description: description,
			},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

func (d *pgDB) Update(ctx context.Context, id uuid.UUID, rec *common.TeaData) (*common.Tea, error) {
	return d.writeRecord(ctx, id, rec)
}

func (d *pgDB) Delete(ctx context.Context, id uuid.UUID) error {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return err
	}

	_, err = d.dbConn.ExecContext(ctx, `
		DELETE FROM tea_records 
		WHERE id = $1
	`, idBytes)

	return err
}

func (d *pgDB) writeRecord(ctx context.Context, id uuid.UUID, rec *common.TeaData) (*common.Tea, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	_, err = d.dbConn.ExecContext(ctx, `
		INSERT INTO tea_records (id, name, type, description)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		SET name = $2, type = $3, description = $4
	`, idBytes, rec.Name, int(rec.Type), rec.Description)

	if err != nil {
		return nil, err
	}

	return &common.Tea{
		ID:      id,
		TeaData: rec,
	}, nil
}
