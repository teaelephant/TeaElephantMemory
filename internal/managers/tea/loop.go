package tea

import (
	"github.com/teaelephant/TeaElephantMemory/pkg/api/v2/common"
	model "github.com/teaelephant/TeaElephantMemory/pkg/api/v2/models"
)

func (m *manager) loop() {
	for {
		select {
		case tea := <-m.create:
			m.createSubscribers.SendAll(model.FromCommonTea(tea))
		case tea := <-m.update:
			m.updateSubscribers.SendAll(model.FromCommonTea(tea))
		case id := <-m.delete:
			m.deleteSubscribers.SendAll(common.ID(id))
		}
		// remove closed connections
		m.createSubscribers.CleanDone()
		m.updateSubscribers.CleanDone()
		m.deleteSubscribers.CleanDone()
	}
}
