package openweather

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func Test_service_CurrentCyprus(t *testing.T) {
	t.Run("get weather", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.DebugLevel)
		s := NewService(os.Getenv("KEY"), logger.WithField("pkg", "openweather"))
		_, err := s.CurrentCyprus(context.Background())
		require.NoError(t, err)
	})

}
