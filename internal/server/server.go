package server

import (
	"context"
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
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
	WriteQR(ctx context.Context, id uuid.UUID, data *common.QR) (err error)
	ReadQR(ctx context.Context, id uuid.UUID) (record *common.QR, err error)
	WriteRecord(ctx context.Context, rec *common.TeaData) (record *common.Tea, err error)
	ReadRecord(ctx context.Context, id uuid.UUID) (record *common.Tea, err error)
	ReadAllRecords(ctx context.Context, search string) ([]common.Tea, error)
	Update(ctx context.Context, id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error)
	Delete(ctx context.Context, id uuid.UUID) error
	CreateTagCategory(ctx context.Context, name string) (category *common.TagCategory, err error)
	UpdateTagCategory(ctx context.Context, id uuid.UUID, name string) error
	DeleteTagCategory(ctx context.Context, id uuid.UUID) (removedTags []uuid.UUID, err error)
	GetTagCategory(ctx context.Context, id uuid.UUID) (category *common.TagCategory, err error)
	ListTagCategories(ctx context.Context, search *string) (list []common.TagCategory, err error)
	CreateTag(ctx context.Context, name, color string, categoryID uuid.UUID) (*common.Tag, error)
	UpdateTag(ctx context.Context, id uuid.UUID, name, color string) (*common.Tag, error)
	ChangeTagCategory(ctx context.Context, id, categoryID uuid.UUID) (*common.Tag, error)
	DeleteTag(ctx context.Context, id uuid.UUID) error
	GetTag(ctx context.Context, id uuid.UUID) (*common.Tag, error)
	ListTags(ctx context.Context, name *string, categoryID *uuid.UUID) (list []common.Tag, err error)
	AddTagToTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error
	DeleteTagFromTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error
	ListByTea(ctx context.Context, id uuid.UUID) ([]common.Tag, error)
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

	srv.AddTransport(&transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Check against your desired domains here
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	})

	r.Handle("/v2/", playground.Handler("GraphQL playground", "/v2/query"))
	r.Handle("/v2/query", srv)

	http.Handle("/", r)

	teaManager.Start()
	tagManager.Start()

	originsOk := handlers.AllowedOrigins([]string{"*"})
	logrus.Info("server start on port 8080")
	if err := http.ListenAndServe(":8080", handlers.CORS(originsOk)(r)); err != nil {
		logrus.WithError(err).Panic("server httperror")
	}
	return nil
}

func NewServer(db storage) *Server {
	return &Server{db}
}
