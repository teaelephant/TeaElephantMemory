package db

import "github.com/lueurxax/teaelephantmemory/pkg/db/common"

func (D *DB) ReadAll() ([]common.KeyValue, error) {
	res := make([]common.KeyValue, 0)
	iter := D.db.NewIterator(nil, nil)
	for iter.Next() {
		res = append(res, common.KeyValue{
			Key:   iter.Key(),
			Value: iter.Value(),
		})
	}
	iter.Release()
	if iter.Error() != nil {
		return nil, iter.Error()
	}
	return res, nil
}
