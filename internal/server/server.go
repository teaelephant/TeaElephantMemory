package server

import (
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/apollotracing"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql/generated"
)

type Server struct {
	resolvers   generated.ResolverRoot
	router      *mux.Router
	middlewares []graphql.HandlerExtension
}

type Middleware func(handler http.Handler) http.Handler

func (s *Server) Run() error {
	http.Handle("/", s.router)

	originsOk := handlers.AllowedOrigins([]string{"*"})

	logrus.Info("server start on port 8080")

	if err := http.ListenAndServe(":8080", handlers.CORS(originsOk)(s.router)); err != nil {
		logrus.WithError(err).Panic("server httperror")
	}
	return nil
}

func (s *Server) InitV2Api() {
	srv := handler.New(
		generated.NewExecutableSchema(
			generated.Config{Resolvers: s.resolvers}))

	srv.AddTransport(&transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Check against your desired domains here
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})

	srv.SetQueryCache(lru.New(1000))

	srv.Use(apollotracing.Tracer{})
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New(100),
	})
	for _, m := range s.middlewares {
		srv.Use(m)
	}
	// srv.Use(extension.FixedComplexityLimit(100))

	s.router.Use()
	s.router.Handle("/v2/", playground.Handler("GraphQL playground", "/v2/query"))
	s.router.Handle("/v2/query", srv)
}

func NewServer(resolvers generated.ResolverRoot, middlewares []graphql.HandlerExtension) *Server {
	return &Server{resolvers: resolvers, router: mux.NewRouter(), middlewares: middlewares}
}
