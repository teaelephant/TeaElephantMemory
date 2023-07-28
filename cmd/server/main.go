package main

import (
	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/internal/httperror"
	"github.com/teaelephant/TeaElephantMemory/internal/qr_manager"
	"github.com/teaelephant/TeaElephantMemory/internal/server"
	"github.com/teaelephant/TeaElephantMemory/internal/tag_manager"
	"github.com/teaelephant/TeaElephantMemory/internal/tea_manager"
	v1 "github.com/teaelephant/TeaElephantMemory/pkg/api/v1"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdb"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/migrations"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/migrator"
)

type configuration struct {
	LogLevel uint32 `default:"4"`
}

func main() {
	cfg := new(configuration)
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}
	logrusLogger := logrus.New()
	logrusLogger.SetLevel(logrus.Level(cfg.LogLevel))
	foundeationDB.MustAPIVersion(710)
	db, err := foundeationDB.OpenDatabase("/usr/local/etc/foundationdb/fdb.cluster")
	if err != nil {
		panic(err)
	}
	st := fdb.NewDb(db)
	mig := migrator.NewManager(migrations.Migrations, st)
	if err := mig.Migrate(); err != nil {
		panic(err)
	}

	errorCreator := httperror.NewCreator(logrus.WithField("pkg", "http_error"))
	tr := server.NewTransport()
	apiV1 := v1.New(st, errorCreator, tr)

	teaManager := tea_manager.NewManager(st)
	qrManager := qr_manager.NewManager(st)
	tagManager := tag_manager.NewManager(st, teaManager, logrusLogger)

	resolvers := graphql.NewResolver(logrusLogger.WithField("pkg", "graphql"), teaManager, qrManager, tagManager)

	s := server.NewServer(apiV1, resolvers)
	s.InitV1Api()
	s.InitV2Api()
	teaManager.Start()
	tagManager.Start()

	if err := s.Run(); err != nil {
		panic(err)
	}
}
