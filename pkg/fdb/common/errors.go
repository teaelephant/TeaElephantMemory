package common

import (
	"fmt"

	"github.com/google/uuid"
)

type ErrNotFound struct {
	Type string
	ID   string
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("not found %s, id: %s", e.Type, e.ID)
}

type ErrTagExist struct {
	Name       string
	CategoryID uuid.UUID
}

func (e ErrTagExist) Error() string {
	return fmt.Sprintf("tag with name: %s, category id: %s is exist", e.Name, e.CategoryID)
}
