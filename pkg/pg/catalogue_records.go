package pg

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"github.com/teaelephant/TeaElephantMemory/common"
)

const teaColumns = `id,name,type,COALESCE(description,''),revision::text`
const tagColumns = `id,name,color,category_id,revision::text`
const categoryColumns = `id,name,revision::text`

func (d *db) WriteRecord(ctx context.Context, data *common.TeaData) (*common.Tea, error) {
	return d.WriteRecordWithTags(ctx, data, nil)
}
func (d *db) ReadRecord(ctx context.Context, id uuid.UUID) (*common.Tea, error) {
	record, err := scanCatalogueTea(d.pg.QueryRowContext(ctx, `SELECT `+teaColumns+` FROM teas WHERE id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, common.ErrTeaNotFound
	}

	return record, err
}
func (d *db) ReadAllRecords(ctx context.Context, prefix string) ([]common.Tea, error) {
	rows, err := d.pg.QueryContext(ctx, `SELECT `+teaColumns+` FROM teas WHERE $1='' OR lower(name) LIKE lower($1)||'%' ORDER BY CASE WHEN $1='' THEN created_at END DESC, CASE WHEN $1<>'' THEN name END ASC,id`, prefix)
	if err != nil {
		return nil, wrapCatalogStorageError(err)
	}
	defer rows.Close()

	result := make([]common.Tea, 0)

	for rows.Next() {
		record, err := scanCatalogueTea(rows)
		if err != nil {
			return nil, wrapCatalogStorageError(err)
		}

		result = append(result, *record)
	}

	return result, wrapCatalogStorageError(rows.Err())
}
func (d *db) Update(ctx context.Context, id uuid.UUID, data *common.TeaData) (*common.Tea, error) {
	var result *common.Tea

	err := d.withCatalogueMutation(ctx, catalogTeaTable, id, func(tx *sql.Tx) error {
		var err error

		result, err = scanCatalogueTea(tx.QueryRowContext(ctx, `UPDATE teas SET name=$2,type=$3,description=$4 WHERE id=$1 RETURNING `+teaColumns, id, data.Name, data.Type.String(), data.Description))

		return wrapCatalogStorageError(err)
	})

	return result, err
}
func (d *db) Delete(ctx context.Context, id uuid.UUID) error {
	return d.withCatalogueMutation(ctx, catalogTeaTable, id, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM teas WHERE id=$1`, id)
		return wrapCatalogStorageError(err)
	})
}

