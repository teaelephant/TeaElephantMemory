package httperror

import (
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/lueurxax/teaelephantmemory/common"
)

type Creator interface {
	ResponseError(w http.ResponseWriter, err common.Error)
}

type errorCreator struct {
	log *logrus.Entry
}

func (e *errorCreator) ResponseError(w http.ResponseWriter, intErr common.Error) {
	w.WriteHeader(intErr.Code)
	msg := common.HttpError{Error: intErr.Msg.Error()}
	data, err := json.Marshal(&msg)
	if err != nil {
		e.log.WithField("unhandled_error", intErr).WithError(err).Error("can't marshal error")
		w.WriteHeader(http.StatusInternalServerError)
		msg = common.HttpError{Error: "can't marshal error"}
		data, err = json.Marshal(&msg)
		if err != nil {
			e.log.WithField("unhandled_error", intErr).WithError(err).Error("can't marshal error")
			return
		}
	}
	if _, err = w.Write(data); err != nil {
		e.log.WithField("unhandled_error", intErr).WithError(err).Error("can't write response")
	}
}

func NewCreator(log *logrus.Entry) Creator {
	return &errorCreator{log: log}
}
