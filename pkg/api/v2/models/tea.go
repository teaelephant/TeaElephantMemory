// Package model contains GraphQL models and helpers for API v2.
package model

import (
	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
)

// FromCommonTea converts a common.Tea into a GraphQL Tea.
func FromCommonTea(source *common.Tea) *Tea {
	return &Tea{
		ID:          gqlCommon.ID(source.ID),
		Name:        source.Name,
		Type:        FromBeverageType(source.Type),
		Description: source.Description,
	}
}

// ToCommonTea converts a GraphQL Tea into a common.Tea.
func (t *Tea) ToCommonTea() common.Tea {
	return common.Tea{
		ID: uuid.UUID(t.ID),
		TeaData: &common.TeaData{
			Name:        t.Name,
			Type:        t.Type.ToBeverageType(),
			Description: t.Description,
		},
	}
}

// ToCommonTeaData converts GraphQL TeaData into a common.TeaData.
func (t *TeaData) ToCommonTeaData() *common.TeaData {
	return &common.TeaData{
		Name:        t.Name,
		Type:        t.Type.ToBeverageType(),
		Description: t.Description,
	}
}

// FromBeverageType maps a common.BeverageType to the GraphQL Type.
func FromBeverageType(bt common.BeverageType) Type {
	switch bt {
	case common.TeaBeverageType:
		return TypeTea
	case common.HerbBeverageType:
		return TypeHerb
	case common.CoffeeBeverageType:
		return TypeCoffee
	case common.OtherBeverageType:
		return TypeOther
	}

	return TypeOther
}

// ToBeverageType maps GraphQL Type to a common.BeverageType.
func (t Type) ToBeverageType() common.BeverageType {
	switch t {
	case TypeTea:
		return common.TeaBeverageType
	case TypeHerb:
		return common.HerbBeverageType
	case TypeCoffee:
		return common.CoffeeBeverageType
	case TypeOther:
		return common.OtherBeverageType
	case TypeUnknown:
		return common.OtherBeverageType
	}

	return common.OtherBeverageType
}
