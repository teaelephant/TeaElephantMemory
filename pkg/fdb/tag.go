package fdb

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
	common2 "github.com/teaelephant/TeaElephantMemory/pkg/fdb/common"
)

type tag interface {
	CreateTagCategory(name string) (category *common.TagCategory, err error)
	UpdateTagCategory(id uuid.UUID, name string) error
	DeleteTagCategory(id uuid.UUID) (removedTags []uuid.UUID, err error)
	GetTagCategory(id uuid.UUID) (category *common.TagCategory, err error)
	ListTagCategories(search *string) (list []common.TagCategory, err error)
	CreateTag(name, color string, categoryID uuid.UUID) (*common.Tag, error)
	UpdateTag(id uuid.UUID, name, color string) (*common.Tag, error)
	ChangeTagCategory(id, categoryID uuid.UUID) (*common.Tag, error)
	DeleteTag(id uuid.UUID) error
	GetTag(id uuid.UUID) (*common.Tag, error)
	ListTags(name *string, categoryID *uuid.UUID) (list []common.Tag, err error)
	AddTagToTea(tea uuid.UUID, tag uuid.UUID) error
	DeleteTagFromTea(tea uuid.UUID, tag uuid.UUID) error
	ListByTea(id uuid.UUID) ([]common.Tag, error)
}

func (d *db) CreateTagCategory(name string) (category *common.TagCategory, err error) {
	id := uuid.NewV4()
	if err = d.writeCategory(id, name); err != nil {
		return nil, err
	}
	return &common.TagCategory{
		ID:   id,
		Name: name,
	}, nil
}

func (d *db) UpdateTagCategory(id uuid.UUID, name string) error {
	return d.writeCategory(id, name)
}

func (d *db) DeleteTagCategory(id uuid.UUID) (removedTags []uuid.UUID, err error) {
	tr, err := d.fdb.CreateTransaction()
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
	if err = tr.Commit().Get(); err != nil {
		return nil, err
	}
	return removedTags, nil
}

func (d *db) GetTagCategory(id uuid.UUID) (category *common.TagCategory, err error) {
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return nil, err
	}
	return d.readCategory(id, tr)
}

func (d *db) ListTagCategories(search *string) (list []common.TagCategory, err error) {
	records := make([]common.TagCategory, 0)
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return nil, err
	}
	if search == nil || *search == "" {
		pr, err := fdb.PrefixRange(d.keyBuilder.TagCategories())
		if err != nil {
			return nil, err
		}
		kvs, err := tr.GetRange(pr, fdb.RangeOptions{}).GetSliceWithError()
		if err != nil {
			return nil, err
		}
		for _, kv := range kvs {
			records = append(records, common.TagCategory{
				ID:   uuid.FromBytesOrNil(kv.Key[1:]),
				Name: string(kv.Value),
			})
		}
		return records, nil
	}
	pr, err := fdb.PrefixRange(d.keyBuilder.TagCategoryByName(*search))
	if err != nil {
		return nil, err
	}
	kvs, err := tr.GetRange(pr, fdb.RangeOptions{}).GetSliceWithError()
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

func (d *db) CreateTag(name, color string, categoryID uuid.UUID) (*common.Tag, error) {
	id := uuid.NewV4()
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return nil, err
	}
	tags, err := d.readTagsByNameAndCategoryID(tr, name, categoryID)
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		if t.Name == name {
			return nil, common2.ErrTagExist{Name: name, CategoryID: categoryID}
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
	if err = tr.Commit().Get(); err != nil {
		return nil, err
	}
	return tag, nil
}

func (d *db) UpdateTag(id uuid.UUID, name, color string) (*common.Tag, error) {
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return nil, err
	}
	data, err := tr.Get(d.keyBuilder.Tag(id)).Get()
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
	if err = tr.Commit().Get(); err != nil {
		return nil, err
	}
	return newTag, nil
}

func (d *db) ChangeTagCategory(id, categoryID uuid.UUID) (*common.Tag, error) {
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return nil, err
	}
	data, err := tr.Get(d.keyBuilder.Tag(id)).Get()
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
	if err = tr.Commit().Get(); err != nil {
		return nil, err
	}
	return newTag, nil
}

func (d *db) DeleteTag(id uuid.UUID) error {
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return err
	}
	tag, err := d.readTag(id, tr)
	if err != nil {
		return err
	}
	d.deleteTag(tr, id, tag)
	return tr.Commit().Get()
}

