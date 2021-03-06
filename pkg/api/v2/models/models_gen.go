// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
)

type QRRecord struct {
	ID             common.ID `json:"id"`
	Tea            *Tea      `json:"tea"`
	BowlingTemp    int       `json:"bowlingTemp"`
	ExpirationDate time.Time `json:"expirationDate"`
}

type QRRecordData struct {
	Tea            common.ID `json:"tea"`
	BowlingTemp    int       `json:"bowlingTemp"`
	ExpirationDate time.Time `json:"expirationDate"`
}

type Tag struct {
	ID       common.ID    `json:"id"`
	Name     string       `json:"name"`
	Color    string       `json:"color"`
	Category *TagCategory `json:"category"`
}

type TagCategory struct {
	ID   common.ID `json:"id"`
	Name string    `json:"name"`
}

type Tea struct {
	ID          common.ID `json:"id"`
	Name        string    `json:"name"`
	Type        Type      `json:"type"`
	Description string    `json:"description"`
	Tags        []*Tag    `json:"tags"`
}

type TeaData struct {
	Name        string `json:"name"`
	Type        Type   `json:"type"`
	Description string `json:"description"`
}

type Type string

const (
	TypeUnknown Type = "unknown"
	TypeTea     Type = "tea"
	TypeCoffee  Type = "coffee"
	TypeHerb    Type = "herb"
	TypeOther   Type = "other"
)

var AllType = []Type{
	TypeUnknown,
	TypeTea,
	TypeCoffee,
	TypeHerb,
	TypeOther,
}

func (e Type) IsValid() bool {
	switch e {
	case TypeUnknown, TypeTea, TypeCoffee, TypeHerb, TypeOther:
		return true
	}
	return false
}

func (e Type) String() string {
	return string(e)
}

func (e *Type) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = Type(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid Type", str)
	}
	return nil
}

func (e Type) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
