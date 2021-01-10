package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/lueurxax/teaelephantmemory/internal/httperror"
	"github.com/lueurxax/teaelephantmemory/internal/server/api"
)

type Server struct {
	db api.Storage
}

func (s *Server) Run() error {
	errorCreator := httperror.NewCreator(logrus.WithField("pkg", "http_error"))
	tr := NewTransport()
	a := api.New(s.db, errorCreator, tr)
	r := mux.NewRouter()
	r.HandleFunc("/new_record", a.NewRecord).Methods("POST")
	r.HandleFunc("/all", a.ReadAllRecords).Methods("GET")
	r.HandleFunc("/{id}", a.ReadRecord).Methods("GET")
	r.HandleFunc("/{id}", a.UpdateRecord).Methods("POST")
	r.HandleFunc("/{id}", a.DeleteRecord).Methods("DELETE")

	http.Handle("/", r)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logrus.WithError(err).Panic("server httperror")
	}
	return nil
}

func NewServer(db api.Storage) *Server {
	return &Server{db}
}
