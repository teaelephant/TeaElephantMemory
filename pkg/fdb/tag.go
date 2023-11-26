package fdb

import (
	"context"

	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
	fdbCommon "github.com/teaelephant/TeaElephantMemory/pkg/fdb/common"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdbclient"
)

type tag interface {
	CreateTagCategory(ctx context.Context, name string) (category *common.TagCategory, err error)
	UpdateTagCategory(ctx context.Context, id uuid.UUID, name string) error
	DeleteTagCategory(ctx context.Context, id uuid.UUID) (removedTags []uuid.UUID, err error)
	GetTagCategory(ctx context.Context, id uuid.UUID) (category *common.TagCategory, err error)
	ListTagCategories(ctx context.Context, search *string) (list []common.TagCategory, err error)
	CreateTag(ctx context.Context, name, color string, categoryID uuid.UUID) (*common.Tag, error)
	UpdateTag(ctx context.Context, id uuid.UUID, name, color string) (*common.Tag, error)
	ChangeTagCategory(ctx context.Context, id, categoryID uuid.UUID) (*common.Tag, error)
	DeleteTag(ctx context.Context, id uuid.UUID) error
	GetTag(ctx context.Context, id uuid.UUID) (*common.Tag, error)
	ListTags(ctx context.Context, name *string, categoryID *uuid.UUID) (list []common.Tag, err error)
	AddTagToTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error
	DeleteTagFromTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error
	ListByTea(ctx context.Context, id uuid.UUID) ([]common.Tag, error)
}

func (d *db) CreateTagCategory(ctx context.Context, name string) (category *common.TagCategory, err error) {
	id := uuid.New()
	if err = d.writeCategory(ctx, id, name); err != nil {
		return nil, err
	}

	return &common.TagCategory{
		ID:   id,
		Name: name,
	}, nil
}

func (d *db) UpdateTagCategory(ctx context.Context, id uuid.UUID, name string) error {
	return d.writeCategory(ctx, id, name)
}

func (d *db) DeleteTagCategory(ctx context.Context, id uuid.UUID) (removedTags []uuid.UUID, err error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	cat, err := d.readCategory(id, tr)
	if err != nil {
		return nil, err
	}

	tags, err := d.readTagsByCategoryID(tr, id)
	if err != nil {
		return nil, err
	}

	removedTags = make([]uuid.UUID, len(tags))

	for i, t := range tags {
		d.deleteTag(tr, id, t.TagData)
		removedTags[i] = t.ID
	}

	tr.Clear(d.keyBuilder.TagCategory(id))
	tr.Clear(d.keyBuilder.TagCategoryByName(cat.Name))

	if err = tr.Commit(); err != nil {
		return nil, err
	}

	return removedTags, nil
}

func (d *db) GetTagCategory(ctx context.Context, id uuid.UUID) (category *common.TagCategory, err error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	return d.readCategory(id, tr)
}

func (d *db) ListTagCategories(ctx context.Context, search *string) (list []common.TagCategory, err error) {
	records := make([]common.TagCategory, 0)

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	if search == nil || *search == "" {
		pr, err := fdb.PrefixRange(d.keyBuilder.TagCategories())
		if err != nil {
			return nil, err
		}

		kvs, err := tr.GetRange(pr)
		if err != nil {
			return nil, err
		}

		for _, kv := range kvs {
			id, err := uuid.FromBytes(kv.Key[1:])
			if err != nil {
				id = uuid.Nil
			}

			records = append(records, common.TagCategory{
				ID:   id,
				Name: string(kv.Value),
			})
		}

		return records, nil
	}

	pr, err := fdb.PrefixRange(d.keyBuilder.TagCategoryByName(*search))
	if err != nil {
		return nil, err
	}

	kvs, err := tr.GetRange(pr)
	if err != nil {
		return nil, err
	}

	for _, kv := range kvs {
		id := new(uuid.UUID)

		if err = id.UnmarshalBinary(kv.Value); err != nil {
			return nil, err
		}

		rec, err := d.readCategory(*id, tr)
		if err != nil {
			return nil, err
		}

		records = append(records, *rec)
	}

	return records, nil
}

func (d *db) CreateTag(ctx context.Context, name, color string, categoryID uuid.UUID) (*common.Tag, error) {
	id := uuid.New()

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	tags, err := d.readTagsByNameAndCategoryID(tr, name, categoryID)
	if err != nil {
		return nil, err
	}

	for _, t := range tags {
		if t.Name == name {
			return nil, fdbCommon.ErrTagExist{Name: name, CategoryID: categoryID}
		}
	}

	if _, err := d.readCategory(categoryID, tr); err != nil {
		return nil, err
	}

	tag := &common.Tag{
		ID: id,
		TagData: &common.TagData{
			Name:       name,
			Color:      color,
			CategoryID: categoryID,
		},
	}

	if err = d.writeTag(tr, tag); err != nil {
		return nil, err
	}

	if err = tr.Commit(); err != nil {
		return nil, err
	}

	return tag, nil
}

