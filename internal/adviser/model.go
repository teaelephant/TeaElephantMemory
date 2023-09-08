package adviser

import "github.com/teaelephant/TeaElephantMemory/common"

type Template struct {
	Teas      []common.Tea
	Additives []common.Tea
	Weather   common.Weather
	TimeOfDay string
	Feelings  Feelings
}

type Feelings string

func (f Feelings) NotEmpty() bool {
	return f != ""
}
