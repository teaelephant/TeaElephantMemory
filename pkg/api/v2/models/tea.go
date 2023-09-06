package model

import (
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
)

func FromCommonTea(source *common.Tea) *Tea {
	return &Tea{
		ID:          gqlCommon.ID(source.ID),
		Name:        source.Name,
		Type:        FromBeverageType(source.Type),
		Description: source.Description,
	}
}

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
	}

	return TypeOther
}

func (t Type) ToBeverageType() common.BeverageType {
	switch t {
	case TypeTea:
		return common.TeaBeverageType
	case TypeHerb:
		return common.HerbBeverageType
	case TypeCoffee:
		return common.CoffeeBeverageType
	}

	return common.OtherBeverageType
}
