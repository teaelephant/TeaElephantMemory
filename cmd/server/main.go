package main

import (
	"context"
	"database/sql"

	foundeationDB "github.com/apple/foundationdb/bindings/go/src/fdb"
	_ "github.com/lib/pq"
	"github.com/kelseyhightower/envconfig"
	"github.com/sashabaranov/go-openai"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
	"github.com/sirupsen/logrus"

	gql "github.com/99designs/gqlgen/graphql"

	"github.com/teaelephant/TeaElephantMemory/internal/adviser"
	"github.com/teaelephant/TeaElephantMemory/internal/apns"
	"github.com/teaelephant/TeaElephantMemory/internal/auth"
	"github.com/teaelephant/TeaElephantMemory/internal/descrgen"
	"github.com/teaelephant/TeaElephantMemory/internal/expiration"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/collection"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/notification"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/qr"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/tag"
	"github.com/teaelephant/TeaElephantMemory/internal/managers/tea"
	"github.com/teaelephant/TeaElephantMemory/internal/openweather"
	"github.com/teaelephant/TeaElephantMemory/internal/server"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdb"
	"github.com/teaelephant/TeaElephantMemory/pkg/postgres"
)

const (
	foundationDBVersion = 710
	pkgKey              = "pkg"
)

type configuration struct {
	LoggerLevel            logrus.Level `envconfig:"LOG_LEVEL" default:"info"`
	DatabasePath           string       `envconfig:"DATABASEPATH" default:"/usr/local/etc/foundationdb/fdb.cluster"`
	DatabaseType           string       `envconfig:"DATABASE_TYPE" default:"fdb"`
	PostgresConnectionString string     `envconfig:"POSTGRES_CONNECTION_STRING"`
	OpenAIToken            string       `envconfig:"OPEN_AI_TOKEN" require:"true"`
}

func main() {
	cfg := new(configuration)
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}

	logrusLogger := logrus.New()
	logrusLogger.SetLevel(cfg.LoggerLevel)
	var st fdb.DB

	if cfg.DatabaseType == "postgres" {
		// Initialize PostgreSQL
		pgDB, err := sql.Open("postgres", cfg.PostgresConnectionString)
		if err != nil {
			panic(err)
		}

		// Test the connection
		if err = pgDB.Ping(); err != nil {
			panic(err)
		}

		// Initialize schema
		if err = postgres.InitSchema(context.Background(), pgDB); err != nil {
			panic(err)
		}

		st = postgres.NewDB(pgDB, logrusLogger.WithField(pkgKey, "postgres"))
	} else {
		// Initialize FoundationDB
		foundeationDB.MustAPIVersion(foundationDBVersion)

		db, err := foundeationDB.OpenDatabase(cfg.DatabasePath)
		if err != nil {
			panic(err)
		}

		st = fdb.NewDB(db, logrusLogger.WithField(pkgKey, "fdb"))
	}

	teaManager := tea.NewManager(st)
	qrManager := qr.NewManager(st)
	tagManager := tag.NewManager(st, teaManager, logrusLogger)
	collectionManager := collection.NewManager(st)

	authCfg := auth.Config()
	authM := auth.NewAuth(authCfg, st, logrusLogger.WithField(pkgKey, "auth"))

	if err := authM.Start(); err != nil {
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

	apnsSender := apns.NewSender(apnsClient, authCfg.ClientID, st, logrusLogger.WithField(pkgKey, "apns"))

	expirationAlerter := expiration.NewAlerter(apnsSender, st, logrusLogger.WithField(pkgKey, "expirationAlerter"))

	if err = expirationAlerter.Start(); err != nil {
		panic(err)
	}

	weather := openweather.NewService(openweather.Config().ApiKey, logrusLogger.WithField(pkgKey, "openweather"))

	adv := adviser.NewService(openai.NewClient(cfg.OpenAIToken), logrusLogger.WithField(pkgKey, "adviser"))
	if err = adv.LoadPrompt(); err != nil {
		panic(err)
	}

	resolvers := graphql.NewResolver(
		logrusLogger.WithField(pkgKey, "graphql"),
		teaManager, qrManager, tagManager, collectionManager, authM, ai, notificationManager, expirationAlerter,
		adv, weather,
	)

	s := server.NewServer(resolvers, []gql.HandlerExtension{authM.Middleware()}, authM.WsInitFunc)
	s.InitV2Api()
	teaManager.Start()
	tagManager.Start()

	if err = s.Run(); err != nil {
		panic(err)
	}
}
