package server

import (
	"github.com/gorilla/mux"
	"github.com/lueurxax/teaelephantmemory/pkg/db"
	"github.com/lueurxax/teaelephantmemory/pkg/server/api"
	"github.com/sirupsen/logrus"
	"net/http"
)

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Run() error {
	st, err := db.NewDB("./database")
	if err != nil {
		return err
	}
	a := api.New(st)
	r := mux.NewRouter()
	r.HandleFunc("/new_record", a.NewRecord).Methods("POST")
	r.HandleFunc("/all", a.ReadAllRecords).Methods("GET")
	r.HandleFunc("/{id}", a.ReadRecord).Methods("GET")
	r.HandleFunc("/{id}", a.UpdateRecord).Methods("POST")

	http.Handle("/", r)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logrus.WithError(err).Panic("server error")
	}
	return nil
}
