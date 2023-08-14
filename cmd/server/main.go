package main

import (
	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/internal/auth"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/collection"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/qr"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/tag"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/tea"
	"github.com/teaelephant/TeaElephantMemory/internal/server"
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

	st := fdb.NewDB(db)

	mig := migrator.NewManager(migrations.Migrations, st)

	if err := mig.Migrate(); err != nil {
		panic(err)
	}

	teaManager := tea.NewManager(st)
	qrManager := qr.NewManager(st)
	tagManager := tag.NewManager(st, teaManager, logrusLogger)
	collectionManager := collection.NewManager(st)

	authCfg := auth.Config()
	authM := auth.NewAuth(authCfg, st, logrusLogger.WithField(pkgKey, "auth"))

	if err = authM.Start(); err != nil {
		panic(err)
	}

	resolvers := graphql.NewResolver(logrusLogger.WithField(pkgKey, "graphql"), teaManager, qrManager, tagManager, collectionManager, authM)

	s := server.NewServer(resolvers, []mux.MiddlewareFunc{authM.Middleware})
	s.InitV2Api()
	teaManager.Start()
	tagManager.Start()

	if err = s.Run(); err != nil {
		panic(err)
	}
}
