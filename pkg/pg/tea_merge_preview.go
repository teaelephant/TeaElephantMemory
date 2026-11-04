package pg

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/teaelephant/TeaElephantMemory/common"
)

func readTeaMergePreview(ctx context.Context, tx *sql.Tx, source, target uuid.UUID) (*common.TeaMergePreview, error) {
	teas, err := readTeaMergePair(ctx, tx, source, target)
	if err != nil {
		return nil, err
	}

	preview := &common.TeaMergePreview{Blockers: make([]string, 0)}

	preview.Source, err = readTeaMergeSnapshot(ctx, tx, teas[source])
	if err != nil {
		return nil, err
	}

	preview.Target, err = readTeaMergeSnapshot(ctx, tx, teas[target])
	if err != nil {
		return nil, err
	}

	dependencies, err := readTeaMergeDependencies(ctx, tx, source, target, &preview.Counts)
	if err != nil {
		return nil, err
	}

	if err := fillTeaMergePlan(ctx, tx, preview); err != nil {
		return nil, err
	}

	preview.Token, err = operationHash(struct {
		Source, Target common.TeaMergeSnapshot
		Dependencies   []string
		Cover          *uuid.UUID
	}{preview.Source, preview.Target, dependencies, preview.ResultingCoverImageID})

	return preview, err
}

func readTeaMergeSnapshot(ctx context.Context, tx *sql.Tx, tea *common.Tea) (common.TeaMergeSnapshot, error) {
	result := common.TeaMergeSnapshot{Tea: tea, Tags: make([]common.TeaMergeTag, 0), Images: make([]common.TeaImage, 0)}

	rows, err := tx.QueryContext(ctx, `SELECT t.id,t.name,t.color,t.category_id,t.revision::text,c.name,c.revision::text FROM tags t JOIN tag_categories c ON c.id=t.category_id JOIN tea_tags tt ON tt.tag_id=t.id WHERE tt.tea_id=$1 ORDER BY t.id`, tea.ID)
	if err != nil {
		return result, wrapCatalogStorageError(err)
	}
	defer rows.Close()

	for rows.Next() {
		tag := &common.Tag{TagData: &common.TagData{}}

		category := &common.TagCategory{}
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CategoryID, &tag.Revision, &category.Name, &category.Revision); err != nil {
			return result, wrapCatalogStorageError(err)
		}

		category.ID = tag.CategoryID
		result.Tags = append(result.Tags, common.TeaMergeTag{Tag: tag, Category: category})
	}

	if err := rows.Err(); err != nil {
		return result, wrapCatalogStorageError(err)
	}

	_ = rows.Close()

	images, err := tx.QueryContext(ctx, `SELECT `+teaImageColumns+` FROM tea_images WHERE tea_id=$1 ORDER BY created_at,id`, tea.ID)
	if err != nil {
		return result, wrapCatalogStorageError(err)
	}
	defer images.Close()

	for images.Next() {
		var image common.TeaImage
		if err := images.Scan(&image.ID, &image.TeaID, &image.ContentType, &image.Width, &image.Height, &image.CreatedAt, &image.IsCover); err != nil {
			return result, wrapCatalogStorageError(err)
		}

		result.Images = append(result.Images, image)
	}

	return result, wrapCatalogStorageError(images.Err())
}

