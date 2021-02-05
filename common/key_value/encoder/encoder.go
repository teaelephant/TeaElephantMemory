package encoder

import (
	"encoding/json"

	"github.com/teaelephant/TeaElephantMemory/common"
)

type Encoder interface {
	Encode() ([]byte, error)
	Decode(data []byte) error
}

type Tea common.Tea

func (t *Tea) Encode() ([]byte, error) {
	return json.Marshal(t)
}

func (t *Tea) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}

type TeaData common.TeaData

func (t *TeaData) Encode() ([]byte, error) {
	return json.Marshal(t)
}

func (t *TeaData) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}

type QR common.QR

func (Q *QR) Encode() ([]byte, error) {
	return json.Marshal(Q)
}

func (Q *QR) Decode(data []byte) error {
	return json.Unmarshal(data, Q)
}

type TagData common.TagData

func (t *TagData) Encode() ([]byte, error) {
	return json.Marshal(t)
}

func (t *TagData) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}
