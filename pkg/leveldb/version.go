package leveldb

import (
	"encoding/binary"

	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/prefix"
)

func (D *levelStorage) WriteVersion(version uint32) error {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, version)
	return D.db.Put([]byte{prefix.Version}, data, nil)
}

func (D *levelStorage) GetVersion() (uint32, error) {
	data, err := D.db.Get([]byte{prefix.Version}, nil)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(data), nil
}
