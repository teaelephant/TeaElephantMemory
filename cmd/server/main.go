package main

import (
	"context"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kelseyhightower/envconfig"
	"github.com/sashabaranov/go-openai"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
	"github.com/sirupsen/logrus"

	gql "github.com/99designs/gqlgen/graphql"

	"github.com/teaelephant/TeaElephantMemory/internal/adviser"
	"github.com/teaelephant/TeaElephantMemory/internal/apns"
	"github.com/teaelephant/TeaElephantMemory/internal/auth"
	"github.com/teaelephant/TeaElephantMemory/internal/consumption"
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
	pgadapter "github.com/teaelephant/TeaElephantMemory/pkg/pg"
)

const (
	pkgKey = "pkg"
)

type configuration struct {
	LoggerLevel logrus.Level `envconfig:"LOG_LEVEL" default:"info"`
	OpenAIToken string       `envconfig:"OPEN_AI_TOKEN" require:"true"`
	PGDSN       string       `envconfig:"PG_DSN" default:""`
}

//nolint:funlen // main wires dependencies; keep it in one place for clarity despite statement count
func main() {
	cfg := new(configuration)
	if err := envconfig.Process("", cfg); err != nil {
		panic(err)
	}

	logrusLogger := logrus.New()
	logrusLogger.SetLevel(cfg.LoggerLevel)

	// Postgres is required. Fail fast if PG_DSN is not provided.
	if cfg.PGDSN == "" {
		panic("PG_DSN is required")
	}

	psql, err := sql.Open("pgx", cfg.PGDSN)
	if err != nil {
		panic(err)
	}

	if err := psql.PingContext(context.Background()); err != nil {
		panic(err)
	}

	st := pgadapter.NewDB(psql, logrusLogger.WithField(pkgKey, "pg"))

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

	// Consumption history uses the same Postgres connection
	cons := consumption.NewPGStore(psql, 0)

	resolvers := graphql.NewResolver(
		logrusLogger.WithField(pkgKey, "graphql"),
		teaManager, qrManager, tagManager, collectionManager, authM, ai, notificationManager, expirationAlerter,
		adv, weather, cons,
	)

	s := server.NewServer(resolvers, []gql.HandlerExtension{authM.Middleware()}, authM.WsInitFunc)
	s.InitV2Api()
	teaManager.Start()
	tagManager.Start()

	if err = s.Run(); err != nil {
		panic(err)
	}
}
