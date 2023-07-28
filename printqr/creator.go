package printqr

import "github.com/unidoc/unipdf/v3/creator"

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
		return err
	}

	indexOnList := index % codesOnList

	var (
		top, bottom, left, right float64
	)

	if indexOnList != 0 && indexOnList != 1 {
		top = 7
	}

	if indexOnList != 4 && indexOnList != 5 {
		bottom = 7
	}

	if indexOnList != 0 && indexOnList != 2 && indexOnList != 4 {
		left = 7
	}

	if indexOnList != 1 && indexOnList != 3 && indexOnList != 5 {
		right = 7
	}

	image.SetMargins(left, right, top, bottom)
	image.SetFitMode(creator.FitModeFillWidth)

	cell := table.NewCell()
	cell.SetBackgroundColor(creator.ColorBlack)

	return cell.SetContent(image)
}
