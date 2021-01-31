package main

import (
	uuid "github.com/satori/go.uuid"
	"github.com/skip2/go-qrcode"
)

func main() {
	for i := 0; i < 50; i++ {
		id := uuid.NewV4()
		if err := qrcode.WriteFile(id.String(), qrcode.Highest, 512, id.String()+".png"); err != nil {
			panic(err)
		}
	}
}
