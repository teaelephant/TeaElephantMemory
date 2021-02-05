package server

import (
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/internal/httperror"
	"github.com/teaelephant/TeaElephantMemory/internal/qr_manager"
	"github.com/teaelephant/TeaElephantMemory/internal/tag_manager"
	"github.com/teaelephant/TeaElephantMemory/internal/tea_manager"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v1"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql"
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/graphql/generated"
)

type storage interface {
	WriteQR(id uuid.UUID, data *common.QR) (err error)
	ReadQR(id uuid.UUID) (record *common.QR, err error)
	WriteRecord(rec *common.TeaData) (record *common.Tea, err error)
	ReadRecord(id uuid.UUID) (record *common.Tea, err error)
	ReadAllRecords(search string) ([]common.Tea, error)
	Update(id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error)
	Delete(id uuid.UUID) error
	CreateTagCategory(name string) (category *common.TagCategory, err error)
	UpdateTagCategory(id uuid.UUID, name string) error
	DeleteTagCategory(id uuid.UUID) (removedTags []uuid.UUID, err error)
	GetTagCategory(id uuid.UUID) (category *common.TagCategory, err error)
	ListTagCategories(search *string) (list []common.TagCategory, err error)
	CreateTag(name, color string, categoryID uuid.UUID) (*common.Tag, error)
	UpdateTag(id uuid.UUID, name, color string) (*common.Tag, error)
	ChangeTagCategory(id, categoryID uuid.UUID) (*common.Tag, error)
	DeleteTag(id uuid.UUID) error
	GetTag(id uuid.UUID) (*common.Tag, error)
	ListTags(name *string, categoryID *uuid.UUID) (list []common.Tag, err error)
	AddTagToTea(tea uuid.UUID, tag uuid.UUID) error
	DeleteTagFromTea(tea uuid.UUID, tag uuid.UUID) error
	ListByTea(id uuid.UUID) ([]common.Tag, error)
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
	tagManager := tag_manager.NewManager(s.db, teaManager, logrus.New())

	resolvers := graphql.NewResolver(logrus.WithField("pkg", "graphql"), teaManager, qrManager, tagManager)

	srv := handler.NewDefaultServer(
		generated.NewExecutableSchema(
			generated.Config{Resolvers: resolvers}))

	r.Handle("/v2/", playground.Handler("GraphQL playground", "/v2/query"))
	r.Handle("/v2/query", srv)

	http.Handle("/", r)

	teaManager.Start()
	tagManager.Start()
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logrus.WithError(err).Panic("server httperror")
	}
	return nil
}

func NewServer(db storage) *Server {
	return &Server{db}
}
