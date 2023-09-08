package adviser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teaelephant/TeaElephantMemory/common"
)

func Test_service_execute(t *testing.T) {
	t.Run("execute prompt", func(t *testing.T) {
		s := &service{}
		require.NoError(t, s.LoadPrompt())
		got, err := s.execute(Template{
			Teas:      []common.Tea{{TeaData: &common.TeaData{Name: "example tea"}}},
			Additives: []common.Tea{{TeaData: &common.TeaData{Name: "example additives"}}},
			Weather: common.Weather{
				Temperature: 3,
				Clouds:      4,
				Rain:        2,
				Humidity:    1,
				WindSpeed:   2,
				Visibility:  3,
			},
			TimeOfDay: "10:00",
			Feelings:  "better now",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, got)
	})
}
