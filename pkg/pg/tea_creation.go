package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

// WriteRecordWithTags commits a fresh Tea and its initial existing tag links together.
func (d *db) WriteRecordWithTags(ctx context.Context, rec *common.TeaData, tagIDs []uuid.UUID) (*common.Tea, error) {
	tx, err := d.pg.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tea creation: %w", err)
	}
	defer tx.Rollback()

	tags, err := lockInitialTeaTags(ctx, tx, tagIDs)
	if err != nil {
		return nil, err
	}

	id := uuid.New()
	if _, err := tx.ExecContext(ctx, `INSERT INTO teas(id,name,type,description) VALUES($1,$2,$3,$4)`,
		id, rec.Name, rec.Type.String(), rec.Description); err != nil {
		return nil, fmt.Errorf("insert new tea: %w", err)
	}

	for _, tagID := range tags {
		if _, err := tx.ExecContext(ctx, `INSERT INTO tea_tags(tea_id,tag_id) VALUES($1,$2)`, id, tagID); err != nil {
			return nil, fmt.Errorf("insert initial tea tag: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tea creation: %w", err)
	}

	return &common.Tea{ID: id, TeaData: &common.TeaData{Name: rec.Name, Type: rec.Type, Description: rec.Description}}, nil
}

func lockInitialTeaTags(ctx context.Context, tx *sql.Tx, tagIDs []uuid.UUID) ([]uuid.UUID, error) {
	tags := make([]uuid.UUID, 0, len(tagIDs))
	seen := make(map[uuid.UUID]bool, len(tagIDs))

	for _, tagID := range tagIDs {
		if seen[tagID] {
			continue
		}

		var found uuid.UUID

		err := tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE id=$1 FOR KEY SHARE`, tagID).Scan(&found)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, common.ErrTagNotFound
		}

		if err != nil {
			return nil, fmt.Errorf("validate initial tea tag: %w", err)
		}

		seen[tagID] = true

		tags = append(tags, found)
	}

	return tags, nil
}
