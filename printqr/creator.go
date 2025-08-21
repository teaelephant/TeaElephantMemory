package printqr

import (
	"github.com/pkg/errors"
	"github.com/unidoc/unipdf/v3/creator"
)

type Client struct {
	creator *creator.Creator
}

func NewClient(creator *creator.Creator) *Client {
	return &Client{creator: creator}
}

func (c *Client) generatePdf(images [][]byte) error {
	err := c.writeQRCodes(images)
	if err != nil {
		return err
	}

	return c.creator.WriteToFile("qr_codes.pdf")
}

func (c *Client) writeQRCodes(images [][]byte) error {
	table := c.creator.NewTable(cols)
	for i, image := range images {
		err := c.addQRCode(table, image, i)
		if err != nil {
			return err
		}
	}

	return c.creator.Draw(table)
}

func (c *Client) addQRCode(table *creator.Table, data []byte, index int) error {
	image, err := c.creator.NewImageFromData(data)
	if err != nil {
		return errors.Wrap(err, "failed to create image")
	}

	l, r, t, b := qrMargins(index % codesOnList)
	image.SetMargins(l, r, t, b)
	image.SetFitMode(creator.FitModeFillWidth)

	cell := table.NewCell()
	cell.SetBackgroundColor(creator.ColorBlack)

	if err = cell.SetContent(image); err != nil {
		return errors.Wrap(err, "failed to set image")
	}

	return nil
}

var qrMarginTable = [codesOnList][4]float64{
	{0, 7, 0, 7}, // idx 0
	{7, 0, 0, 7}, // idx 1
	{0, 7, 7, 7}, // idx 2
	{7, 0, 7, 7}, // idx 3
	{0, 7, 7, 0}, // idx 4
	{7, 0, 7, 0}, // idx 5
}

func qrMargins(indexOnList int) (left, right, top, bottom float64) {
	m := qrMarginTable[indexOnList]
	return m[0], m[1], m[2], m[3]
}
