package printqr

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unidoc/unipdf/v5/creator"
)

func TestQRCodesCanBeDrawnInPDFTable(t *testing.T) {
	t.Parallel()

	data, err := NewQR()
	require.NoError(t, err)

	image, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	require.Positive(t, image.Bounds().Dx())
	require.Equal(t, image.Bounds().Dx(), image.Bounds().Dy())

	client := NewClient(creator.New())
	require.NoError(t, client.writeQRCodes([][]byte{data}))
}
