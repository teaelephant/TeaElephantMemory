package model

import "github.com/teaelephant/TeaElephantMemory/common"

func FromCommonTea(source *common.Tea) *Tea {
	return &Tea{
		ID:          source.ID,
		Name:        source.Name,
		Type:        Type(source.Type),
		Description: source.Description,
	}
}

func (t *Tea) ToCommonTea() common.Tea {
	return common.Tea{
		ID: t.ID,
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
