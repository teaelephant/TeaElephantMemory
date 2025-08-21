package key_builder

import (
	"encoding/binary"
	"math"
	"time"

	"github.com/google/uuid"
)

// Builder constructs byte-encoded keys for the FoundationDB keyspace.
// It centralizes key layouts to keep them consistent across the codebase.
// The returned slices are suitable for direct use with FDB APIs.
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
	ConsumptionByUserID(id uuid.UUID) []byte
	ConsumptionKey(userID uuid.UUID, ts time.Time, teaID uuid.UUID) []byte
}

type builder struct {
}

func (b *builder) ConsumptionByUserID(id uuid.UUID) []byte {
	return appendIndex(userIndexConsumption, id[:])
}

func (b *builder) ConsumptionKey(userID uuid.UUID, ts time.Time, teaID uuid.UUID) []byte {
	prefix := b.ConsumptionByUserID(userID)
	key := make([]byte, 0, len(prefix)+8+16)
	key = append(key, prefix...)

	var tb [8]byte
	// ts.UnixNano() is expected to be non-negative (post-epoch). Casting to uint64 is safe in this domain.
	//nolint:gosec // G115: application-level timestamps are >= 0; we intentionally encode as uint64 for ordering.
	binary.BigEndian.PutUint64(tb[:], uint64(ts.UnixNano()))

	key = append(key, tb[:]...)
	key = append(key, teaID[:]...)

	return key
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

// internal constants for sizes used in consumption key composition
const (
	uuidSize      = 16
	timestampSize = 8
)

// ParseConsumptionKey parses a consumption key that starts with the provided prefix
// and returns the timestamp and teaID encoded in the key. It returns ok=false if the key
// does not match the prefix or has insufficient length.
func ParseConsumptionKey(prefix, key []byte) (time.Time, uuid.UUID, bool) {
	need := len(prefix) + timestampSize + uuidSize
	if len(key) < need {
		return time.Time{}, uuid.UUID{}, false
	}
	// verify prefix matches
	for i := 0; i < len(prefix); i++ {
		if key[i] != prefix[i] {
			return time.Time{}, uuid.UUID{}, false
		}
	}

	u := binary.BigEndian.Uint64(key[len(prefix) : len(prefix)+timestampSize])
	if u > math.MaxInt64 { // clamp to avoid overflow on int64 cast
		u = math.MaxInt64
	}
	//nolint:gosec // G115: value is clamped to MaxInt64 above; conversion is safe.
	n := int64(u)
	ts := time.Unix(0, n)

	var teaID uuid.UUID
	copy(teaID[:], key[len(prefix)+timestampSize:need])

	return ts, teaID, true
}
