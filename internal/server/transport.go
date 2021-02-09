package server

import (
	"encoding/json"
	"net/http"
)

type Transport interface {
	Response(w http.ResponseWriter, answer interface{}) error
}

type restTransport struct {
}

func (t *restTransport) Response(w http.ResponseWriter, answer interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(answer)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func NewTransport() Transport {
	return &restTransport{}
}
