package api

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/lueurxax/teaelephantmemory/common"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

type Storage interface {
	Write(rec *common.Record) (record *common.RecordWithID, err error)
	Read(id string) (record *common.RecordWithID, err error)
	ReadAll() ([]common.RecordWithID, error)
}

type RecordManager struct {
	Storage
}

func New(s Storage) *RecordManager {
	return &RecordManager{
		Storage: s,
	}
}

// Create new record in Storage
func (m *RecordManager) NewRecord(w http.ResponseWriter, r *http.Request) {
	logrus.Info("new record")
	record := new(common.Record)
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.WithError(err).Error("read request error")
		// TODO handle error
		return
	}
	if err := json.Unmarshal(data, record); err != nil {
		logrus.WithError(err).Error("unmarshal request error")
		// TODO handle error
		return
	}
	recWithID, err := m.Storage.Write(record)
	if err != nil {
		logrus.WithError(err).Error("write request error")
		// TODO handle error
		return
	}
	resp, err := json.Marshal(recWithID)
	if err != nil {
		logrus.WithError(err).Error("marshal response error")
		// TODO handle error
		return
	}
	if _, err := w.Write(resp); err != nil {
		logrus.WithError(err).Error("write response error")
		// TODO handle error
		return
	}
}

// Read record from Storage by id
func (m *RecordManager) ReadRecord(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	logrus.WithField("id", id).Info("read record")
	if id == "" {
		logrus.Error("empty id")
		// TODO handle error
		return
	}
	rec, err := m.Storage.Read(id)
	if err != nil {
		logrus.WithError(err).Error("read from Storage error")
		// TODO handle error
		return
	}
	data, err := json.Marshal(rec)
	if err != nil {
		logrus.WithError(err).Error("marshal response error")
		// TODO handle error
		return
	}
	if _, err := w.Write(data); err != nil {
		logrus.WithError(err).Error("write response error")
		// TODO handle error
		return
	}
}

func (m *RecordManager) ReadAllRecords(w http.ResponseWriter, _ *http.Request) {
	logrus.Info("read record")
	rec, err := m.Storage.ReadAll()
	if err != nil {
		logrus.WithError(err).Error("read from Storage error")
		// TODO handle error
		return
	}
	data, err := json.Marshal(rec)
	if err != nil {
		logrus.WithError(err).Error("marshal response error")
		// TODO handle error
		return
	}
	if _, err := w.Write(data); err != nil {
		logrus.WithError(err).Error("write response error")
		// TODO handle error
		return
	}
}
