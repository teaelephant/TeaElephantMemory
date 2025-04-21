package common

import (
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
)

// ErrNotString is returned when a value that should be a string is not
var ErrNotString = errors.New("value is not a string")

type ID uuid.UUID

func (id ID) MarshalGQL(w io.Writer) {
	str := uuid.UUID(id).String()
	if _, err := fmt.Fprintf(w, "\"%s\"", str); err != nil {
		fmt.Print(err)
	}
}

func (id *ID) UnmarshalGQL(v interface{}) error {
	switch v := v.(type) {
	case string:
		u, err := uuid.Parse(v)
		if err != nil {
			return err
		}

		*id = ID(u)

		return nil
	default:
		return fmt.Errorf("%w: %T", ErrNotString, v)
	}
}
