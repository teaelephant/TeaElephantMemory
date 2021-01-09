package migrator

import (
	"github.com/syndtr/goleveldb/leveldb"

	"github.com/lueurxax/teaelephantmemory/pkg/db/migrations"
)

type Manager interface {
	Migrate() error
}

type db interface {
	migrations.MigratingDB
	versionDB
}

type manager struct {
	migrations map[uint32]migrations.Migration
	db
}

func (m *manager) Migrate() error {
	ver, err := m.db.GetVersion()
	if err != nil && err != leveldb.ErrNotFound {
		return err
	}
	for i := ver; i < currentVersion; i++ {
		if err = m.migrations[i].Migrate(m.db); err != nil {
			return err
		}
	}
	return m.db.WriteVersion(currentVersion)
}

func NewManager(migrations map[uint32]migrations.Migration, db db) Manager {
	return &manager{migrations: migrations, db: db}

}
