package graphql

import (
	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

//go:generate go run ../scripts/gqlgen.go

type logger interface {
	Debug(msgs ...interface{})
}

type teaData interface {
	Create(data *common.TeaData) (tea *common.Tea, err error)
	Update(id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error)
	Delete(id uuid.UUID) error
	Get(id uuid.UUID) (record *common.Tea, err error)
	List(search *string) ([]common.Tea, error)
	SubscribeOnCreate() (<-chan *model.Tea, error)
	SubscribeOnUpdate() (<-chan *model.Tea, error)
	SubscribeOnDelete() (<-chan gqlCommon.ID, error)
}

type qrManager interface {
	Set(id uuid.UUID, data *model.QRRecordData) (err error)
	Get(id uuid.UUID) (*model.QRRecordData, error)
}

type tagManager interface {
	CreateCategory(name string) (category *common.TagCategory, err error)
	UpdateCategory(id uuid.UUID, name string) (category *common.TagCategory, err error)
	DeleteCategory(id uuid.UUID) (err error)
	GetCategory(id uuid.UUID) (category *common.TagCategory, err error)
	ListCategory(search *string) (list []common.TagCategory, err error)
	SubscribeOnCreateCategory() (<-chan *model.TagCategory, error)
	SubscribeOnUpdateCategory() (<-chan *model.TagCategory, error)
	SubscribeOnDeleteCategory() (<-chan gqlCommon.ID, error)
	Create(name, color string, categoryID uuid.UUID) (*common.Tag, error)
	Update(id uuid.UUID, name, color string) (*common.Tag, error)
	ChangeCategory(id, categoryID uuid.UUID) (*common.Tag, error)
	Delete(id uuid.UUID) error
	Get(id uuid.UUID) (*common.Tag, error)
	List(name *string, categoryID *uuid.UUID) (list []common.Tag, err error)
	SubscribeOnCreate() (<-chan *model.Tag, error)
	SubscribeOnUpdate() (<-chan *model.Tag, error)
	SubscribeOnDelete() (<-chan gqlCommon.ID, error)
	ListByTea(id uuid.UUID) (list []common.Tag, err error)
	AddTagToTea(tea uuid.UUID, tag uuid.UUID) error
	DeleteTagFromTea(tea uuid.UUID, tag uuid.UUID) error
	SubscribeOnAddTagToTea() (<-chan *model.Tea, error)
	SubscribeOnDeleteTagToTea() (<-chan *model.Tea, error)
}

type Resolver struct {
	teaData
	qrManager
	tagManager

	log logger
}

func NewResolver(logger logger, teaData teaData, qrManager qrManager, tagManager tagManager) *Resolver {
	return &Resolver{
		teaData:    teaData,
		qrManager:  qrManager,
		tagManager: tagManager,
		log:        logger,
	}
}
