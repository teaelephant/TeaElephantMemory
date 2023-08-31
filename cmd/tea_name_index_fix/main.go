package main

import (
	"context"

	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/pkg/fdb"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/migrations"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/migrator"
)

const (
	foundationDBVersion = 710
	pkgKey              = "pkg"
)

type configuration struct {
	LogLevel     uint32 `default:"4"`
	DatabasePath string `default:"/usr/local/etc/foundationdb/fdb.cluster"`
}

func main() {
	cfg := new(configuration)
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}

	logrusLogger := logrus.New()
	logrusLogger.SetLevel(logrus.Level(cfg.LogLevel))
	foundeationDB.MustAPIVersion(foundationDBVersion)

	db, err := foundeationDB.OpenDatabase(cfg.DatabasePath)
	if err != nil {
		panic(err)
	}

	st := fdb.NewDB(db, logrusLogger.WithField(pkgKey, "fdb"))

	mig := migrator.NewManager(migrations.Migrations, st)

	if err := mig.Migrate(); err != nil {
		panic(err)
	}

	ctx := context.Background()

	data, err := st.ReadAllRecords(ctx, "")
	if err != nil {
		panic(err)
	}

	for i, tea := range data {
		el, err := st.ReadAllRecords(ctx, tea.Name)
		if err != nil {
			panic(err)
		}

		logrus.WithField("index", i).WithField("data", el).Info("tea in storage")
	}
}
