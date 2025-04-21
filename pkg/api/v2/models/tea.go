package model

import (
	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
)

// FromCommonTea converts a common.Tea to a Tea model.
func FromCommonTea(source *common.Tea) *Tea {
	return &Tea{
		ID:          gqlCommon.ID(source.ID),
		Name:        source.Name,
		Type:        FromBeverageType(source.Type),
		Description: source.Description,
	}
}

// ToCommonTea converts a Tea model to a common.Tea.
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

func (t *TeaData) ToCommonTeaData() *common.TeaData {
	return &common.TeaData{
		Name:        t.Name,
		Type:        t.Type.ToBeverageType(),
		Description: t.Description,
	}
}

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

	return TypeUnknown
}

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
