package tea_manager

import (
	"sync"

	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

type TeaManager interface {
	Create(data *common.TeaData) (tea *common.Tea, err error)
	Update(id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error)
	Delete(id uuid.UUID) error
	Get(id uuid.UUID) (record *common.Tea, err error)
	List(search *string) ([]common.Tea, error)
	SubscribeOnCreate() (<-chan *model.Tea, error)
	SubscribeOnUpdate() (<-chan *model.Tea, error)
	SubscribeOnDelete() (<-chan gqlCommon.ID, error)
	Start()
}

type storage interface {
	WriteRecord(rec *common.TeaData) (record *common.Tea, err error)
	ReadRecord(id uuid.UUID) (record *common.Tea, err error)
	ReadAllRecords(search string) ([]common.Tea, error)
	Update(id uuid.UUID, rec *common.TeaData) (record *common.Tea, err error)
	Delete(id uuid.UUID) error
}

type manager struct {
	storage
	muCreate          sync.RWMutex
	createSubscribers []chan<- *model.Tea
	muUpdate          sync.RWMutex
	updateSubscribers []chan<- *model.Tea
	muDelete          sync.RWMutex
	deleteSubscribers []chan<- gqlCommon.ID
	create            chan *common.Tea
	update            chan *common.Tea
	delete            chan uuid.UUID
}

func (m *manager) SubscribeOnCreate() (<-chan *model.Tea, error) {
	ch := make(chan *model.Tea)
	m.muCreate.Lock()
	defer m.muCreate.Unlock()
	m.createSubscribers = append(m.createSubscribers, ch)
	return ch, nil
}

func (m *manager) SubscribeOnUpdate() (<-chan *model.Tea, error) {
	ch := make(chan *model.Tea)
	m.muUpdate.Lock()
	defer m.muUpdate.Unlock()
	m.updateSubscribers = append(m.updateSubscribers, ch)
	return ch, nil
}

func (m *manager) SubscribeOnDelete() (<-chan gqlCommon.ID, error) {
	ch := make(chan gqlCommon.ID)
	m.muDelete.Lock()
	defer m.muDelete.Unlock()
	m.deleteSubscribers = append(m.deleteSubscribers, ch)
	return ch, nil
}

func (m *manager) Get(id uuid.UUID) (record *common.Tea, err error) {
	return m.ReadRecord(id)
}

func (m *manager) List(search *string) ([]common.Tea, error) {
	if search == nil {
		return m.ReadAllRecords("")
	}
	return m.ReadAllRecords(*search)
}

func (m *manager) Create(data *common.TeaData) (*common.Tea, error) {
	res, err := m.storage.WriteRecord(data)
	if err != nil {
		return nil, err
	}
	m.create <- res
	return res, nil
}

func (m *manager) Update(id uuid.UUID, rec *common.TeaData) (*common.Tea, error) {
	res, err := m.storage.Update(id, rec)
	if err != nil {
		return nil, err
	}
	m.update <- res
	return res, nil
}

func (m *manager) Delete(id uuid.UUID) error {
	if err := m.storage.Delete(id); err != nil {
		return err
	}
	m.delete <- id
	return nil
}

func (m *manager) Start() {
	go m.loop()
}

func NewManager(storage storage) TeaManager {
	return &manager{
		storage:           storage,
		createSubscribers: make([]chan<- *model.Tea, 0),
		updateSubscribers: make([]chan<- *model.Tea, 0),
		deleteSubscribers: make([]chan<- gqlCommon.ID, 0),
		create:            make(chan *common.Tea, 100),
		update:            make(chan *common.Tea, 100),
		delete:            make(chan uuid.UUID, 100),
	}
}
