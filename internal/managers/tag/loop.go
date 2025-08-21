package tag

import (
	"context"

	"github.com/google/uuid"
	rootCommon "github.com/teaelephant/TeaElephantMemory/common"
	gqlCommon "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

func (m *manager) loop() {
	for {
		select {
		case tag := <-m.create:
			m.handleCreateTag(tag)
		case tag := <-m.update:
			m.handleUpdateTag(tag)
		case id := <-m.delete:
			m.deleteSubscribers.SendAll(gqlCommon.ID(id))
		case tag := <-m.createCategory:
			m.createSubscribersCategory.SendAll(&model.TagCategory{ID: gqlCommon.ID(tag.ID), Name: tag.Name})
		case tag := <-m.updateCategory:
			m.updateSubscribersCategory.SendAll(&model.TagCategory{ID: gqlCommon.ID(tag.ID), Name: tag.Name})
		case id := <-m.deleteCategory:
			m.deleteSubscribersCategory.SendAll(gqlCommon.ID(id))
		case id := <-m.addTagToTea:
			m.handleTeaTagChange(id, true)
		case id := <-m.deleteTagFromTea:
			m.handleTeaTagChange(id, false)
		}

		m.cleanAll()
	}
}

func (m *manager) handleCreateTag(tag *rootCommon.Tag) {
	cat, err := m.GetTagCategory(context.TODO(), tag.CategoryID)
	if err != nil {
		m.log.Error(err)
		return
	}

	m.createSubscribers.SendAll(&model.Tag{
		ID:       gqlCommon.ID(tag.ID),
		Name:     tag.Name,
		Color:    tag.Color,
		Category: &model.TagCategory{ID: gqlCommon.ID(cat.ID), Name: cat.Name},
	})
}

func (m *manager) handleUpdateTag(tag *rootCommon.Tag) {
	cat, err := m.GetTagCategory(context.TODO(), tag.CategoryID)
	if err != nil {
		m.log.Error(err)
		return
	}

	m.updateSubscribers.SendAll(&model.Tag{
		ID:       gqlCommon.ID(tag.ID),
		Name:     tag.Name,
		Color:    tag.Color,
		Category: &model.TagCategory{ID: gqlCommon.ID(cat.ID), Name: cat.Name},
	})
}

func (m *manager) handleTeaTagChange(id uuid.UUID, added bool) {
	tea, err := m.teaManager.Get(context.TODO(), id)
	if err != nil {
		m.log.Error(err)
		return
	}

	if added {
		m.addTagToTeaSubscribers.SendAll(model.FromCommonTea(tea))
		return
	}

	m.deleteTagToTeaSubscribers.SendAll(model.FromCommonTea(tea))
}

func (m *manager) cleanAll() {
	// remove closed connection
	m.createSubscribersCategory.CleanDone()
	m.updateSubscribersCategory.CleanDone()
	m.deleteSubscribersCategory.CleanDone()
	m.deleteSubscribers.CleanDone()
	m.createSubscribers.CleanDone()
	m.updateSubscribers.CleanDone()
	m.addTagToTeaSubscribers.CleanDone()
	m.deleteTagToTeaSubscribers.CleanDone()
}
