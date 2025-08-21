// Package server wires HTTP server, GraphQL transports, routes, and middlewares.
package server

import (
	"fmt"
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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql/generated"
)

const (
	v2QueryPath     = "/v2/query"
	staticIndexPath = "./static/index.html"
)

// Server aggregates the GraphQL resolvers, router, middlewares and ws init logic.
type Server struct {
	resolvers   generated.ResolverRoot
	router      *mux.Router
	middlewares []graphql.HandlerExtension
	wsInitFunc  transport.WebsocketInitFunc
}

// Middleware represents an HTTP middleware function.
type Middleware func(handler http.Handler) http.Handler

// Run starts the HTTP server.
func (s *Server) Run() error {
	http.Handle("/", s.router)

	originsOk := handlers.AllowedOrigins([]string{"*"})

	logrus.Info("server start on port 8080")

	if err := http.ListenAndServe(":8080", handlers.CORS(originsOk)(s.router)); err != nil { //nolint:gosec // dev server without custom timeouts
		return fmt.Errorf("listen and serve: %w", err)
	}

	return nil
}

// InitV2Api configures GraphQL transports, cache, middlewares, and routes.
func (s *Server) InitV2Api() {
	srv := handler.New(
		generated.NewExecutableSchema(
			generated.Config{Resolvers: s.resolvers}))

	srv.AddTransport(&transport.Websocket{
		InitFunc:              s.wsInitFunc,
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool {
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

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(apollotracing.Tracer{})
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	for _, m := range s.middlewares {
		srv.Use(m)
	}
	// srv.Use(extension.FixedComplexityLimit(100))

	s.router.Use()
	s.router.Handle("/v2/", playground.Handler("GraphQL playground", v2QueryPath))
	s.router.Handle(v2QueryPath, srv)
	s.router.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	s.router.Handle("/metrics", promhttp.Handler())
	s.router.HandleFunc("/.well-known/apple-app-site-association", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/apple-app-site-association")
	})
	s.router.HandleFunc("/index.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, staticIndexPath)
	})
	s.router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, staticIndexPath)
	})
}

// NewServer creates the HTTP server wiring resolvers, middlewares, and websocket init.
func NewServer(resolvers generated.ResolverRoot, middlewares []graphql.HandlerExtension, wsInitFunc transport.WebsocketInitFunc) *Server {
	return &Server{resolvers: resolvers, router: mux.NewRouter(), middlewares: middlewares, wsInitFunc: wsInitFunc}
}
