package common

import (
	"fmt"
	"io"

	uuid "github.com/satori/go.uuid"
)

type ID uuid.UUID

func (id ID) MarshalGQL(w io.Writer) {
	str := uuid.UUID(id).String()
	if _, err := w.Write([]byte(fmt.Sprintf("\"%s\"", str))); err != nil {
		fmt.Print(err)
	}
}

func (id *ID) UnmarshalGQL(v interface{}) error {
	switch v := v.(type) {
	case string:
		u, err := uuid.FromString(v)
		if err != nil {
			return err
		}
		*id = ID(u)
		return nil
	default:
		return fmt.Errorf("%T is not a string", v)
	}
}
