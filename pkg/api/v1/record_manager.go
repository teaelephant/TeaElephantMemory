package v1

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
)

const (
	emptyIDText = "is not id"
)

var errorEmptyID = errors.New(emptyIDText)

type Storage interface {
	WriteRecord(ctx context.Context, rec *common.TeaData) (record *common.Tea, err error)
	ReadRecord(ctx context.Context, id uuid.UUID) (record *common.Tea, err error)
	ReadAllRecords(ctx context.Context, search string) ([]common.Tea, error)
	Update(ctx context.Context, id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type errorCreator interface {
	ResponseError(w http.ResponseWriter, err common.Error)
}

type transport interface {
	Response(w http.ResponseWriter, answer interface{}) error
}

type RecordManager struct {
	Storage
	errorCreator
	transport
}

// Create new record in Storage
func (m *RecordManager) NewRecord(w http.ResponseWriter, r *http.Request) {
	logrus.Info("new record")

	record := new(common.TeaData)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.WithError(err).Error("read request httperror")

		m.ResponseError(w, common.Error{Code: http.StatusBadRequest, Msg: err})

		return
	}

	if err := json.Unmarshal(data, record); err != nil {
		logrus.WithError(err).Error("unmarshal request httperror")

		m.ResponseError(w, common.Error{Code: http.StatusBadRequest, Msg: err})

		return
	}

	recWithID, err := m.Storage.WriteRecord(context.TODO(), record)
	if err != nil {
		logrus.WithError(err).Error("write request httperror")

		m.ResponseError(w, common.Error{Code: http.StatusInternalServerError, Msg: err})

		return
	}

	if err := m.transport.Response(w, recWithID); err != nil {
		logrus.WithError(err).Error("write response httperror")

		m.ResponseError(w, common.Error{Code: http.StatusInternalServerError, Msg: err})

		return
	}
}

// Read record from Storage by id
func (m *RecordManager) ReadRecord(w http.ResponseWriter, r *http.Request) {
	id := new(uuid.UUID)

	if err := id.UnmarshalText([]byte(mux.Vars(r)["id"])); err != nil {
		logrus.Error(emptyIDText)

		m.ResponseError(w, common.Error{Code: http.StatusBadRequest, Msg: errorEmptyID})

		return
	}

	logrus.WithField("id", id).Info("read record")

	rec, err := m.Storage.ReadRecord(context.TODO(), *id)
	if err != nil {
		logrus.WithError(err).Error("read from Storage httperror")

		m.ResponseError(w, common.Error{Code: http.StatusInternalServerError, Msg: err})

		return
	}
	if err := m.transport.Response(w, rec); err != nil {
		logrus.WithError(err).Error("write response httperror")

		m.ResponseError(w, common.Error{Code: http.StatusInternalServerError, Msg: err})

		return
	}
}

func (m *RecordManager) ReadAllRecords(w http.ResponseWriter, r *http.Request) {
	logrus.Info("read record")

	name := r.URL.Query().Get("name")

	logrus.WithField("name", name).Info("search record by name")

	rec, err := m.Storage.ReadAllRecords(context.TODO(), name)
	if err != nil {
		logrus.WithError(err).Error("read from Storage httperror")

		m.ResponseError(w, common.Error{Code: http.StatusBadRequest, Msg: err})

		return
	}

	if err := m.transport.Response(w, rec); err != nil {
		logrus.WithError(err).Error("write response httperror")

		m.ResponseError(w, common.Error{Code: http.StatusInternalServerError, Msg: err})

		return
	}
}

func (m *RecordManager) UpdateRecord(w http.ResponseWriter, r *http.Request) {
	id := new(uuid.UUID)

	if err := id.UnmarshalText([]byte(mux.Vars(r)["id"])); err != nil {
		logrus.Error(emptyIDText)

		m.ResponseError(w, common.Error{Code: http.StatusBadRequest, Msg: errorEmptyID})

		return
	}

	logrus.WithField("id", id).Info("update record")

	record := new(common.TeaData)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.WithError(err).Error("read request httperror")

		m.ResponseError(w, common.Error{Code: http.StatusBadRequest, Msg: err})

		return
	}

	if err := json.Unmarshal(data, record); err != nil {
		logrus.WithError(err).Error("unmarshal request httperror")

		m.ResponseError(w, common.Error{Code: http.StatusInternalServerError, Msg: err})

		return
	}

	rec, err := m.Storage.Update(context.TODO(), *id, record)
	if err != nil {
		logrus.WithError(err).Error("read from Storage httperror")
		m.ResponseError(w, common.Error{Code: http.StatusInternalServerError, Msg: err})
		return
	}

	if err := m.transport.Response(w, rec); err != nil {
		logrus.WithError(err).Error("write response httperror")
		m.ResponseError(w, common.Error{Code: http.StatusInternalServerError, Msg: err})
		return
	}
}

func (m *RecordManager) DeleteRecord(w http.ResponseWriter, r *http.Request) {
	id := new(uuid.UUID)

	if err := id.UnmarshalText([]byte(mux.Vars(r)["id"])); err != nil {
		logrus.Error(emptyIDText)

		m.ResponseError(w, common.Error{Code: http.StatusBadRequest, Msg: errorEmptyID})

		return
	}

	logrus.WithField("id", id).Info("delete record")

	if err := m.Storage.Delete(context.TODO(), *id); err != nil {
		logrus.WithError(err).Error("delete from Storage httperror")

		m.ResponseError(w, common.Error{Code: http.StatusInternalServerError, Msg: err})
	}
	if err := m.transport.Response(w, struct {
		ID string `json:"id"`
	}{ID: id.String()}); err != nil {
		logrus.WithError(err).Error("write response httperror")

		m.ResponseError(w, common.Error{Code: http.StatusInternalServerError, Msg: err})

		return
	}
}

func New(s Storage, errorCreator errorCreator, tr transport) *RecordManager {
	return &RecordManager{
		Storage:      s,
		errorCreator: errorCreator,
		transport:    tr,
	}
}
