package migrations

import (
	"github.com/teaelephant/TeaElephantMemory/common"
	dbCommon "github.com/teaelephant/TeaElephantMemory/pkg/leveldb/common"
)

type MigratingDB interface {
	ReadAll() ([]dbCommon.KeyValue, error)
	Update(id string, rec *common.TeaData) (record *common.Tea, err error)
}

type Migration interface {
	Migrate(db MigratingDB) error
}

var Migrations = map[uint32]Migration{
	0: &addPrefixes{},
}
