package teaimages

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/teaelephant/TeaElephantMemory/common"
)

const fixtureJPEGFormat = "jpeg"

func TestDecodeTeaImagePreservesValidatedJPEGAndPNG(t *testing.T) {
	t.Parallel()

	for _, format := range []string{fixtureJPEGFormat, "png"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			var source bytes.Buffer

			picture := image.NewRGBA(image.Rect(0, 0, 2, 3))
			if format == fixtureJPEGFormat {
				require.NoError(t, jpeg.Encode(&source, picture, nil))
			} else {
				require.NoError(t, png.Encode(&source, picture))
			}

			result, err := Decode(base64.StdEncoding.EncodeToString(source.Bytes()))
			require.NoError(t, err)
			require.Equal(t, "image/"+format, result.ContentType)
			require.Equal(t, 2, result.Width)
			require.Equal(t, 3, result.Height)
			require.Equal(t, source.Bytes(), result.Data)
		})
	}
}

func TestDecodeTeaImageRejectsMalformedAndTruncatedData(t *testing.T) {
	t.Parallel()

	var pngData bytes.Buffer
	require.NoError(t, png.Encode(&pngData, image.NewGray(image.Rect(0, 0, 2, 3))))
	truncated := pngData.Bytes()[:33]
	_, _, err := image.DecodeConfig(bytes.NewReader(truncated))
	require.NoError(t, err, "truncated fixture must have a readable header")

	for _, encoded := range []string{"", "not base64", "AA==", base64.StdEncoding.EncodeToString([]byte("GIF89a")), base64.StdEncoding.EncodeToString(truncated)} {
		_, err := Decode(encoded)
		require.ErrorIs(t, err, common.ErrInvalidTeaImage)
	}
}

func TestDecodeTeaImageEnforcesByteLimit(t *testing.T) {
	t.Parallel()

	var source bytes.Buffer
	require.NoError(t, jpeg.Encode(&source, image.NewGray(image.Rect(0, 0, 1, 1)), nil))

	padded := make([]byte, 5*1024*1024, 5*1024*1024+1)
	copy(padded, source.Bytes())
	_, err := Decode(base64.StdEncoding.EncodeToString(padded))
	require.NoError(t, err)

	_, err = Decode(base64.StdEncoding.EncodeToString(append(padded, 0)))
	require.ErrorIs(t, err, common.ErrInvalidTeaImage)
}

func TestDecodeTeaImageEnforcesDimensions(t *testing.T) {
	t.Parallel()

	for _, bounds := range []image.Rectangle{
		image.Rect(0, 0, 8193, 1),
		image.Rect(0, 0, 1, 8193),
		image.Rect(0, 0, 4000, 4001),
	} {
		var source bytes.Buffer
		require.NoError(t, png.Encode(&source, image.NewGray(bounds)))
		_, err := Decode(base64.StdEncoding.EncodeToString(source.Bytes()))
		require.ErrorIs(t, err, common.ErrInvalidTeaImage)
	}
}
