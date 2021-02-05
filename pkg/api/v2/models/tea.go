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
		Type:        Type(source.Type),
		Description: source.Description,
	}
}

func (t *Tea) ToCommonTea() common.Tea {
	return common.Tea{
		ID: uuid.UUID(t.ID),
		TeaData: &common.TeaData{
			Name:        t.Name,
			Type:        t.Type.String(),
			Description: t.Description,
		},
	}
}

func (t *TeaData) ToCommonTeaData() *common.TeaData {
	return &common.TeaData{
		Name:        t.Name,
		Type:        t.Type.String(),
		Description: t.Description,
	}
}
