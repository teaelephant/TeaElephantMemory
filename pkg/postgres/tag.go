package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

// Error definitions
var (
	ErrTagCategoryNotFound = errors.New("tag category not found")
	ErrTagNotFound         = errors.New("tag not found")
)

// CreateTagCategory creates a new tag category
func (d *pgDB) CreateTagCategory(ctx context.Context, name string) (*common.TagCategory, error) {
	id := uuid.New()

	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	_, err = d.dbConn.ExecContext(ctx, `
		INSERT INTO tag_categories (id, name)
		VALUES ($1, $2)
	`, idBytes, name)

	if err != nil {
		return nil, err
	}

	return &common.TagCategory{
		ID:   id,
		Name: name,
	}, nil
}

// UpdateTagCategory updates an existing tag category
func (d *pgDB) UpdateTagCategory(ctx context.Context, id uuid.UUID, name string) error {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return err
	}

	result, err := d.dbConn.ExecContext(ctx, `
		UPDATE tag_categories
		SET name = $2
		WHERE id = $1
	`, idBytes, name)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%w: %s", ErrTagCategoryNotFound, id)
	}

	return nil
}

// DeleteTagCategory deletes a tag category and returns the IDs of removed tags
func (d *pgDB) DeleteTagCategory(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	// Get all tags in this category
	rows, err := d.dbConn.QueryContext(ctx, `
		SELECT id FROM tags WHERE category_id = $1
	`, idBytes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	removedTags := make([]uuid.UUID, 0)

	for rows.Next() {
		var tagIDBytes []byte
		if err := rows.Scan(&tagIDBytes); err != nil {
			return nil, err
		}

		var tagID uuid.UUID
		if err := tagID.UnmarshalBinary(tagIDBytes); err != nil {
			return nil, err
		}

		removedTags = append(removedTags, tagID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Delete the category (cascade will delete tags)
	_, err = d.dbConn.ExecContext(ctx, `
		DELETE FROM tag_categories WHERE id = $1
	`, idBytes)
	if err != nil {
		return nil, err
	}

	return removedTags, nil
}

// GetTagCategory retrieves a tag category by ID
func (d *pgDB) GetTagCategory(ctx context.Context, id uuid.UUID) (*common.TagCategory, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	var name string
	err = d.dbConn.QueryRowContext(ctx, `
		SELECT name FROM tag_categories WHERE id = $1
	`, idBytes).Scan(&name)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrTagCategoryNotFound, id)
	} else if err != nil {
		return nil, err
	}

	return &common.TagCategory{
		ID:   id,
		Name: name,
	}, nil
}

// ListTagCategories lists all tag categories, optionally filtered by name
func (d *pgDB) ListTagCategories(ctx context.Context, search *string) ([]common.TagCategory, error) {
	var rows *sql.Rows

	var err error

	if search == nil || *search == "" {
		rows, err = d.dbConn.QueryContext(ctx, `
			SELECT id, name FROM tag_categories
		`)
	} else {
		searchPattern := "%" + strings.ToLower(*search) + "%"
		rows, err = d.dbConn.QueryContext(ctx, `
			SELECT id, name FROM tag_categories
			WHERE LOWER(name) LIKE $1
		`, searchPattern)
	}

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	categories := make([]common.TagCategory, 0)

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

		categories = append(categories, common.TagCategory{
			ID:   id,
			Name: name,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

// CreateTag creates a new tag
func (d *pgDB) CreateTag(ctx context.Context, name, color string, categoryID uuid.UUID) (*common.Tag, error) {
	id := uuid.New()

	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	categoryIDBytes, err := categoryID.MarshalBinary()
	if err != nil {
		return nil, err
	}

	_, err = d.dbConn.ExecContext(ctx, `
		INSERT INTO tags (id, name, color, category_id)
		VALUES ($1, $2, $3, $4)
	`, idBytes, name, color, categoryIDBytes)

	if err != nil {
		return nil, err
	}

	return &common.Tag{
		ID: id,
		TagData: &common.TagData{
			Name:       name,
			Color:      color,
			CategoryID: categoryID,
		},
	}, nil
}

// UpdateTag updates an existing tag
func (d *pgDB) UpdateTag(ctx context.Context, id uuid.UUID, name, color string) (*common.Tag, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	// Get the current category ID
	var categoryIDBytes []byte
	err = d.dbConn.QueryRowContext(ctx, `
		SELECT category_id FROM tags WHERE id = $1
	`, idBytes).Scan(&categoryIDBytes)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrTagNotFound, id)
	} else if err != nil {
		return nil, err
	}

	var categoryID uuid.UUID
	if err := categoryID.UnmarshalBinary(categoryIDBytes); err != nil {
		return nil, err
	}

	// Update the tag
	_, err = d.dbConn.ExecContext(ctx, `
		UPDATE tags
		SET name = $2, color = $3
		WHERE id = $1
	`, idBytes, name, color)

	if err != nil {
		return nil, err
	}

	return &common.Tag{
		ID: id,
		TagData: &common.TagData{
			Name:       name,
			Color:      color,
			CategoryID: categoryID,
		},
	}, nil
}

// ChangeTagCategory changes the category of a tag
func (d *pgDB) ChangeTagCategory(ctx context.Context, id, categoryID uuid.UUID) (*common.Tag, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	categoryIDBytes, err := categoryID.MarshalBinary()
	if err != nil {
		return nil, err
	}

	// Get the current tag data
	var name, color string
	err = d.dbConn.QueryRowContext(ctx, `
		SELECT name, color FROM tags WHERE id = $1
	`, idBytes).Scan(&name, &color)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrTagNotFound, id)
	} else if err != nil {
		return nil, err
	}

	// Update the tag's category
	_, err = d.dbConn.ExecContext(ctx, `
		UPDATE tags
		SET category_id = $2
		WHERE id = $1
	`, idBytes, categoryIDBytes)

	if err != nil {
		return nil, err
	}

	return &common.Tag{
		ID: id,
		TagData: &common.TagData{
			Name:       name,
			Color:      color,
			CategoryID: categoryID,
		},
	}, nil
}