// Dependency contents are hashed only; API responses return aggregate transfer counts.
func readTeaMergeDependencies(ctx context.Context, tx *sql.Tx, source, target uuid.UUID, counts *common.TeaMergeCounts) ([]string, error) {
	definitions := []struct {
		query string
		count *int
	}{
		{`SELECT COALESCE(jsonb_agg(jsonb_build_array(id,tea_id,boiling_temp,expiration_date,created_at) ORDER BY id),'[]'::jsonb)::text,count(*) FILTER(WHERE tea_id=$1) FROM qr_records WHERE tea_id IN($1,$2)`, &counts.QRCount},
		{`SELECT COALESCE(jsonb_agg(jsonb_build_array(c.collection_id,c.qr_id) ORDER BY c.collection_id,c.qr_id),'[]'::jsonb)::text,count(*) FILTER(WHERE q.tea_id=$1) FROM collection_qr_items c JOIN qr_records q ON q.id=c.qr_id WHERE q.tea_id IN($1,$2)`, &counts.CollectionMembershipCount},
		{`SELECT COALESCE(jsonb_agg(jsonb_build_array(user_id,ts,tea_id) ORDER BY user_id,ts,tea_id),'[]'::jsonb)::text,count(*) FILTER(WHERE tea_id=$1) FROM consumptions WHERE tea_id IN($1,$2)`, &counts.ConsumptionsMoved},
	}

	result := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		var value string
		if err := tx.QueryRowContext(ctx, definition.query, source, target).Scan(&value, definition.count); err != nil {
			return nil, fmt.Errorf("read Tea merge dependencies: %w", err)
		}

		result = append(result, value)
	}

	return result, nil
}

func chooseTeaMergeCover(preview *common.TeaMergePreview) *uuid.UUID {
	for _, gallery := range [][]common.TeaImage{preview.Target.Images, preview.Source.Images} {
		for _, image := range gallery {
			if image.IsCover {
				id := image.ID
				return &id
			}
		}
	}

	return oldestTeaMergeImage(preview.Target.Images, preview.Source.Images)
}

func oldestTeaMergeImage(target, source []common.TeaImage) *uuid.UUID {
	var oldest *common.TeaImage

	for _, gallery := range [][]common.TeaImage{target, source} {
		for _, image := range gallery {
			if oldest == nil || image.CreatedAt.Before(oldest.CreatedAt) || (image.CreatedAt.Equal(oldest.CreatedAt) && image.ID.String() < oldest.ID.String()) {
				value := image
				oldest = &value
			}
		}
	}

	if oldest == nil {
		return nil
	}

	return &oldest.ID
}

func readTeaMergePair(ctx context.Context, tx *sql.Tx, source, target uuid.UUID) (map[uuid.UUID]*common.Tea, error) {
	// All dependent write tables are locked before parent rows to avoid FK cycles.
	rows, err := tx.QueryContext(ctx, `SELECT `+teaColumns+` FROM teas WHERE id IN ($1,$2) ORDER BY id FOR UPDATE`, source, target)
	if err != nil {
		return nil, wrapCatalogStorageError(err)
	}
	defer rows.Close()

	teas := make(map[uuid.UUID]*common.Tea)

	for rows.Next() {
		item, err := scanCatalogueTea(rows)
		if err != nil {
			return nil, err
		}

		teas[item.ID] = item
	}

	if err := rows.Err(); err != nil {
		return nil, wrapCatalogStorageError(err)
	}

	_ = rows.Close()

	if len(teas) != 2 {
		return nil, common.ErrTeaNotFound
	}

	return teas, nil
}

func fillTeaMergePlan(ctx context.Context, tx *sql.Tx, preview *common.TeaMergePreview) error {
	preview.ResultingCoverImageID = chooseTeaMergeCover(preview)
	preview.Counts.ImagesMoved = len(preview.Source.Images)
	preview.ResultingImageCount = len(preview.Source.Images) + len(preview.Target.Images)

	assigned := make(map[uuid.UUID]bool)
	for _, tag := range preview.Target.Tags {
		assigned[tag.Tag.ID] = true
	}

	for _, tag := range preview.Source.Tags {
		if !assigned[tag.Tag.ID] {
			preview.Counts.TagsAdded++
		}
	}

	if preview.ResultingImageCount > common.MaxTeaImages {
		preview.Blockers = append(preview.Blockers, common.TeaMergeImageLimit)
	}

	var collision bool

	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM consumptions s JOIN consumptions t ON s.user_id=t.user_id AND s.ts=t.ts WHERE s.tea_id=$1 AND t.tea_id=$2)`, preview.Source.Tea.ID, preview.Target.Tea.ID).Scan(&collision)
	if err != nil {
		return wrapCatalogStorageError(err)
	}

	if collision {
		preview.Blockers = append(preview.Blockers, common.TeaMergeConsumptionCollision)
	}

	return nil
}
