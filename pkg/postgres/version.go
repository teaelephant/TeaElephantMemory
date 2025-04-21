package postgres

import (
	"context"
	"database/sql"
	"errors"
)

func (d *pgDB) GetVersion(ctx context.Context) (uint32, error) {
	var version uint32
	err := d.dbConn.QueryRowContext(ctx, `
		SELECT version FROM version WHERE id = 1
	`).Scan(&version)

	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return version, nil
}

func (d *pgDB) WriteVersion(ctx context.Context, version uint32) error {
	_, err := d.dbConn.ExecContext(ctx, `
		INSERT INTO version (id, version)
		VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE
		SET version = $1
	`, version)

	return err
}
