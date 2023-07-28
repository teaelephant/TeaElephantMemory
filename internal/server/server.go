package server

import (
	"net/http"
	"time"

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

	"github.com/teaelephant/TeaElephantMemory/pkg/api/v1"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql/generated"
)

type Server struct {
	apiV1     *v1.RecordManager
	resolvers generated.ResolverRoot
	router    *mux.Router
}

func (s *Server) Run() error {
	http.Handle("/", s.router)
	originsOk := handlers.AllowedOrigins([]string{"*"})
	logrus.Info("server start on port 8080")
	if err := http.ListenAndServe(":8080", handlers.CORS(originsOk)(s.router)); err != nil {
		logrus.WithError(err).Panic("server httperror")
	}
	return nil
}

func (s *Server) InitV1Api() {
	s.router.HandleFunc("/v1/new_record", s.apiV1.NewRecord).Methods("POST")
	s.router.HandleFunc("/v1/all", s.apiV1.ReadAllRecords).Methods("GET")
	s.router.HandleFunc("/v1/{id}", s.apiV1.ReadRecord).Methods("GET")
	s.router.HandleFunc("/v1/{id}", s.apiV1.UpdateRecord).Methods("POST")
	s.router.HandleFunc("/v1/{id}", s.apiV1.DeleteRecord).Methods("DELETE")
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
	// srv.Use(extension.FixedComplexityLimit(100))

	s.router.Handle("/v2/", playground.Handler("GraphQL playground", "/v2/query"))
	s.router.Handle("/v2/query", srv)
}

func NewServer(apiV1 *v1.RecordManager, resolvers generated.ResolverRoot) *Server {
	return &Server{apiV1: apiV1, resolvers: resolvers, router: mux.NewRouter()}
}
