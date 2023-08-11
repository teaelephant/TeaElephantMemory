package main

import (
	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/internal/auth"
	"github.com/teaelephant/TeaElephantMemory/internal/httperror"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/collection"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/qr"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/tag"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/tea"
	"github.com/teaelephant/TeaElephantMemory/internal/server"
	v1 "github.com/teaelephant/TeaElephantMemory/pkg/api/v1"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdb"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/migrations"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb/migrator"
)

const (
	foundationDBVersion = 710
	pkgKey              = "pkg"
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
	foundeationDB.MustAPIVersion(foundationDBVersion)

	db, err := foundeationDB.OpenDatabase("/usr/local/etc/foundationdb/fdb.cluster")
	if err != nil {
		panic(err)
	}

	st := fdb.NewDB(db)

	mig := migrator.NewManager(migrations.Migrations, st)

	if err := mig.Migrate(); err != nil {
		panic(err)
	}

	errorCreator := httperror.NewCreator(logrus.WithField(pkgKey, "http_error"))
	tr := server.NewTransport()
	apiV1 := v1.New(st, errorCreator, tr)

	teaManager := tea.NewManager(st)
	qrManager := qr.NewManager(st)
	tagManager := tag.NewManager(st, teaManager, logrusLogger)
	collectionManager := collection.NewManager(st)
	authM := auth.NewAuth()

	resolvers := graphql.NewResolver(logrusLogger.WithField(pkgKey, "graphql"), teaManager, qrManager, tagManager, collectionManager, authM)

	s := server.NewServer(apiV1, resolvers)
	s.InitV1Api()
	s.InitV2Api()
	teaManager.Start()
	tagManager.Start()

	if err := s.Run(); err != nil {
		panic(err)
	}
}
