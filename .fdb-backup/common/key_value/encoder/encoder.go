package encoder

import (
	"encoding/json"

	"github.com/google/uuid"

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

type TeaData struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

func FromCommonTeaData(td *common.TeaData) *TeaData {
	if td == nil {
		return nil
	}

	return &TeaData{
		Name:        td.Name,
		Type:        td.Type.String(),
		Description: td.Description,
	}
}

func (t *TeaData) ToCommonTeaData() *common.TeaData {
	if t == nil {
		return nil
	}

	return &common.TeaData{
		Name:        t.Name,
		Type:        common.StringToBeverageType(t.Type),
		Description: t.Description,
	}
}

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

type Collection struct {
	Name   string
	UserID uuid.UUID
}

func (c *Collection) Encode() ([]byte, error) {
	return json.Marshal(c)
}

func (c *Collection) Decode(data []byte) error {
	return json.Unmarshal(data, c)
}

type User common.User

func (t *User) Encode() ([]byte, error) {
	return json.Marshal(t)
}

func (t *User) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}

type Device common.Device

func (t *Device) Encode() ([]byte, error) {
	return json.Marshal(t)
}

func (t *Device) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}

type Notification common.Notification

func (t *Notification) Encode() ([]byte, error) {
	return json.Marshal(t)
}

func (t *Notification) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}

func Decode(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func Encode(v any) ([]byte, error) {
	return json.Marshal(v)
}