func scanCategory(row rowScanner) (*common.TagCategory, error) {
	cat := &common.TagCategory{}
	err := row.Scan(&cat.ID, &cat.Name, &cat.Revision)

	return cat, wrapCatalogStorageError(err)
}
func scanTag(row rowScanner) (*common.Tag, error) {
	tag := &common.Tag{TagData: &common.TagData{}}
	err := row.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CategoryID, &tag.Revision)

	return tag, wrapCatalogStorageError(err)
}
func (d *db) CreateTagCategory(ctx context.Context, name string) (*common.TagCategory, error) {
	tx, err := beginCatalogueWrite(ctx, d.pg)
	if err != nil {
		return nil, wrapCatalogStorageError(err)
	}
	defer tx.Rollback()

	result, err := scanCategory(tx.QueryRowContext(ctx, `INSERT INTO tag_categories(id,name) VALUES($1,$2) RETURNING `+categoryColumns, uuid.New(), name))
	if err != nil {
		return nil, wrapCatalogStorageError(err)
	}

	return result, wrapCatalogStorageError(tx.Commit())
}
func (d *db) UpdateTagCategory(ctx context.Context, id uuid.UUID, name string) error {
	return d.withCatalogueMutation(ctx, catalogCategoryTable, id, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE tag_categories SET name=$2 WHERE id=$1`, id, name)
		return wrapCatalogStorageError(err)
	})
}
func (d *db) DeleteTagCategory(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	removed := make([]uuid.UUID, 0)
	err := d.withCatalogueMutation(ctx, catalogCategoryTable, id, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM tags WHERE category_id=$1 ORDER BY id`, id)
		if err != nil {
			return wrapCatalogStorageError(err)
		}
		defer rows.Close()

		for rows.Next() {
			var tagID uuid.UUID
			if err := rows.Scan(&tagID); err != nil {
				_ = rows.Close()
				return wrapCatalogStorageError(err)
			}

			removed = append(removed, tagID)
		}

		err = rows.Err()
		_ = rows.Close()

		if err != nil {
			return wrapCatalogStorageError(err)
		}

		if _, err = tx.ExecContext(ctx, `DELETE FROM tags WHERE category_id=$1`, id); err != nil {
			return wrapCatalogStorageError(err)
		}

		_, err = tx.ExecContext(ctx, `DELETE FROM tag_categories WHERE id=$1`, id)

		return wrapCatalogStorageError(err)
	})

	return removed, err
}
func (d *db) GetTagCategory(ctx context.Context, id uuid.UUID) (*common.TagCategory, error) {
	return scanCategory(d.pg.QueryRowContext(ctx, `SELECT `+categoryColumns+` FROM tag_categories WHERE id=$1`, id))
}
func (d *db) ListTagCategories(ctx context.Context, prefix *string) ([]common.TagCategory, error) {
	term := ""
	if prefix != nil {
		term = *prefix
	}

	rows, err := d.pg.QueryContext(ctx, `SELECT `+categoryColumns+` FROM tag_categories WHERE $1='' OR lower(name) LIKE lower($1)||'%' ORDER BY name,id`, term)
	if err != nil {
		return nil, wrapCatalogStorageError(err)
	}
	defer rows.Close()

	result := make([]common.TagCategory, 0)

	for rows.Next() {
		cat, err := scanCategory(rows)
		if err != nil {
			return nil, wrapCatalogStorageError(err)
		}

		result = append(result, *cat)
	}

	return result, wrapCatalogStorageError(rows.Err())
}
func (d *db) CreateTag(ctx context.Context, name, color string, categoryID uuid.UUID) (*common.Tag, error) {
	tx, err := beginCatalogueWrite(ctx, d.pg)
	if err != nil {
		return nil, wrapCatalogStorageError(err)
	}
	defer tx.Rollback()

	tag, err := scanTag(tx.QueryRowContext(ctx, `INSERT INTO tags(id,name,color,category_id) VALUES($1,$2,$3,$4) RETURNING `+tagColumns, uuid.New(), name, color, categoryID))
	if err != nil {
		return nil, wrapCatalogStorageError(err)
	}

	return tag, wrapCatalogStorageError(tx.Commit())
}
func (d *db) UpdateTag(ctx context.Context, id uuid.UUID, name, color string) (*common.Tag, error) {
	var result *common.Tag

	err := d.withCatalogueMutation(ctx, catalogTagTable, id, func(tx *sql.Tx) error {
		var err error

		result, err = scanTag(tx.QueryRowContext(ctx, `UPDATE tags SET name=$2,color=$3 WHERE id=$1 RETURNING `+tagColumns, id, name, color))

		return wrapCatalogStorageError(err)
	})

	return result, err
}
func (d *db) ChangeTagCategory(ctx context.Context, id, categoryID uuid.UUID) (*common.Tag, error) {
	var result *common.Tag

	err := d.withCatalogueMutation(ctx, catalogTagTable, id, func(tx *sql.Tx) error {
		var err error

		result, err = scanTag(tx.QueryRowContext(ctx, `UPDATE tags SET category_id=$2 WHERE id=$1 RETURNING `+tagColumns, id, categoryID))

		return wrapCatalogStorageError(err)
	})

	return result, err
}
func (d *db) DeleteTag(ctx context.Context, id uuid.UUID) error {
	return d.withCatalogueMutation(ctx, catalogTagTable, id, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM tags WHERE id=$1`, id)
		return wrapCatalogStorageError(err)
	})
}
func (d *db) GetTag(ctx context.Context, id uuid.UUID) (*common.Tag, error) {
	return scanTag(d.pg.QueryRowContext(ctx, `SELECT `+tagColumns+` FROM tags WHERE id=$1`, id))
}
func (d *db) ListTags(ctx context.Context, name *string, categoryID *uuid.UUID) ([]common.Tag, error) {
	prefix := ""
	if name != nil {
		prefix = *name
	}

	var category any
	if categoryID != nil {
		category = *categoryID
	}

	rows, err := d.pg.QueryContext(ctx, `SELECT `+tagColumns+` FROM tags WHERE ($1='' OR lower(name) LIKE lower($1)||'%') AND ($2::uuid IS NULL OR category_id=$2) ORDER BY name,id`, prefix, category)

	return collectTags(rows, err)
}
func collectTags(rows *sql.Rows, err error) ([]common.Tag, error) {
	if err != nil {
		return nil, wrapCatalogStorageError(err)
	}

	defer rows.Close()

	result := make([]common.Tag, 0)

	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, wrapCatalogStorageError(err)
		}

		result = append(result, *tag)
	}

	return result, wrapCatalogStorageError(rows.Err())
}
func (d *db) AddTagToTea(ctx context.Context, teaID, tagID uuid.UUID) error {
	return d.withCatalogueMutation(ctx, catalogTeaTable, teaID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO tea_tags(tea_id,tag_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, teaID, tagID)
		return wrapCatalogStorageError(err)
	})
}
func (d *db) DeleteTagFromTea(ctx context.Context, teaID, tagID uuid.UUID) error {
	return d.withCatalogueMutation(ctx, catalogTeaTable, teaID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM tea_tags WHERE tea_id=$1 AND tag_id=$2`, teaID, tagID)
		return wrapCatalogStorageError(err)
	})
}
func (d *db) ListByTea(ctx context.Context, id uuid.UUID) ([]common.Tag, error) {
	rows, err := d.pg.QueryContext(ctx, `SELECT t.id,t.name,t.color,t.category_id,t.revision::text FROM tags t JOIN tea_tags tt ON tt.tag_id=t.id WHERE tt.tea_id=$1 ORDER BY t.name,t.id`, id)
	return collectTags(rows, err)
}
