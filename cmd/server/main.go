package main

import (
	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/kelseyhightower/envconfig"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
	"github.com/sirupsen/logrus"

	gql "github.com/99designs/gqlgen/graphql"

	"github.com/teaelephant/TeaElephantMemory/internal/apns"
	"github.com/teaelephant/TeaElephantMemory/internal/auth"
	"github.com/teaelephant/TeaElephantMemory/internal/descrgen"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/collection"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/notification"
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
	OpenAIToken  string `envconfig:"OPEN_AI_TOKEN" require:"true"`
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

	teaManager := tea.NewManager(st)
	qrManager := qr.NewManager(st)
	tagManager := tag.NewManager(st, teaManager, logrusLogger)
	collectionManager := collection.NewManager(st)

	authCfg := auth.Config()
	authM := auth.NewAuth(authCfg, st, logrusLogger.WithField(pkgKey, "auth"))

	if err = authM.Start(); err != nil {
		panic(err)
	}

	ai := descrgen.NewGenerator(cfg.OpenAIToken, logrusLogger.WithField(pkgKey, "descrgen"))

	notificationManager := notification.NewManager(st)

	authKey, err := token.AuthKeyFromFile(authCfg.SecretPath)
	if err != nil {
		panic(err)
	}

	apnsClient := apns2.NewTokenClient(&token.Token{
		AuthKey: authKey,
		// KeyID from developer account (Certificates, Identifiers & Profiles -> Keys)
		KeyID: authCfg.KeyID,
		// TeamID from developer account (View Account -> Membership)
		TeamID: authCfg.TeamID,
	}).Development()

	apnsSender := apns.NewSender(apnsClient, st, logrusLogger.WithField(pkgKey, "apns"))

	resolvers := graphql.NewResolver(
		logrusLogger.WithField(pkgKey, "graphql"),
		teaManager, qrManager, tagManager, collectionManager, authM, ai, notificationManager, apnsSender,
	)

	s := server.NewServer(resolvers, []gql.HandlerExtension{authM.Middleware()})
	s.InitV2Api()
	teaManager.Start()
	tagManager.Start()

	if err = s.Run(); err != nil {
		panic(err)
	}
}
