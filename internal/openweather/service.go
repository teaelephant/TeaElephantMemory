package openweather

import (
	"context"

	owm "github.com/briandowns/openweathermap"
	"github.com/sirupsen/logrus"

	"github.com/teaelephant/TeaElephantMemory/common"
)

type Weather interface {
	CurrentCyprus(ctx context.Context) (common.Weather, error)
}

type service struct {
	key string

	log *logrus.Entry
}

func (s *service) CurrentCyprus(context.Context) (common.Weather, error) {
	data, err := owm.NewCurrent("C", "en", s.key)
	if err != nil {
		return common.Weather{}, err
	}

	if err = data.CurrentByName("Pegeia"); err != nil {
		return common.Weather{}, err
	}

	s.log.WithField("data", data).Debug("Current weather")

	return common.Weather{
		Temperature: data.Main.Temp,
		Clouds:      data.Clouds.All,
		Rain:        common.Rain(data.Rain.OneH),
		Humidity:    data.Main.Humidity,
		WindSpeed:   data.Wind.Speed,
		Visibility:  data.Visibility,
	}, nil
}

func NewService(key string, log *logrus.Entry) Weather {
	return &service{
		key: key,
		log: log,
	}
}
