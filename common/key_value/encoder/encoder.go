package encoder

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

type Encoder interface {
	Encode() ([]byte, error)
	Decode(data []byte) error
}

type Tea struct {
	ID      uuid.UUID `json:"id"`
	TeaData *TeaData  `json:"tea_data,omitempty"`
}

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

type QR struct {
	Tea            uuid.UUID `json:"tea"`
	BowlingTemp    int       `json:"bowling_temp"`
	ExpirationDate time.Time `json:"expiration_date"`
}

func (q *QR) Encode() ([]byte, error) {
	return json.Marshal(q)
}

func (q *QR) Decode(data []byte) error {
	return json.Unmarshal(data, q)
}

type TagData struct {
	Name       string    `json:"name"`
	Color      string    `json:"color"`
	CategoryID uuid.UUID `json:"category_id"`
}

func (t *TagData) Encode() ([]byte, error) {
	return json.Marshal(t)
}

func (t *TagData) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}

type Collection struct {
	Name   string    `json:"name"`
	UserID uuid.UUID `json:"user_id"`
}

func (c *Collection) Encode() ([]byte, error) {
	return json.Marshal(c)
}

func (c *Collection) Decode(data []byte) error {
	return json.Unmarshal(data, c)
}

type User struct {
	ID      uuid.UUID `json:"id"`
	AppleID string    `json:"apple_id"`
	Session Session   `json:"session"`
}

type Session struct {
	JWT       string    `json:"jwt"`
	User      *User     `json:"user,omitempty"`
	ExpiredAt time.Time `json:"expired_at"`
}

func (t *User) Encode() ([]byte, error) {
	return json.Marshal(t)
}

func (t *User) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}

// ToCommonUser converts an encoder.User to a common.User
func (t *User) ToCommonUser() *common.User {
	return &common.User{
		ID:      t.ID,
		AppleID: t.AppleID,
		Session: common.Session{
			JWT:       t.Session.JWT,
			ExpiredAt: t.Session.ExpiredAt,
		},
	}
}

// FromCommonUser converts a common.User to an encoder.User
func FromCommonUser(user *common.User) *User {
	if user == nil {
		return nil
	}

	return &User{
		ID:      user.ID,
		AppleID: user.AppleID,
		Session: Session{
			JWT:       user.JWT,
			ExpiredAt: user.ExpiredAt,
		},
	}
}

type Device struct {
	UserID uuid.UUID `json:"user_id"`
	Token  string    `json:"token"`
}

func (t *Device) Encode() ([]byte, error) {
	return json.Marshal(t)
}

func (t *Device) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}

type Notification struct {
	UserID uuid.UUID               `json:"user_id"`
	Type   common.NotificationType `json:"type"`
}

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
