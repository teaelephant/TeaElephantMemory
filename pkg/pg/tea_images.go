package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

const teaImageColumns = `id, tea_id, content_type, width, height, created_at, is_cover`

// ListTeaImages returns metadata only; image bytes are read by the HTTP endpoint.
func (d *db) ListTeaImages(ctx context.Context, teaID uuid.UUID) ([]common.TeaImage, error) {
	rows, err := d.pg.QueryContext(ctx, `SELECT `+teaImageColumns+` FROM tea_images WHERE tea_id = $1 ORDER BY created_at, id`, teaID)
	if err != nil {
		return nil, fmt.Errorf("list tea images: %w", err)
	}
	defer rows.Close()

	images := make([]common.TeaImage, 0)

	for rows.Next() {
		var item common.TeaImage
		if err := rows.Scan(&item.ID, &item.TeaID, &item.ContentType, &item.Width, &item.Height, &item.CreatedAt, &item.IsCover); err != nil {
			return nil, fmt.Errorf("scan tea image: %w", err)
		}

		images = append(images, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tea images: %w", err)
	}

	return images, nil
}

// ReadTeaImage reads immutable bytes for a public image download.
func (d *db) ReadTeaImage(ctx context.Context, imageID uuid.UUID) (*common.TeaImageContent, error) {
	var content common.TeaImageContent

	err := d.pg.QueryRowContext(ctx, `SELECT `+teaImageColumns+`, data FROM tea_images WHERE id = $1`, imageID).
		Scan(&content.ID, &content.TeaID, &content.ContentType, &content.Width, &content.Height, &content.CreatedAt, &content.IsCover, &content.Data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, common.ErrTeaImageNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("read tea image: %w", err)
	}

	return &content, nil
}

// withLockedTea serializes every image write for one catalog entry.
func (d *db) withLockedTea(ctx context.Context, teaID uuid.UUID, update func(*sql.Tx) error) error {
	tx, err := d.pg.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tea image transaction: %w", err)
	}
	defer tx.Rollback()

	var found uuid.UUID

	err = tx.QueryRowContext(ctx, `SELECT id FROM teas WHERE id = $1 FOR UPDATE`, teaID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return common.ErrTeaNotFound
	}

	if err != nil {
		return fmt.Errorf("lock tea images: %w", err)
	}

	if err := update(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tea images: %w", err)
	}

	return nil
}

// InsertTeaImage enforces the image count and first-image cover under the tea lock.
func (d *db) InsertTeaImage(ctx context.Context, content *common.TeaImageContent) error {
	return d.withLockedTea(ctx, content.TeaID, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM tea_images WHERE tea_id = $1`, content.TeaID).Scan(&count); err != nil {
			return fmt.Errorf("count tea images: %w", err)
		}

		if count >= common.MaxTeaImages {
			return common.ErrTeaImageLimit
		}

		_, err := tx.ExecContext(ctx, `INSERT INTO tea_images (id, tea_id, content_type, width, height, data, is_cover)
            VALUES ($1, $2, $3, $4, $5, $6, $7)`, content.ID, content.TeaID, content.ContentType, content.Width, content.Height, content.Data, count == 0)
		if err != nil {
			return fmt.Errorf("insert tea image: %w", err)
		}

		return nil
	})
}

// DeleteTeaImage cannot delete another tea's image and promotes the oldest survivor.
func (d *db) DeleteTeaImage(ctx context.Context, teaID, imageID uuid.UUID) error {
	return d.withLockedTea(ctx, teaID, func(tx *sql.Tx) error {
		var wasCover bool

		err := tx.QueryRowContext(ctx, `DELETE FROM tea_images WHERE tea_id = $1 AND id = $2 RETURNING is_cover`, teaID, imageID).Scan(&wasCover)
		if errors.Is(err, sql.ErrNoRows) {
			return common.ErrTeaImageNotFound
		}

		if err != nil {
			return fmt.Errorf("delete tea image: %w", err)
		}

		if wasCover {
			_, err = tx.ExecContext(ctx, `UPDATE tea_images SET is_cover = true WHERE id =
                (SELECT id FROM tea_images WHERE tea_id = $1 ORDER BY created_at, id LIMIT 1)`, teaID)
			if err != nil {
				return fmt.Errorf("promote tea cover image: %w", err)
			}
		}

		return nil
	})
}

// SetTeaCoverImage changes the single cover atomically without reordering images.
func (d *db) SetTeaCoverImage(ctx context.Context, teaID, imageID uuid.UUID) error {
	return d.withLockedTea(ctx, teaID, func(tx *sql.Tx) error {
		var found uuid.UUID

		err := tx.QueryRowContext(ctx, `SELECT id FROM tea_images WHERE tea_id = $1 AND id = $2`, teaID, imageID).Scan(&found)
		if errors.Is(err, sql.ErrNoRows) {
			return common.ErrTeaImageNotFound
		}

		if err != nil {
			return fmt.Errorf("find tea cover image: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `UPDATE tea_images SET is_cover = false WHERE tea_id = $1 AND is_cover`, teaID); err != nil {
			return fmt.Errorf("clear tea cover image: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `UPDATE tea_images SET is_cover = true WHERE tea_id = $1 AND id = $2`, teaID, imageID); err != nil {
			return fmt.Errorf("set tea cover image: %w", err)
		}

		return nil
	})
}
