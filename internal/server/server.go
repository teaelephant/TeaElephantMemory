package server

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/lueurxax/teaelephantmemory/internal/httperror"
	"github.com/lueurxax/teaelephantmemory/internal/server/api/v1"
	"github.com/lueurxax/teaelephantmemory/internal/server/api/v2/graphql"
	"github.com/lueurxax/teaelephantmemory/internal/server/api/v2/graphql/generated"
	"github.com/lueurxax/teaelephantmemory/internal/tea_manager"
)

type Server struct {
	db v1.Storage
}

func (s *Server) Run() error {
	errorCreator := httperror.NewCreator(logrus.WithField("pkg", "http_error"))
	tr := NewTransport()
	a := v1.New(s.db, errorCreator, tr)
	r := mux.NewRouter()
	r.HandleFunc("/v1/new_record", a.NewRecord).Methods("POST")
	r.HandleFunc("/v1/all", a.ReadAllRecords).Methods("GET")
	r.HandleFunc("/v1/{id}", a.ReadRecord).Methods("GET")
	r.HandleFunc("/v1/{id}", a.UpdateRecord).Methods("POST")
	r.HandleFunc("/v1/{id}", a.DeleteRecord).Methods("DELETE")

	manager := tea_manager.NewManager(s.db)

	resolvers := graphql.NewResolver(logrus.WithField("pkg", "graphql"), manager)

	srv := handler.NewDefaultServer(
		generated.NewExecutableSchema(
			generated.Config{Resolvers: resolvers}))

	r.Handle("/v2/", playground.Handler("GraphQL playground", "/v2/query"))
	r.Handle("/v2/query", srv)

	http.Handle("/", r)

	manager.Start()
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logrus.WithError(err).Panic("server httperror")
	}
	return nil
}

func NewServer(db v1.Storage) *Server {
	return &Server{db}
}
