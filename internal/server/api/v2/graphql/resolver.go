package graphql

import (
	"github.com/teaelephant/TeaElephantMemory/common"
	model "github.com/teaelephant/TeaElephantMemory/internal/server/api/v2/models"
)

//go:generate go run ../scripts/gqlgen.go

type logger interface {
	Debug(msgs ...interface{})
}

type teaData interface {
	Create(data *common.TeaData) (tea *common.Tea, err error)
	Update(id string, rec *common.TeaData) (record *common.Tea, err error)
	Delete(id string) error
	Get(id string) (record *common.Tea, err error)
	List(search *string) ([]common.Tea, error)
	SubscribeOnCreate() (<-chan *model.Tea, error)
	SubscribeOnUpdate() (<-chan *model.Tea, error)
	SubscribeOnDelete() (<-chan string, error)
}

type qrManager interface {
	Set(id string, data *model.QRRecordData) (err error)
	Get(id string) (*model.QRRecordData, error)
}

type Resolver struct {
	teaData
	qrManager

	log logger
}

func NewResolver(logger logger, teaData teaData, qrManager qrManager) *Resolver {
	return &Resolver{
		teaData:   teaData,
		qrManager: qrManager,
		log:       logger,
	}
}
