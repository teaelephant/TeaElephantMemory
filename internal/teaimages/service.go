// Package teaimages validates and manages images on shared catalog teas.
package teaimages

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg" // Register the supported image decoders.
	_ "image/png"

	"github.com/google/uuid"

	"github.com/teaelephant/TeaElephantMemory/common"
)

type storage interface {
	ListTeaImages(ctx context.Context, teaID uuid.UUID) ([]common.TeaImage, error)
	InsertTeaImage(ctx context.Context, content *common.TeaImageContent) error
	DeleteTeaImage(ctx context.Context, teaID, imageID uuid.UUID) error
	SetTeaCoverImage(ctx context.Context, teaID, imageID uuid.UUID) error
	ReadTeaImage(ctx context.Context, imageID uuid.UUID) (*common.TeaImageContent, error)
}

// Service delegates atomic catalog writes to storage after validating bytes.
type Service struct {
	storage
}

// NewService creates an image service using the existing catalog database.
func NewService(store storage) *Service {
	return &Service{storage: store}
}

// Add decodes and validates an image before attaching it to a tea.
func (s *Service) Add(ctx context.Context, teaID uuid.UUID, encoded string) error {
	content, err := Decode(encoded)
	if err != nil {
		return err
	}

	content.ID = uuid.New()
	content.TeaID = teaID

	return s.InsertTeaImage(ctx, content)
}

// Decode verifies both the image header and full image stream, before persistence.
func Decode(encoded string) (*common.TeaImageContent, error) {
	if encoded == "" || len(encoded) > base64.StdEncoding.EncodedLen(common.MaxTeaImageBytes) {
		return nil, common.ErrInvalidTeaImage
	}

	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(data) == 0 || len(data) > common.MaxTeaImageBytes {
		return nil, common.ErrInvalidTeaImage
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !validImageConfig(config, format) {
		return nil, common.ErrInvalidTeaImage
	}

	if !validImageStream(data, config, format) {
		return nil, common.ErrInvalidTeaImage
	}

	return &common.TeaImageContent{
		TeaImage: common.TeaImage{
			ContentType: fmt.Sprintf("image/%s", format),
			Width:       config.Width,
			Height:      config.Height,
		},
		Data: data,
	}, nil
}

func validImageConfig(config image.Config, format string) bool {
	if format != "jpeg" && format != "png" {
		return false
	}

	if config.Width <= 0 || config.Height <= 0 || config.Width > common.MaxTeaImageEdge || config.Height > common.MaxTeaImageEdge {
		return false
	}

	return int64(config.Width)*int64(config.Height) <= common.MaxTeaImagePixels
}

func validImageStream(data []byte, config image.Config, format string) bool {
	decoded, actualFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return false
	}

	return actualFormat == format && decoded.Bounds().Dx() == config.Width && decoded.Bounds().Dy() == config.Height
}
