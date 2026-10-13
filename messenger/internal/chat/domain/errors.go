package domain

import "errors"

// ErrNotMember — нарушение инварианта агрегата: писать и читать может только участник.
var ErrNotMember = errors.New("пользователь не участник чата")

func invalid(field, reason string) error {
	return errors.New(field + ": " + reason)
}