// DeleteTag deletes a tag
func (d *pgDB) DeleteTag(ctx context.Context, id uuid.UUID) error {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return err
	}

	_, err = d.dbConn.ExecContext(ctx, `
		DELETE FROM tags WHERE id = $1
	`, idBytes)

	return err
}

// GetTag retrieves a tag by ID
func (d *pgDB) GetTag(ctx context.Context, id uuid.UUID) (*common.Tag, error) {
	idBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	var name, color string

	var categoryIDBytes []byte
	err = d.dbConn.QueryRowContext(ctx, `
		SELECT name, color, category_id FROM tags WHERE id = $1
	`, idBytes).Scan(&name, &color, &categoryIDBytes)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrTagNotFound, id)
	} else if err != nil {
		return nil, err
	}

	var categoryID uuid.UUID
	if err := categoryID.UnmarshalBinary(categoryIDBytes); err != nil {
		return nil, err
	}

	return &common.Tag{
		ID: id,
		TagData: &common.TagData{
			Name:       name,
			Color:      color,
			CategoryID: categoryID,
		},
	}, nil
}

// ListTags lists all tags, optionally filtered by name and/or category ID
func (d *pgDB) ListTags(ctx context.Context, name *string, categoryID *uuid.UUID) ([]common.Tag, error) {
	var query string

	var args []interface{}

	var argCount int

	query = "SELECT id, name, color, category_id FROM tags"

	// Build WHERE clause
	var conditions []string

	if name != nil && *name != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("LOWER(name) LIKE $%d", argCount))
		args = append(args, "%"+strings.ToLower(*name)+"%")
	}

	if categoryID != nil {
		categoryIDBytes, err := categoryID.MarshalBinary()
		if err != nil {
			return nil, err
		}

		argCount++
		conditions = append(conditions, fmt.Sprintf("category_id = $%d", argCount))
		args = append(args, categoryIDBytes)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := d.dbConn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]common.Tag, 0)

	for rows.Next() {
		var idBytes, catIDBytes []byte

		var name, color string

		if err := rows.Scan(&idBytes, &name, &color, &catIDBytes); err != nil {
			return nil, err
		}

		var id, catID uuid.UUID
		if err := id.UnmarshalBinary(idBytes); err != nil {
			return nil, err
		}

		if err := catID.UnmarshalBinary(catIDBytes); err != nil {
			return nil, err
		}

		tags = append(tags, common.Tag{
			ID: id,
			TagData: &common.TagData{
				Name:       name,
				Color:      color,
				CategoryID: catID,
			},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tags, nil
}

// AddTagToTea adds a tag to a tea record
func (d *pgDB) AddTagToTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error {
	teaIDBytes, err := tea.MarshalBinary()
	if err != nil {
		return err
	}

	tagIDBytes, err := tag.MarshalBinary()
	if err != nil {
		return err
	}

	_, err = d.dbConn.ExecContext(ctx, `
		INSERT INTO tea_tags (tea_id, tag_id)
		VALUES ($1, $2)
		ON CONFLICT (tea_id, tag_id) DO NOTHING
	`, teaIDBytes, tagIDBytes)

	return err
}

// DeleteTagFromTea removes a tag from a tea record
func (d *pgDB) DeleteTagFromTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error {
	teaIDBytes, err := tea.MarshalBinary()
	if err != nil {
		return err
	}

	tagIDBytes, err := tag.MarshalBinary()
	if err != nil {
		return err
	}

	_, err = d.dbConn.ExecContext(ctx, `
		DELETE FROM tea_tags
		WHERE tea_id = $1 AND tag_id = $2
	`, teaIDBytes, tagIDBytes)

	return err
}

// ListByTea lists all tags associated with a tea record
func (d *pgDB) ListByTea(ctx context.Context, id uuid.UUID) ([]common.Tag, error) {
	teaIDBytes, err := id.MarshalBinary()
	if err != nil {
		return nil, err
	}

	rows, err := d.dbConn.QueryContext(ctx, `
		SELECT t.id, t.name, t.color, t.category_id
		FROM tags t
		JOIN tea_tags tt ON t.id = tt.tag_id
		WHERE tt.tea_id = $1
	`, teaIDBytes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]common.Tag, 0)

	for rows.Next() {
		var idBytes, catIDBytes []byte

		var name, color string

		if err := rows.Scan(&idBytes, &name, &color, &catIDBytes); err != nil {
			return nil, err
		}

		var id, catID uuid.UUID
		if err := id.UnmarshalBinary(idBytes); err != nil {
			return nil, err
		}

		if err := catID.UnmarshalBinary(catIDBytes); err != nil {
			return nil, err
		}

		tags = append(tags, common.Tag{
			ID: id,
			TagData: &common.TagData{
				Name:       name,
				Color:      color,
				CategoryID: catID,
			},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tags, nil
}
