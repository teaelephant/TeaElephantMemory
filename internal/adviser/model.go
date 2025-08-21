// Package adviser contains AI-driven tea recommendation helpers and models.
package adviser

import "github.com/teaelephant/TeaElephantMemory/common"

// Template is a data container passed into prompt templates for AI requests.
type Template struct {
	Teas      []common.Tea
	Additives []common.Tea
	Weather   common.Weather
	TimeOfDay string
	Feelings  Feelings
}

// Feelings represent optional free-form feelings input that affects recommendations.
type Feelings string

// NotEmpty reports whether feelings string is not empty.
func (f Feelings) NotEmpty() bool {
	return f != ""
}
