package graphql

import (
	"context"

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
	Create(ctx context.Context, data *common.TeaData) (tea *common.Tea, err error)
	Update(ctx context.Context, id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error)
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (record *common.Tea, err error)
	List(ctx context.Context, search *string) ([]common.Tea, error)
	SubscribeOnCreate(ctx context.Context) (<-chan *model.Tea, error)
	SubscribeOnUpdate(ctx context.Context) (<-chan *model.Tea, error)
	SubscribeOnDelete(ctx context.Context) (<-chan gqlCommon.ID, error)
}

type qrManager interface {
	Set(ctx context.Context, id uuid.UUID, data *model.QRRecordData) (err error)
	Get(ctx context.Context, id uuid.UUID) (*model.QRRecordData, error)
}

type tagManager interface {
	CreateCategory(ctx context.Context, name string) (category *common.TagCategory, err error)
	UpdateCategory(ctx context.Context, id uuid.UUID, name string) (category *common.TagCategory, err error)
	DeleteCategory(ctx context.Context, id uuid.UUID) (err error)
	GetCategory(ctx context.Context, id uuid.UUID) (category *common.TagCategory, err error)
	ListCategory(ctx context.Context, search *string) (list []common.TagCategory, err error)
	SubscribeOnCreateCategory(ctx context.Context) (<-chan *model.TagCategory, error)
	SubscribeOnUpdateCategory(ctx context.Context) (<-chan *model.TagCategory, error)
	SubscribeOnDeleteCategory(ctx context.Context) (<-chan gqlCommon.ID, error)
	Create(ctx context.Context, name, color string, categoryID uuid.UUID) (*common.Tag, error)
	Update(ctx context.Context, id uuid.UUID, name, color string) (*common.Tag, error)
	ChangeCategory(ctx context.Context, id, categoryID uuid.UUID) (*common.Tag, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*common.Tag, error)
	List(ctx context.Context, name *string, categoryID *uuid.UUID) (list []common.Tag, err error)
	SubscribeOnCreate(ctx context.Context) (<-chan *model.Tag, error)
	SubscribeOnUpdate(ctx context.Context) (<-chan *model.Tag, error)
	SubscribeOnDelete(ctx context.Context) (<-chan gqlCommon.ID, error)
	ListByTea(ctx context.Context, id uuid.UUID) (list []common.Tag, err error)
	AddTagToTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error
	DeleteTagFromTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error
	SubscribeOnAddTagToTea(ctx context.Context) (<-chan *model.Tea, error)
	SubscribeOnDeleteTagToTea(ctx context.Context) (<-chan *model.Tea, error)
}

type collectionManager interface {
	Create(ctx context.Context, userID uuid.UUID, name string) (*model.Collection, error)
	AddRecords(ctx context.Context, userID uuid.UUID, id uuid.UUID, teas []uuid.UUID) (*model.Collection, error)
	DeleteRecords(ctx context.Context, userID uuid.UUID, id uuid.UUID, teas []uuid.UUID) (*model.Collection, error)
	Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	List(ctx context.Context, userID uuid.UUID) ([]*model.Collection, error)
	ListRecords(ctx context.Context, id, userID uuid.UUID) ([]*model.QRRecord, error)
}

type auth interface {
	Auth(ctx context.Context, token string) (*common.Session, error)
}

type Resolver struct {
	teaData
	qrManager
	tagManager
	collectionManager
	auth

	log logger
}

func NewResolver(
	logger logger,
	teaData teaData,
	qrManager qrManager,
	tagManager tagManager,
	manager collectionManager,
	auth auth,
) *Resolver {
	return &Resolver{
		teaData:           teaData,
		qrManager:         qrManager,
		tagManager:        tagManager,
		collectionManager: manager,
		auth:              auth,
		log:               logger,
	}
}