func (d *db) UpdateTag(ctx context.Context, id uuid.UUID, name, color string) (*common.Tag, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	data, err := tr.Get(d.keyBuilder.Tag(id))
	if err != nil {
		return nil, err
	}

	tagData := new(encoder.TagData)
	if err = tagData.Decode(data); err != nil {
		return nil, err
	}

	newTag := &common.Tag{
		ID: id,
		TagData: &common.TagData{
			Name:       name,
			Color:      color,
			CategoryID: tagData.CategoryID,
		},
	}

	if err = d.writeTag(tr, newTag); err != nil {
		return nil, err
	}

	if err = tr.Commit(); err != nil {
		return nil, err
	}

	return newTag, nil
}

func (d *db) ChangeTagCategory(ctx context.Context, id, categoryID uuid.UUID) (*common.Tag, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	data, err := tr.Get(d.keyBuilder.Tag(id))
	if err != nil {
		return nil, err
	}

	tagData := new(encoder.TagData)
	if err = tagData.Decode(data); err != nil {
		return nil, err
	}

	newTag := &common.Tag{
		ID: id,
		TagData: &common.TagData{
			Name:       tagData.Name,
			Color:      tagData.Color,
			CategoryID: categoryID,
		},
	}

	if err = d.writeTag(tr, newTag); err != nil {
		return nil, err
	}

	if err = tr.Commit(); err != nil {
		return nil, err
	}

	return newTag, nil
}

func (d *db) DeleteTag(ctx context.Context, id uuid.UUID) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	tag, err := d.readTag(id, tr)
	if err != nil {
		return err
	}

	d.deleteTag(tr, id, tag)

	return tr.Commit()
}

func (d *db) GetTag(ctx context.Context, id uuid.UUID) (*common.Tag, error) {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	tagData, err := d.readTag(id, tr)
	if err != nil {
		return nil, err
	}

	return &common.Tag{
		ID:      id,
		TagData: tagData,
	}, nil
}

func (d *db) ListTags(ctx context.Context, name *string, categoryID *uuid.UUID) (list []common.Tag, err error) {
	records := make([]common.Tag, 0)

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	hasFilterByName := name != nil && *name != ""
	if !hasFilterByName && categoryID == nil {
		pr, err := fdb.PrefixRange(d.keyBuilder.Tags())
		if err != nil {
			return nil, err
		}

		kvs, err := tr.GetRange(pr)
		if err != nil {
			return nil, err
		}

		for _, kv := range kvs {
			tagData := new(encoder.TagData)
			if err = tagData.Decode(kv.Value); err != nil {
				return nil, err
			}

			id, err := uuid.FromBytes(kv.Key[1:])
			if err != nil {
				id = uuid.Nil
			}

			records = append(records, common.Tag{
				ID:      id,
				TagData: (*common.TagData)(tagData),
			})
		}

		return records, nil
	}

	if categoryID == nil {
		pr, err := fdb.PrefixRange(d.keyBuilder.TagsByName(*name))
		if err != nil {
			return nil, err
		}

		kvs, err := tr.GetRange(pr)
		if err != nil {
			return nil, err
		}

		for _, kv := range kvs {
			id := new(uuid.UUID)
			if err = id.UnmarshalBinary(kv.Value); err != nil {
				return nil, err
			}

			rec, err := d.readTag(*id, tr)
			if err != nil {
				return nil, err
			}

			records = append(records, common.Tag{
				ID:      *id,
				TagData: rec,
			})
		}

		return records, nil
	}

	if !hasFilterByName {
		if records, err = d.readTagsByCategoryID(tr, *categoryID); err != nil {
			return nil, err
		}

		return records, nil
	}

	if records, err = d.readTagsByNameAndCategoryID(tr, *name, *categoryID); err != nil {
		return nil, err
	}

	return records, nil
}

func (d *db) AddTagToTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	if _, err = d.readRecord(tea, tr); err != nil {
		return err
	}

	if _, err = d.readTag(tag, tr); err != nil {
		return err
	}

	tr.Set(d.keyBuilder.TeaTagPair(tea, tag), tag[:])

	tr.Set(d.keyBuilder.TagTeaPair(tag, tea), tea[:])

	return tr.Commit()
}

