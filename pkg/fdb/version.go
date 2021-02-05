package fdb

import (
	"encoding/binary"
)

type version interface {
	GetVersion() (uint32, error)
	WriteVersion(version uint32) error
}

func (d *db) WriteVersion(version uint32) error {
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return err
	}
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, version)
	tr.Set(d.keyBuilder.Version(), data)
	return tr.Commit().Get()
}

func (d *db) GetVersion() (uint32, error) {
	tr, err := d.fdb.CreateTransaction()
	if err != nil {
		return 0, err
	}
	data, err := tr.Get(d.keyBuilder.Version()).Get()
	if err != nil {
		return 0, err
	}
	if data == nil {
		return 0, nil
	}
	return binary.BigEndian.Uint32(data), nil
}
