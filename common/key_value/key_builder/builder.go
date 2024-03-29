package key_builder

import (
	"github.com/google/uuid"
)

type Builder interface {
	Version() []byte
	Records() []byte
	Record(id uuid.UUID) []byte
	RecordsByName(name string) []byte
	QR(id uuid.UUID) []byte
	TagCategories() []byte
	TagCategory(id uuid.UUID) []byte
	TagCategoryByName(name string) []byte
	Tags() []byte
	Tag(id uuid.UUID) []byte
	TagsByName(name string) []byte
	TagsByNameAndCategory(category uuid.UUID, name string) []byte
	TagsByCategory(category uuid.UUID) []byte
	TagsByTea(tea uuid.UUID) []byte
	TeasByTag(tag uuid.UUID) []byte
	TagTeaPair(tag, tea uuid.UUID) []byte
	TeaTagPair(tea, tag uuid.UUID) []byte
	Collection(id, userID uuid.UUID) []byte
	UserCollections(id uuid.UUID) []byte
	CollectionsTeas(id, teaID uuid.UUID) []byte
	RecordsByCollection(id uuid.UUID) []byte
	Users() []byte
	User(id uuid.UUID) []byte
	UserByAppleID(id string) []byte
	Device(id uuid.UUID) []byte
	DevicesByUserID(id uuid.UUID) []byte
	Notification(id uuid.UUID) []byte
	NotificationByUserID(id uuid.UUID) []byte
}

type builder struct {
}

func (b *builder) Users() []byte {
	return []byte{user}
}

func (b *builder) Device(id uuid.UUID) []byte {
	return appendUUID(device, id)
}

func (b *builder) DevicesByUserID(id uuid.UUID) []byte {
	return appendIndex(userIndexDevices, id[:])
}

func (b *builder) Notification(id uuid.UUID) []byte {
	return appendUUID(notification, id)
}

func (b *builder) NotificationByUserID(id uuid.UUID) []byte {
	return appendIndex(userIndexNotifications, id[:])
}

func (b *builder) UserByAppleID(id string) []byte {
	return appendIndex(userIndexAppleID, []byte(id))
}

func (b *builder) User(id uuid.UUID) []byte {
	return appendUUID(user, id)
}

func (b *builder) RecordsByCollection(id uuid.UUID) []byte {
	return appendIndex(collectionIndexTea, id[:])
}

func (b *builder) Collection(id, userID uuid.UUID) []byte {
	return appendIndex(appendUUID(collection, userID), id[:])
}

func (b *builder) UserCollections(id uuid.UUID) []byte {
	return appendUUID(collection, id)
}

func (b *builder) CollectionsTeas(id, teaID uuid.UUID) []byte {
	return appendIndex(collectionIndexTea, append(id[:], teaID[:]...))
}

func (b *builder) TagTeaPair(tag, tea uuid.UUID) []byte {
	return appendIndex(tagIndexTea, append(tag[:], tea[:]...))
}

func (b *builder) TeaTagPair(tea, tag uuid.UUID) []byte {
	return appendIndex(teaIndexTag, append(tea[:], tag[:]...))
}

func (b *builder) TagsByTea(tea uuid.UUID) []byte {
	return appendIndex(teaIndexTag, tea[:])
}

func (b *builder) TeasByTag(tag uuid.UUID) []byte {
	return appendIndex(tagIndexTea, tag[:])
}

func (b *builder) Version() []byte {
	return []byte{version}
}

func (b *builder) Records() []byte {
	return []byte{record}
}

func (b *builder) Record(id uuid.UUID) []byte {
	return appendUUID(record, id)
}

func (b *builder) RecordsByName(name string) []byte {
	return appendPrefix(recordNameIndex, []byte(name))
}

func (b *builder) QR(id uuid.UUID) []byte {
	return appendUUID(qr, id)
}

func (b *builder) TagCategories() []byte {
	return []byte{tagCategory}
}

func (b *builder) TagCategory(id uuid.UUID) []byte {
	return appendUUID(tagCategory, id)
}

func (b *builder) TagCategoryByName(name string) []byte {
	return appendIndex(tagCategoryIndexName, []byte(name))
}

func (b *builder) Tags() []byte {
	return []byte{tag}
}

func (b *builder) Tag(id uuid.UUID) []byte {
	return appendUUID(tag, id)
}

func (b *builder) TagsByName(name string) []byte {
	return appendIndex(tagIndexName, []byte(name))
}

func (b *builder) TagsByNameAndCategory(category uuid.UUID, name string) []byte {
	return appendIndex(tagIndexCategoryName, append(category[:], []byte(name)...))
}

func (b *builder) TagsByCategory(category uuid.UUID) []byte {
	return appendIndex(tagIndexCategoryName, category[:])
}

func appendPrefix(prefix byte, data []byte) []byte {
	res := make([]byte, len(data)+1)
	res[0] = prefix

	for i := 1; i < len(data)+1; i++ {
		res[i] = data[i-1]
	}

	return res
}

func appendUUID(prefix byte, uuid2 uuid.UUID) []byte {
	return appendPrefix(prefix, uuid2[:])
}

func appendIndex(prefix []byte, data []byte) []byte {
	return append(prefix, data...)
}

func NewBuilder() Builder {
	return &builder{}
}
