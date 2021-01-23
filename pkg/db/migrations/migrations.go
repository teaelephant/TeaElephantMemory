package migrations

import (
	"github.com/lueurxax/teaelephantmemory/common"
	dbCommon "github.com/lueurxax/teaelephantmemory/pkg/db/common"
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
