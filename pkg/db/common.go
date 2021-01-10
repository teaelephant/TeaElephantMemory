package db

import "github.com/lueurxax/teaelephantmemory/pkg/db/common"

func (D *DB) ReadAll() ([]common.KeyValue, error) {
	res := make([]common.KeyValue, 0)
	iter := D.db.NewIterator(nil, nil)
	for iter.Next() {
		key := make([]byte, len(iter.Key()))
		value := make([]byte, len(iter.Value()))
		copy(key, iter.Key())
		copy(value, iter.Value())
		res = append(res, common.KeyValue{
			Key:   key,
			Value: value,
		})
	}
	iter.Release()
	if iter.Error() != nil {
		return nil, iter.Error()
	}
	return res, nil
}
