package common

import (
	"time"

	uuid "github.com/satori/go.uuid"
)

const (
	TeaBeverageType BeverageType = iota
	HerbBeverageType
	CoffeeBeverageType
	OtherBeverageType
)

type BeverageType int

func StringToBeverageType(str string) BeverageType {
	for beverage, data := range BeverageTypeStringMap {
		if data == str {
			return beverage
		}
	}

	return OtherBeverageType
}

func (b BeverageType) String() string {
	if str, ok := BeverageTypeStringMap[b]; ok {
		return str
	}

	return BeverageTypeStringMap[OtherBeverageType]
}

var BeverageTypeStringMap = map[BeverageType]string{
	TeaBeverageType:    "tea",
	HerbBeverageType:   "herb",
	CoffeeBeverageType: "coffee",
	OtherBeverageType:  "other",
}

type TeaData struct {
	Name        string       `json:"name"`
	Type        BeverageType `json:"type"`
	Description string       `json:"description"`
}

type Tea struct {
	ID uuid.UUID
	*TeaData
}

type QR struct {
	Tea            uuid.UUID
	BowlingTemp    int
	ExpirationDate time.Time
}
