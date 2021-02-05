package migrations

import (
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	dbCommon "github.com/teaelephant/TeaElephantMemory/pkg/leveldb/common"
)

type MigratingDB interface {
	ReadAll() ([]dbCommon.KeyValue, error)
	Update(id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error)
}

type Migration interface {
	Migrate(db MigratingDB) error
}

var Migrations = map[uint32]Migration{
	0: &addPrefixes{},
}
