package printqr

import (
	"image/color"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
)

func NewQR() ([]byte, error) {
	id := uuid.New()

	code, err := qrcode.New(id.String(), qrcode.Highest)
	if err != nil {
		return nil, err
	}

	code.DisableBorder = true
	code.BackgroundColor = color.Black
	code.ForegroundColor = color.White

	data, err := code.PNG(-5)
	if err != nil {
		return nil, err
	}

	return data, nil
}
