package fdb

import "github.com/teaelephant/TeaElephantMemory/common"

type record interface {
	WriteRecord(rec *common.TeaData) (record *common.Tea, err error)
	ReadRecord(id string) (record *common.Tea, err error)
	ReadAllRecords(search string) ([]common.Tea, error)
	Update(id string, rec *common.TeaData) (record *common.Tea, err error)
	Delete(id string) error
}
