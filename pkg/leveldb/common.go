package leveldb

import "github.com/teaelephant/TeaElephantMemory/pkg/leveldb/common"

func (l *levelStorage) ReadAll() ([]common.KeyValue, error) {
	res := make([]common.KeyValue, 0)
	iter := l.db.NewIterator(nil, nil)
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
