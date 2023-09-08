package encoder

import (
	"encoding/json"
	"testing"

	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	t.Run("decode strings", func(t *testing.T) {
		input := []string{"werwererw"}
		data, err := json.Marshal(input)
		require.NoError(t, err)

		var el []string
		require.NoError(t, Decode(data, &el))
		assert.Equal(t, input, el)
	})
	t.Run("decode uuids", func(t *testing.T) {
		input := []uuid.UUID{uuid.NewV4()}
		data, err := json.Marshal(input)
		require.NoError(t, err)

		var el []uuid.UUID
		require.NoError(t, Decode(data, &el))
		assert.Equal(t, input, el)
	})
}
