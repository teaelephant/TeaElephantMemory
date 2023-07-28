package printqr

import (
	"github.com/unidoc/unipdf/v3/common/license"
	"github.com/unidoc/unipdf/v3/creator"
)

const (
	marginRight = 80
	marginEmpty = 0
	codesOnList = 6
	cols        = 2
)

type Generator interface {
	GenerateAndSave(lists int) error
}

type generator struct {
	key string
}

func NewGenerator(key string) Generator {
	return &generator{key: key}
}

func (g *generator) GenerateAndSave(lists int) (err error) {
	items := make([][]byte, lists*codesOnList)
	for i := 0; i < lists*codesOnList; i++ {
		items[i], err = NewQR()
		if err != nil {
			return err
		}
	}

	return g.GenerateQRPdf(items)
}

func (g *generator) GenerateQRPdf(images [][]byte) error {
	err := license.SetMeteredKey(g.key)
	if err != nil {
		return err
	}

	c := creator.New()
	c.SetPageMargins(marginEmpty, marginRight, marginEmpty, marginEmpty)

	cr := NewClient(c)

	return cr.generatePdf(images)
}
