package common

import "fmt"

type Weather struct {
	Temperature float64
	Clouds      int
	Rain        float64
	Humidity    int
	WindSpeed   float64
	Visibility  int
}

func (w Weather) String() string {
	rain := "no rain"
	if w.Rain > 0 {
		rain = fmt.Sprintf("rain is %f mm/h", w.Rain)
	}

	return fmt.Sprintf(
		"temperature is %f, clouds percent is %d, %s, humidity level is %d, wind speed is %f meter/sec,, visibility is %d meters",
		w.Temperature, w.Clouds, rain, w.Humidity, w.WindSpeed, w.Visibility,
	)
}
