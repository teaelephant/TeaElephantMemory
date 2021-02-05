package tag_manager

import (
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

func (m *manager) loop() {
	for {
		select {
		case tag := <-m.create:
			cat, err := m.storage.GetTagCategory(tag.CategoryID)
			if err != nil {
				m.log.Error(err)
				continue
			}
			m.muCreate.RLock()
			for _, el := range m.createSubscribers {
				el <- &model.Tag{
					ID:    common.ID(tag.ID),
					Name:  tag.Name,
					Color: tag.Color,
					Category: &model.TagCategory{
						ID:   common.ID(cat.ID),
						Name: cat.Name,
					},
				}
			}
			m.muCreate.RUnlock()
		case tag := <-m.update:
			cat, err := m.storage.GetTagCategory(tag.CategoryID)
			if err != nil {
				m.log.Error(err)
				continue
			}
			m.muUpdate.RLock()
			for _, el := range m.updateSubscribers {
				el <- &model.Tag{
					ID:    common.ID(tag.ID),
					Name:  tag.Name,
					Color: tag.Color,
					Category: &model.TagCategory{
						ID:   common.ID(cat.ID),
						Name: cat.Name,
					},
				}
			}
			m.muUpdate.RUnlock()
		case id := <-m.delete:
			m.muDelete.RLock()
			for _, el := range m.deleteSubscribers {
				el <- common.ID(id)
			}
			m.muDelete.RUnlock()
		case tag := <-m.createCategory:
			m.muCreateCategory.RLock()
			for _, el := range m.createSubscribersCategory {
				el <- &model.TagCategory{
					ID:   common.ID(tag.ID),
					Name: tag.Name,
				}
			}
			m.muCreateCategory.RUnlock()
		case tag := <-m.updateCategory:
			m.muUpdateCategory.RLock()
			for _, el := range m.updateSubscribersCategory {
				el <- &model.TagCategory{
					ID:   common.ID(tag.ID),
					Name: tag.Name,
				}
			}
			m.muUpdateCategory.RUnlock()
		case id := <-m.deleteCategory:
			m.muDeleteCategory.RLock()
			for _, el := range m.deleteSubscribersCategory {
				el <- common.ID(id)
			}
			m.muDeleteCategory.RUnlock()
		case id := <-m.addTagToTea:
			tea, err := m.teaManager.Get(id)
			if err != nil {
				m.log.Error(err)
				continue
			}
			m.muAddTagToTea.RLock()
			for _, el := range m.addTagToTeaSubscribers {
				el <- model.FromCommonTea(tea)
			}
			m.muAddTagToTea.RUnlock()
		case id := <-m.deleteTagFromTea:
			tea, err := m.teaManager.Get(id)
			if err != nil {
				m.log.Error(err)
				continue
			}
			m.muDeleteTagToTea.RLock()
			for _, el := range m.deleteTagToTeaSubscribers {
				el <- model.FromCommonTea(tea)
			}
			m.muDeleteTagToTea.RUnlock()
		}
	}
}
