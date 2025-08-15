package openweather

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func Test_service_CurrentCyprus(t *testing.T) {
	key := os.Getenv("KEY")
	if key == "" {
		t.Skip("OPENWEATHER key not set")
	}

	t.Run("get weather", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.DebugLevel)
		s := NewService(key, logger.WithField("pkg", "openweather"))
		_, err := s.CurrentCyprus(context.Background())
		require.NoError(t, err)
	})
}
