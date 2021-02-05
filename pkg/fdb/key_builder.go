package fdb

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common/key_value/key_builder"
)

type KeyBuilder interface {
	Version() fdb.Key
	Records() fdb.Key
	Record(id uuid.UUID) fdb.Key
	RecordsByName(name string) fdb.Key
	QR(id uuid.UUID) fdb.Key
	TagCategories() fdb.Key
	TagCategory(id uuid.UUID) fdb.Key
	TagCategoryByName(name string) fdb.Key
	Tags() fdb.Key
	Tag(id uuid.UUID) fdb.Key
	TagsByName(name string) fdb.Key
	TagsByNameAndCategory(category uuid.UUID, name string) fdb.Key
	TagsByCategory(category uuid.UUID) fdb.Key
	TagsByTea(tea uuid.UUID) fdb.Key
	TeasByTag(tag uuid.UUID) fdb.Key
	TagTeaPair(tag, tea uuid.UUID) fdb.Key
	TeaTagPair(tea, tag uuid.UUID) fdb.Key
}

type keyBuilder struct {
	inner key_builder.Builder
}

func (k *keyBuilder) TagTeaPair(tag, tea uuid.UUID) fdb.Key {
	return k.inner.TagTeaPair(tag, tea)
}

func (k *keyBuilder) TeaTagPair(tea, tag uuid.UUID) fdb.Key {
	return k.inner.TeaTagPair(tea, tag)
}

func (k *keyBuilder) TagsByTea(tea uuid.UUID) fdb.Key {
	return k.inner.TagsByTea(tea)
}

func (k *keyBuilder) TeasByTag(tag uuid.UUID) fdb.Key {
	return k.inner.TeasByTag(tag)
}

func (k *keyBuilder) TagsByCategory(category uuid.UUID) fdb.Key {
	return k.inner.TagsByCategory(category)
}

func (k *keyBuilder) Version() fdb.Key {
	return k.inner.Version()
}

func (k *keyBuilder) Records() fdb.Key {
	return k.inner.Records()
}

func (k *keyBuilder) Record(id uuid.UUID) fdb.Key {
	return k.inner.Record(id)
}

func (k *keyBuilder) RecordsByName(name string) fdb.Key {
	return k.inner.RecordsByName(name)
}

func (k *keyBuilder) QR(id uuid.UUID) fdb.Key {
	return k.inner.QR(id)
}

func (k *keyBuilder) TagCategories() fdb.Key {
	return k.inner.TagCategories()
}

func (k *keyBuilder) TagCategory(id uuid.UUID) fdb.Key {
	return k.inner.TagCategory(id)
}

func (k *keyBuilder) TagCategoryByName(name string) fdb.Key {
	return k.inner.TagCategoryByName(name)
}

func (k *keyBuilder) Tags() fdb.Key {
	return k.inner.Tags()
}

func (k *keyBuilder) Tag(id uuid.UUID) fdb.Key {
	return k.inner.Tag(id)
}

func (k *keyBuilder) TagsByName(name string) fdb.Key {
	return k.inner.TagsByName(name)
}

func (k *keyBuilder) TagsByNameAndCategory(category uuid.UUID, name string) fdb.Key {
	return k.inner.TagsByNameAndCategory(category, name)
}

func NewKeyBuilder(inner key_builder.Builder) KeyBuilder {
	return &keyBuilder{inner: inner}
}