func (d *db) GetTag(id uuid.UUID) (*common.Tag, error) {
	tr, err := d.fdb.CreateTransaction()
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

func (d *db) ListTags(name *string, categoryID *uuid.UUID) (list []common.Tag, err error) {
	records := make([]common.Tag, 0)
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return nil, err
	}
	hasFilterByName := name != nil && *name != ""
	if !hasFilterByName && categoryID == nil {
		pr, err := fdb.PrefixRange(d.keyBuilder.Tags())
		if err != nil {
			return nil, err
		}
		kvs, err := tr.GetRange(pr, fdb.RangeOptions{}).GetSliceWithError()
		if err != nil {
			return nil, err
		}
		for _, kv := range kvs {
			tagData := new(encoder.TagData)
			if err = tagData.Decode(kv.Value); err != nil {
				return nil, err
			}
			records = append(records, common.Tag{
				ID:      uuid.FromBytesOrNil(kv.Key[1:]),
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
		kvs, err := tr.GetRange(pr, fdb.RangeOptions{}).GetSliceWithError()
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
	}
	if records, err = d.readTagsByNameAndCategoryID(tr, *name, *categoryID); err != nil {
		return nil, err
	}

	return records, nil
}

func (d *db) AddTagToTea(tea uuid.UUID, tag uuid.UUID) error {
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return err
	}
	if _, err = d.readRecord(tea, tr); err != nil {
		return err
	}
	if _, err = d.readTag(tag, tr); err != nil {
		return err
	}
	tr.Set(d.keyBuilder.TeaTagPair(tea, tag), tag.Bytes())
	tr.Set(d.keyBuilder.TagTeaPair(tag, tea), tea.Bytes())
	return tr.Commit().Get()
}

func (d *db) DeleteTagFromTea(tea uuid.UUID, tag uuid.UUID) error {
	tr, err := d.fdb.CreateTransaction()
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
	return tr.Commit().Get()
}

func (d *db) ListByTea(id uuid.UUID) ([]common.Tag, error) {
	records := make([]common.Tag, 0)
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return nil, err
	}
	pr, err := fdb.PrefixRange(d.keyBuilder.TagsByTea(id))
	if err != nil {
		return nil, err
	}
	kvs, err := tr.GetRange(pr, fdb.RangeOptions{}).GetSliceWithError()
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

func (d *db) writeCategory(id uuid.UUID, name string) error {
	_, err := d.fdb.Transact(func(tr fdb.Transaction) (interface{}, error) {
		tr.Set(d.keyBuilder.TagCategory(id), []byte(name))
		tr.Set(d.keyBuilder.TagCategoryByName(name), id.Bytes())
		return tr.Get(d.keyBuilder.TagCategory(id)).Get()
	})
	return err
}

func (d *db) writeTag(tr fdb.Transaction, tag *common.Tag) error {
	data, err := (*encoder.TagData)(tag.TagData).Encode()
	if err != nil {
		return err
	}
	tr.Set(d.keyBuilder.Tag(tag.ID), data)
	tr.Set(d.keyBuilder.TagsByName(tag.Name), tag.ID.Bytes())
	tr.Set(d.keyBuilder.TagsByNameAndCategory(tag.CategoryID, tag.Name), tag.ID.Bytes())
	return nil
}

func (d *db) readTag(id uuid.UUID, tr fdb.Transaction) (*common.TagData, error) {
	data, err := tr.Get(d.keyBuilder.Tag(id)).Get()
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, common2.ErrNotFound{
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

func (d *db) readCategory(id uuid.UUID, tr fdb.Transaction) (*common.TagCategory, error) {
	data, err := tr.Get(d.keyBuilder.TagCategory(id)).Get()
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, common2.ErrNotFound{
			Type: "category",
			ID:   id.String(),
		}
	}
	return &common.TagCategory{
		ID:   id,
		Name: string(data),
	}, nil
}

func (d *db) readTagsByNameAndCategoryID(tr fdb.Transaction, name string, categoryID uuid.UUID) ([]common.Tag, error) {
	return d.readTagsByPrefix(tr, d.keyBuilder.TagsByNameAndCategory(categoryID, name))
}

func (d *db) readTagsByCategoryID(tr fdb.Transaction, categoryID uuid.UUID) ([]common.Tag, error) {
	return d.readTagsByPrefix(tr, d.keyBuilder.TagsByCategory(categoryID))
}

func (d *db) readTagsByPrefix(tr fdb.Transaction, prefix []byte) ([]common.Tag, error) {
	records := make([]common.Tag, 0)
	pr, err := fdb.PrefixRange(prefix)
	if err != nil {
		return nil, err
	}
	kvs, err := tr.GetRange(pr, fdb.RangeOptions{}).GetSliceWithError()
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

func (d *db) deleteTag(tr fdb.Transaction, id uuid.UUID, data *common.TagData) {
	tr.Clear(d.keyBuilder.Tag(id))
	tr.Clear(d.keyBuilder.TagsByName(data.Name))
	tr.Clear(d.keyBuilder.TagsByNameAndCategory(data.CategoryID, data.Name))
}
