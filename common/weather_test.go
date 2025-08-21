package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const noRainText = "no rain"

func TestWeather_String(t *testing.T) {
	t.Run(noRainText, func(t *testing.T) {
		w := Weather{
			Temperature: -10.5,
			Clouds:      99,
			Rain:        0,
			Humidity:    55,
			WindSpeed:   56.4,
			Visibility:  10000,
		}
		assert.Equal(t, "temperature is -10.500000, clouds percent is 99, "+noRainText+", humidity level is 55, wind speed is 56.400000 meter/sec,, visibility is 10000 meters", w.String())
	})
	t.Run("rainy", func(t *testing.T) {
		w := Weather{
			Temperature: -10.5,
			Clouds:      99,
			Rain:        100.5,
			Humidity:    55,
			WindSpeed:   56.4,
			Visibility:  10000,
		}
		assert.Equal(t, "temperature is -10.500000, clouds percent is 99, rain is 100.500000 mm/h, humidity level is 55, wind speed is 56.400000 meter/sec,, visibility is 10000 meters", w.String())
	})
}
