package migrator

import (
	"context"
	"errors"

	"github.com/syndtr/goleveldb/leveldb"

	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/migrations"
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
	ver, err := m.db.GetVersion(context.TODO())

	if err != nil && !errors.Is(err, leveldb.ErrNotFound) {
		return err
	}

	for i := ver; i < currentVersion; i++ {
		if err = m.migrations[i].Migrate(m.db); err != nil {
			return err
		}
	}

	return m.db.WriteVersion(context.TODO(), currentVersion)
}

func NewManager(migrations map[uint32]migrations.Migration, db db) Manager {
	return &manager{migrations: migrations, db: db}
}
