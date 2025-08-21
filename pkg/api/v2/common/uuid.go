// Package common provides shared GraphQL scalar types and helpers.
//
//revive:disable:var-naming // package name is part of public API and cannot be changed
package common

//revive:enable:var-naming

import (
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
)

// ErrNotString indicates a non-string value was provided where a string was expected.
var ErrNotString = errors.New("value is not a string")

// ID is a GraphQL-compatible UUID scalar.
type ID uuid.UUID

// MarshalGQL implements graphql.Marshaler for the ID scalar.
func (id ID) MarshalGQL(w io.Writer) {
	str := uuid.UUID(id).String()
	// Best-effort write; the interface does not allow returning an error.
	_, _ = fmt.Fprintf(w, "\"%s\"", str) //nolint:errcheck // interface does not allow returning an error
}

// UnmarshalGQL implements graphql.Unmarshaler for the ID scalar.
func (id *ID) UnmarshalGQL(v interface{}) error {
	switch v := v.(type) {
	case string:
		u, err := uuid.Parse(v)
		if err != nil {
			return fmt.Errorf("parse uuid: %w", err)
		}

		*id = ID(u)

		return nil
	default:
		return fmt.Errorf("%w: %T", ErrNotString, v)
	}
}
