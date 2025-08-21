package common

import "fmt"

// Weather captures environmental conditions relevant to recommendations.
type Weather struct {
	Temperature float64
	Clouds      int
	Rain        Rain
	Humidity    int
	WindSpeed   float64
	Visibility  int
}

// Rain represents precipitation intensity in mm/h.
type Rain float64

func (r Rain) String() string {
	if r > 0 {
		return fmt.Sprintf("rain is %f mm/h", r)
	}

	return "no rain"
}

func (w Weather) String() string {
	return fmt.Sprintf(
		"temperature is %f, clouds percent is %d, %s, humidity level is %d, wind speed is %f meter/sec,, visibility is %d meters",
		w.Temperature, w.Clouds, w.Rain.String(), w.Humidity, w.WindSpeed, w.Visibility,
	)
}
