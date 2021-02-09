package tag_manager

import (
	"context"

	uuid "github.com/satori/go.uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
	"github.com/teaelephant/TeaElephantMemory/internal/tag_manager/subscribers"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

type Manager interface {
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
	AddTagToTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error
	DeleteTagFromTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error
	SubscribeOnCreate(ctx context.Context) (<-chan *model.Tag, error)
	SubscribeOnUpdate(ctx context.Context) (<-chan *model.Tag, error)
	SubscribeOnDelete(ctx context.Context) (<-chan gqlCommon.ID, error)
	SubscribeOnAddTagToTea(ctx context.Context) (<-chan *model.Tea, error)
	SubscribeOnDeleteTagToTea(ctx context.Context) (<-chan *model.Tea, error)
	ListByTea(ctx context.Context, id uuid.UUID) (list []common.Tag, err error)
	Start()
}

type storage interface {
	CreateTagCategory(ctx context.Context, name string) (category *common.TagCategory, err error)
	UpdateTagCategory(ctx context.Context, id uuid.UUID, name string) error
	DeleteTagCategory(ctx context.Context, id uuid.UUID) (removedTags []uuid.UUID, err error)
	GetTagCategory(ctx context.Context, id uuid.UUID) (category *common.TagCategory, err error)
	ListTagCategories(ctx context.Context, search *string) (list []common.TagCategory, err error)
	CreateTag(ctx context.Context, name, color string, categoryID uuid.UUID) (*common.Tag, error)
	UpdateTag(ctx context.Context, id uuid.UUID, name, color string) (*common.Tag, error)
	ChangeTagCategory(ctx context.Context, id, categoryID uuid.UUID) (*common.Tag, error)
	DeleteTag(ctx context.Context, id uuid.UUID) error
	GetTag(ctx context.Context, id uuid.UUID) (*common.Tag, error)
	ListTags(ctx context.Context, name *string, categoryID *uuid.UUID) (list []common.Tag, err error)
	AddTagToTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error
	DeleteTagFromTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error
	ListByTea(ctx context.Context, id uuid.UUID) ([]common.Tag, error)
}

type logger interface {
	Error(err ...interface{})
}

type teaManager interface {
	Get(ctx context.Context, id uuid.UUID) (*common.Tea, error)
}

type manager struct {
	teaManager
	storage
	createSubscribers subscribers.TagSubscribers
	updateSubscribers subscribers.TagSubscribers
	deleteSubscribers subscribers.IDSubscribers
	create            chan *common.Tag
	update            chan *common.Tag
	delete            chan uuid.UUID

	createSubscribersCategory subscribers.TagCategorySubscribers
	updateSubscribersCategory subscribers.TagCategorySubscribers
	deleteSubscribersCategory subscribers.IDSubscribers
	createCategory            chan *common.TagCategory
	updateCategory            chan *common.TagCategory
	deleteCategory            chan uuid.UUID

	addTagToTea               chan uuid.UUID
	deleteTagFromTea          chan uuid.UUID
	addTagToTeaSubscribers    subscribers.TeaSubscribers
	deleteTagToTeaSubscribers subscribers.TeaSubscribers

	log logger
}

func (m *manager) AddTagToTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error {
	if err := m.storage.AddTagToTea(ctx, tea, tag); err != nil {
		return err
	}
	m.addTagToTea <- tea
	return nil
}

func (m *manager) DeleteTagFromTea(ctx context.Context, tea uuid.UUID, tag uuid.UUID) error {
	if err := m.storage.DeleteTagFromTea(ctx, tea, tag); err != nil {
		return err
	}
	m.deleteTagFromTea <- tea
	return nil
}

func (m *manager) ListByTea(ctx context.Context, id uuid.UUID) (list []common.Tag, err error) {
	return m.storage.ListByTea(ctx, id)
}

func (m *manager) CreateCategory(ctx context.Context, name string) (category *common.TagCategory, err error) {
	cat, err := m.storage.CreateTagCategory(ctx, name)
	if err != nil {
		return nil, err
	}
	m.createCategory <- cat
	return cat, nil
}

func (m *manager) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	tags, err := m.storage.DeleteTagCategory(ctx, id)
	if err != nil {
		return err
	}
	for _, t := range tags {
		m.delete <- t
	}
	m.deleteCategory <- id
	return nil
}

func (m *manager) GetCategory(ctx context.Context, id uuid.UUID) (category *common.TagCategory, err error) {
	return m.storage.GetTagCategory(ctx, id)
}

func (m *manager) ListCategory(ctx context.Context, search *string) (list []common.TagCategory, err error) {
	return m.storage.ListTagCategories(ctx, search)
}

func (m *manager) Create(ctx context.Context, name, color string, categoryID uuid.UUID) (*common.Tag, error) {
	tag, err := m.storage.CreateTag(ctx, name, color, categoryID)
	if err != nil {
		return nil, err
	}
	m.create <- tag
	return tag, nil
}

