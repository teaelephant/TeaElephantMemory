package leveldb

import (
	"encoding/binary"
)

func (l *levelStorage) WriteVersion(version uint32) error {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, version)
	return l.db.Put(l.keyBuilder.Version(), data, nil)
}

func (l *levelStorage) GetVersion() (uint32, error) {
	data, err := l.db.Get(l.keyBuilder.Version(), nil)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(data), nil
}
