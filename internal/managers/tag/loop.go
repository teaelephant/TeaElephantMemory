package tag

import (
	"context"

	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

func (m *manager) loop() {
	for {
		select {
		case tag := <-m.create:
			cat, err := m.GetTagCategory(context.TODO(), tag.CategoryID)
			if err != nil {
				m.log.Error(err)
				continue
			}

			m.createSubscribers.SendAll(&model.Tag{
				ID:    common.ID(tag.ID),
				Name:  tag.Name,
				Color: tag.Color,
				Category: &model.TagCategory{
					ID:   common.ID(cat.ID),
					Name: cat.Name,
				},
			})
		case tag := <-m.update:
			cat, err := m.GetTagCategory(context.TODO(), tag.CategoryID)
			if err != nil {
				m.log.Error(err)
				continue
			}

			m.updateSubscribers.SendAll(&model.Tag{
				ID:    common.ID(tag.ID),
				Name:  tag.Name,
				Color: tag.Color,
				Category: &model.TagCategory{
					ID:   common.ID(cat.ID),
					Name: cat.Name,
				},
			})
		case id := <-m.delete:
			m.deleteSubscribers.SendAll(common.ID(id))
		case tag := <-m.createCategory:
			m.createSubscribersCategory.SendAll(&model.TagCategory{
				ID:   common.ID(tag.ID),
				Name: tag.Name,
			})
		case tag := <-m.updateCategory:
			m.updateSubscribersCategory.SendAll(&model.TagCategory{
				ID:   common.ID(tag.ID),
				Name: tag.Name,
			})
		case id := <-m.deleteCategory:
			m.deleteSubscribersCategory.SendAll(common.ID(id))
		case id := <-m.addTagToTea:
			tea, err := m.teaManager.Get(context.TODO(), id)
			if err != nil {
				m.log.Error(err)
				continue
			}

			m.addTagToTeaSubscribers.SendAll(model.FromCommonTea(tea))
		case id := <-m.deleteTagFromTea:
			tea, err := m.teaManager.Get(context.TODO(), id)
			if err != nil {
				m.log.Error(err)
				continue
			}

			m.deleteTagToTeaSubscribers.SendAll(model.FromCommonTea(tea))
		}

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
}
