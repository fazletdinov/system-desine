package domain

import "errors"

// ErrNotMember — нарушение инварианта агрегата: писать и читать может только участник.
var ErrNotMember = errors.New("пользователь не участник чата")

// invalid — единая точка создания ошибок валидации. В уроке 1.3 она станет *ValidationError.
func invalid(field, reason string) error {
	return errors.New(field + ": " + reason)
}
