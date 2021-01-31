package server

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/internal/httperror"
	"github.com/teaelephant/TeaElephantMemory/internal/qr_manager"
	"github.com/teaelephant/TeaElephantMemory/internal/server/api/v1"
	"github.com/teaelephant/TeaElephantMemory/internal/server/api/v2/graphql"
	"github.com/teaelephant/TeaElephantMemory/internal/server/api/v2/graphql/generated"
	"github.com/teaelephant/TeaElephantMemory/internal/tea_manager"
	"github.com/teaelephant/TeaElephantMemory/pkg/leveldb"
)

type storage interface {
	WriteQR(id string, data *common.QR) (err error)
	ReadQR(id string) (record *common.QR, err error)
	WriteRecord(rec *common.TeaData) (record *common.Tea, err error)
	ReadRecord(id string) (record *common.Tea, err error)
	ReadAllRecords(search string) ([]common.Tea, error)
	Update(id string, rec *common.TeaData) (record *common.Tea, err error)
	Delete(id string) error
}

type Server struct {
	db storage
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

	teaManager := tea_manager.NewManager(s.db)
	qrManager := qr_manager.NewManager(s.db)

	resolvers := graphql.NewResolver(logrus.WithField("pkg", "graphql"), teaManager, qrManager)

	srv := handler.NewDefaultServer(
		generated.NewExecutableSchema(
			generated.Config{Resolvers: resolvers}))

	r.Handle("/v2/", playground.Handler("GraphQL playground", "/v2/query"))
	r.Handle("/v2/query", srv)

	http.Handle("/", r)

	teaManager.Start()
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logrus.WithError(err).Panic("server httperror")
	}
	return nil
}

func NewServer(db leveldb.Storage) *Server {
	return &Server{db}
}