func (m *manager) Update(ctx context.Context, id uuid.UUID, name, color string) (*common.Tag, error) {
	tag, err := m.storage.UpdateTag(ctx, id, name, color)
	if err != nil {
		return nil, err
	}
	m.update <- tag
	return tag, nil
}

func (m *manager) ChangeCategory(ctx context.Context, id, categoryID uuid.UUID) (*common.Tag, error) {
	tag, err := m.storage.ChangeTagCategory(ctx, id, categoryID)
	if err != nil {
		return nil, err
	}
	m.update <- tag
	return tag, nil
}

func (m *manager) Delete(ctx context.Context, id uuid.UUID) error {
	if err := m.storage.DeleteTag(ctx, id); err != nil {
		return err
	}
	m.delete <- id
	return nil
}

func (m *manager) Get(ctx context.Context, id uuid.UUID) (*common.Tag, error) {
	return m.storage.GetTag(ctx, id)
}

func (m *manager) List(ctx context.Context, name *string, categoryID *uuid.UUID) (list []common.Tag, err error) {
	return m.storage.ListTags(ctx, name, categoryID)
}

func (m *manager) UpdateCategory(ctx context.Context, id uuid.UUID, name string) (category *common.TagCategory, err error) {
	if err = m.storage.UpdateTagCategory(ctx, id, name); err != nil {
		return nil, err
	}
	res := &common.TagCategory{
		ID:   id,
		Name: name,
	}
	m.updateCategory <- res
	return res, nil
}

func (m *manager) SubscribeOnCreateCategory(ctx context.Context) (<-chan *model.TagCategory, error) {
	ch := make(chan *model.TagCategory)
	m.createSubscribersCategory.Push(ctx, ch)
	return ch, nil
}

func (m *manager) SubscribeOnUpdateCategory(ctx context.Context) (<-chan *model.TagCategory, error) {
	ch := make(chan *model.TagCategory)
	m.updateSubscribersCategory.Push(ctx, ch)
	return ch, nil
}

func (m *manager) SubscribeOnDeleteCategory(ctx context.Context) (<-chan gqlCommon.ID, error) {
	ch := make(chan gqlCommon.ID)
	m.deleteSubscribersCategory.Push(ctx, ch)
	return ch, nil
}

func (m *manager) SubscribeOnCreate(ctx context.Context) (<-chan *model.Tag, error) {
	ch := make(chan *model.Tag)
	m.createSubscribers.Push(ctx, ch)
	return ch, nil
}

func (m *manager) SubscribeOnUpdate(ctx context.Context) (<-chan *model.Tag, error) {
	ch := make(chan *model.Tag)
	m.updateSubscribers.Push(ctx, ch)
	return ch, nil
}

func (m *manager) SubscribeOnDelete(ctx context.Context) (<-chan gqlCommon.ID, error) {
	ch := make(chan gqlCommon.ID)
	m.deleteSubscribers.Push(ctx, ch)
	return ch, nil
}

func (m *manager) SubscribeOnAddTagToTea(ctx context.Context) (<-chan *model.Tea, error) {
	ch := make(chan *model.Tea)
	m.addTagToTeaSubscribers.Push(ctx, ch)
	return ch, nil
}

func (m *manager) SubscribeOnDeleteTagToTea(ctx context.Context) (<-chan *model.Tea, error) {
	ch := make(chan *model.Tea)
	m.deleteTagToTeaSubscribers.Push(ctx, ch)
	return ch, nil
}

func (m *manager) Start() {
	go m.loop()
}

func NewManager(storage storage, teaManager teaManager, log logger) Manager {
	return &manager{
		teaManager:                teaManager,
		storage:                   storage,
		createSubscribers:         subscribers.NewTagSubscribers(),
		updateSubscribers:         subscribers.NewTagSubscribers(),
		deleteSubscribers:         subscribers.NewIDSubscribers(),
		createSubscribersCategory: subscribers.NewTagCategorySubscribers(),
		updateSubscribersCategory: subscribers.NewTagCategorySubscribers(),
		deleteSubscribersCategory: subscribers.NewIDSubscribers(),
		addTagToTeaSubscribers:    subscribers.NewTeaSubscribers(),
		deleteTagToTeaSubscribers: subscribers.NewTeaSubscribers(),
		create:                    make(chan *common.Tag, 100),
		update:                    make(chan *common.Tag, 100),
		delete:                    make(chan uuid.UUID, 100),
		createCategory:            make(chan *common.TagCategory, 100),
		updateCategory:            make(chan *common.TagCategory, 100),
		deleteCategory:            make(chan uuid.UUID, 100),
		addTagToTea:               make(chan uuid.UUID, 100),
		deleteTagFromTea:          make(chan uuid.UUID, 100),
		log:                       log,
	}
}
