// Package common contains shared domain models used across the application.
//
//revive:disable:var-naming // keep package name for compatibility across modules
package common

//revive:enable:var-naming

import (
	"time"

	"github.com/google/uuid"
)

// BeverageType enumerations define available beverage categories.
const (
	// TeaBeverageType represents traditional tea beverages.
	TeaBeverageType BeverageType = iota
	// HerbBeverageType represents herbal infusions/additives.
	HerbBeverageType
	// CoffeeBeverageType represents coffee beverages.
	CoffeeBeverageType
	// OtherBeverageType represents any other beverage type.
	OtherBeverageType
)

// BeverageType categorizes beverages like tea, herb, coffee, or other.
type BeverageType int

// StringToBeverageType converts a string label to a BeverageType, defaulting to OtherBeverageType.
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

// BeverageTypeStringMap maps BeverageType values to their string representations.
var BeverageTypeStringMap = map[BeverageType]string{
	TeaBeverageType:    "tea",
	HerbBeverageType:   "herb",
	CoffeeBeverageType: "coffee",
	OtherBeverageType:  "other",
}

// TeaData holds descriptive fields for a tea beverage.
type TeaData struct {
	Name        string       `json:"name"`
	Type        BeverageType `json:"type"`
	Description string       `json:"description"`
}

// Tea represents a beverage entity with its metadata.
type Tea struct {
	ID uuid.UUID
	*TeaData
}

// QR describes QR-stored metadata that refers to a particular tea instance.
type QR struct {
	Tea            uuid.UUID
	BowlingTemp    int
	ExpirationDate time.Time
}