func (d *db) DeleteTagFromTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error {
	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	if _, err = d.readRecord(tea, tr); err != nil {
		return err
	}

	if _, err = d.readTag(tag, tr); err != nil {
		return err
	}

	tr.Clear(d.keyBuilder.TeaTagPair(tea, tag))
	tr.Clear(d.keyBuilder.TagTeaPair(tag, tea))

	return tr.Commit()
}

func (d *db) ListByTea(ctx context.Context, id uuid.UUID) ([]common.Tag, error) {
	records := make([]common.Tag, 0)

	tr, err := d.db.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	pr, err := fdb.PrefixRange(d.keyBuilder.TagsByTea(id))
	if err != nil {
		return nil, err
	}

	kvs, err := tr.GetRange(pr)
	if err != nil {
		return nil, err
	}

	for _, kv := range kvs {
		id := new(uuid.UUID)
		if err = id.UnmarshalBinary(kv.Value); err != nil {
			return nil, err
		}

		rec, err := d.readTag(*id, tr)
		if err != nil {
			return nil, err
		}

		records = append(records, common.Tag{
			ID:      *id,
			TagData: rec,
		})
	}

	return records, nil
}

func (d *db) writeCategory(ctx context.Context, id uuid.UUID, name string) error {
	tx, err := d.db.NewTransaction(ctx)
	if err != nil {
		return err
	}

	tx.Set(d.keyBuilder.TagCategory(id), []byte(name))

	tx.Set(d.keyBuilder.TagCategoryByName(name), id[:])

	return tx.Commit()
}

func (d *db) writeTag(tr fdbclient.Transaction, tag *common.Tag) error {
	data, err := (*encoder.TagData)(tag.TagData).Encode()
	if err != nil {
		return err
	}

	tr.Set(d.keyBuilder.Tag(tag.ID), data)

	tr.Set(d.keyBuilder.TagsByName(tag.Name), tag.ID[:])

	tr.Set(d.keyBuilder.TagsByNameAndCategory(tag.CategoryID, tag.Name), tag.ID[:])

	return nil
}

func (d *db) readTag(id uuid.UUID, tr fdbclient.Transaction) (*common.TagData, error) {
	data, err := tr.Get(d.keyBuilder.Tag(id))
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, fdbCommon.ErrNotFound{
			Type: "tag",
			ID:   id.String(),
		}
	}

	tagData := new(encoder.TagData)
	if err = tagData.Decode(data); err != nil {
		return nil, err
	}

	return (*common.TagData)(tagData), nil
}

func (d *db) readCategory(id uuid.UUID, tr fdbclient.Transaction) (*common.TagCategory, error) {
	data, err := tr.Get(d.keyBuilder.TagCategory(id))
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, fdbCommon.ErrNotFound{
			Type: "category",
			ID:   id.String(),
		}
	}

	return &common.TagCategory{
		ID:   id,
		Name: string(data),
	}, nil
}

func (d *db) readTagsByNameAndCategoryID(tr fdbclient.Transaction, name string, categoryID uuid.UUID) ([]common.Tag, error) {
	return d.readTagsByPrefix(tr, d.keyBuilder.TagsByNameAndCategory(categoryID, name))
}

func (d *db) readTagsByCategoryID(tr fdbclient.Transaction, categoryID uuid.UUID) ([]common.Tag, error) {
	return d.readTagsByPrefix(tr, d.keyBuilder.TagsByCategory(categoryID))
}

func (d *db) readTagsByPrefix(tr fdbclient.Transaction, prefix []byte) ([]common.Tag, error) {
	records := make([]common.Tag, 0)

	pr, err := fdb.PrefixRange(prefix)
	if err != nil {
		return nil, err
	}

	kvs, err := tr.GetRange(pr)
	if err != nil {
		return nil, err
	}

	for _, kv := range kvs {
		id := new(uuid.UUID)
		if err = id.UnmarshalBinary(kv.Value); err != nil {
			return nil, err
		}

		rec, err := d.readTag(*id, tr)
		if err != nil {
			return nil, err
		}

		records = append(records, common.Tag{
			ID:      *id,
			TagData: rec,
		})
	}

	return records, nil
}

func (d *db) deleteTag(tr fdbclient.Transaction, id uuid.UUID, data *common.TagData) {
	tr.Clear(d.keyBuilder.Tag(id))
	tr.Clear(d.keyBuilder.TagsByName(data.Name))
	tr.Clear(d.keyBuilder.TagsByNameAndCategory(data.CategoryID, data.Name))
}
