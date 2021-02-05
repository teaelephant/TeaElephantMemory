package tag_manager

import (
	"sync"

	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

type Manager interface {
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
	AddTagToTea(tea uuid.UUID, tag uuid.UUID) error
	DeleteTagFromTea(tea uuid.UUID, tag uuid.UUID) error
	SubscribeOnCreate() (<-chan *model.Tag, error)
	SubscribeOnUpdate() (<-chan *model.Tag, error)
	SubscribeOnDelete() (<-chan gqlCommon.ID, error)
	SubscribeOnAddTagToTea() (<-chan *model.Tea, error)
	SubscribeOnDeleteTagToTea() (<-chan *model.Tea, error)
	ListByTea(id uuid.UUID) (list []common.Tag, err error)
	Start()
}

type storage interface {
	CreateTagCategory(name string) (category *common.TagCategory, err error)
	UpdateTagCategory(id uuid.UUID, name string) error
	DeleteTagCategory(id uuid.UUID) (removedTags []uuid.UUID, err error)
	GetTagCategory(id uuid.UUID) (category *common.TagCategory, err error)
	ListTagCategories(search *string) (list []common.TagCategory, err error)
	CreateTag(name, color string, categoryID uuid.UUID) (*common.Tag, error)
	UpdateTag(id uuid.UUID, name, color string) (*common.Tag, error)
	ChangeTagCategory(id, categoryID uuid.UUID) (*common.Tag, error)
	DeleteTag(id uuid.UUID) error
	GetTag(id uuid.UUID) (*common.Tag, error)
	ListTags(name *string, categoryID *uuid.UUID) (list []common.Tag, err error)
	AddTagToTea(tea uuid.UUID, tag uuid.UUID) error
	DeleteTagFromTea(tea uuid.UUID, tag uuid.UUID) error
	ListByTea(id uuid.UUID) ([]common.Tag, error)
}

type logger interface {
	Error(err ...interface{})
}

type teaManager interface {
	Get(id uuid.UUID) (*common.Tea, error)
}

type manager struct {
	teaManager
	storage
	muCreate          sync.RWMutex
	createSubscribers []chan<- *model.Tag
	muUpdate          sync.RWMutex
	updateSubscribers []chan<- *model.Tag
	muDelete          sync.RWMutex
	deleteSubscribers []chan<- gqlCommon.ID
	create            chan *common.Tag
	update            chan *common.Tag
	delete            chan uuid.UUID

	muCreateCategory          sync.RWMutex
	createSubscribersCategory []chan<- *model.TagCategory
	muUpdateCategory          sync.RWMutex
	updateSubscribersCategory []chan<- *model.TagCategory
	muDeleteCategory          sync.RWMutex
	deleteSubscribersCategory []chan<- gqlCommon.ID
	createCategory            chan *common.TagCategory
	updateCategory            chan *common.TagCategory
	deleteCategory            chan uuid.UUID

	addTagToTea               chan uuid.UUID
	deleteTagFromTea          chan uuid.UUID
	muAddTagToTea             sync.RWMutex
	addTagToTeaSubscribers    []chan<- *model.Tea
	muDeleteTagToTea          sync.RWMutex
	deleteTagToTeaSubscribers []chan<- *model.Tea

	log logger
}

func (m *manager) AddTagToTea(tea uuid.UUID, tag uuid.UUID) error {
	if err := m.storage.AddTagToTea(tea, tag); err != nil {
		return err
	}
	m.addTagToTea <- tea
	return nil
}

func (m *manager) DeleteTagFromTea(tea uuid.UUID, tag uuid.UUID) error {
	if err := m.storage.DeleteTagFromTea(tea, tag); err != nil {
		return err
	}
	m.deleteTagFromTea <- tea
	return nil
}

func (m *manager) ListByTea(id uuid.UUID) (list []common.Tag, err error) {
	return m.storage.ListByTea(id)
}

func (m *manager) CreateCategory(name string) (category *common.TagCategory, err error) {
	cat, err := m.storage.CreateTagCategory(name)
	if err != nil {
		return nil, err
	}
	m.createCategory <- cat
	return cat, nil
}

func (m *manager) DeleteCategory(id uuid.UUID) error {
	tags, err := m.storage.DeleteTagCategory(id)
	if err != nil {
		return err
	}
	for _, t := range tags {
		m.delete <- t
	}
	m.deleteCategory <- id
	return nil
}

func (m *manager) GetCategory(id uuid.UUID) (category *common.TagCategory, err error) {
	return m.storage.GetTagCategory(id)
}

func (m *manager) ListCategory(search *string) (list []common.TagCategory, err error) {
	return m.storage.ListTagCategories(search)
}

func (m *manager) Create(name, color string, categoryID uuid.UUID) (*common.Tag, error) {
	tag, err := m.storage.CreateTag(name, color, categoryID)
	if err != nil {
		return nil, err
	}
	m.create <- tag
	return tag, nil
}

func (m *manager) Update(id uuid.UUID, name, color string) (*common.Tag, error) {
	tag, err := m.storage.UpdateTag(id, name, color)
	if err != nil {
		return nil, err
	}
	m.update <- tag
	return tag, nil
}

func (m *manager) ChangeCategory(id, categoryID uuid.UUID) (*common.Tag, error) {
	tag, err := m.storage.ChangeTagCategory(id, categoryID)
	if err != nil {
		return nil, err
	}
	m.update <- tag
	return tag, nil
}

func (m *manager) Delete(id uuid.UUID) error {
	if err := m.storage.DeleteTag(id); err != nil {
		return err
	}
	m.delete <- id
	return nil
}

func (m *manager) Get(id uuid.UUID) (*common.Tag, error) {
	return m.storage.GetTag(id)
}

func (m *manager) List(name *string, categoryID *uuid.UUID) (list []common.Tag, err error) {
	return m.storage.ListTags(name, categoryID)
}

func (m *manager) UpdateCategory(id uuid.UUID, name string) (category *common.TagCategory, err error) {
	if err = m.storage.UpdateTagCategory(id, name); err != nil {
		return nil, err
	}
	res := &common.TagCategory{
		ID:   id,
		Name: name,
	}
	m.updateCategory <- res
	return res, nil
}

func (m *manager) SubscribeOnCreateCategory() (<-chan *model.TagCategory, error) {
	ch := make(chan *model.TagCategory)
	m.muCreateCategory.Lock()
	defer m.muCreateCategory.Unlock()
	m.createSubscribersCategory = append(m.createSubscribersCategory, ch)
	return ch, nil
}

func (m *manager) SubscribeOnUpdateCategory() (<-chan *model.TagCategory, error) {
	ch := make(chan *model.TagCategory)
	m.muUpdateCategory.Lock()
	defer m.muUpdateCategory.Unlock()
	m.updateSubscribersCategory = append(m.updateSubscribersCategory, ch)
	return ch, nil
}

func (m *manager) SubscribeOnDeleteCategory() (<-chan gqlCommon.ID, error) {
	ch := make(chan gqlCommon.ID)
	m.muDeleteCategory.Lock()
	defer m.muDeleteCategory.Unlock()
	m.deleteSubscribersCategory = append(m.deleteSubscribersCategory, ch)
	return ch, nil
}

func (m *manager) SubscribeOnCreate() (<-chan *model.Tag, error) {
	ch := make(chan *model.Tag)
	m.muCreate.Lock()
	defer m.muCreate.Unlock()
	m.createSubscribers = append(m.createSubscribers, ch)
	return ch, nil
}

func (m *manager) SubscribeOnUpdate() (<-chan *model.Tag, error) {
	ch := make(chan *model.Tag)
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

func (m *manager) SubscribeOnAddTagToTea() (<-chan *model.Tea, error) {
	ch := make(chan *model.Tea)
	m.muAddTagToTea.Lock()
	defer m.muAddTagToTea.Unlock()
	m.addTagToTeaSubscribers = append(m.addTagToTeaSubscribers, ch)
	return ch, nil
}

func (m *manager) SubscribeOnDeleteTagToTea() (<-chan *model.Tea, error) {
	ch := make(chan *model.Tea)
	m.muDeleteTagToTea.Lock()
	defer m.muDeleteTagToTea.Unlock()
	m.deleteTagToTeaSubscribers = append(m.deleteTagToTeaSubscribers, ch)
	return ch, nil
}

func (m *manager) Start() {
	go m.loop()
}

func NewManager(storage storage, teaManager teaManager, log logger) Manager {
	return &manager{
		teaManager:                teaManager,
		storage:                   storage,
		muCreate:                  sync.RWMutex{},
		createSubscribers:         make([]chan<- *model.Tag, 0),
		muUpdate:                  sync.RWMutex{},
		updateSubscribers:         make([]chan<- *model.Tag, 0),
		muDelete:                  sync.RWMutex{},
		deleteSubscribers:         make([]chan<- gqlCommon.ID, 0),
		create:                    make(chan *common.Tag, 100),
		update:                    make(chan *common.Tag, 100),
		delete:                    make(chan uuid.UUID, 100),
		muCreateCategory:          sync.RWMutex{},
		createSubscribersCategory: make([]chan<- *model.TagCategory, 0),
		muUpdateCategory:          sync.RWMutex{},
		updateSubscribersCategory: make([]chan<- *model.TagCategory, 0),
		muDeleteCategory:          sync.RWMutex{},
		deleteSubscribersCategory: make([]chan<- gqlCommon.ID, 0),
		createCategory:            make(chan *common.TagCategory, 100),
		updateCategory:            make(chan *common.TagCategory, 100),
		deleteCategory:            make(chan uuid.UUID, 100),
		addTagToTea:               make(chan uuid.UUID, 100),
		deleteTagFromTea:          make(chan uuid.UUID, 100),
		muAddTagToTea:             sync.RWMutex{},
		addTagToTeaSubscribers:    make([]chan<- *model.Tea, 0),
		muDeleteTagToTea:          sync.RWMutex{},
		deleteTagToTeaSubscribers: make([]chan<- *model.Tea, 0),
		log:                       log,
	}
}
