package domain

import "errors"

// invalid — единая точка создания ошибок валидации. В уроке 1.3 она станет *ValidationError.
func invalid(field, reason string) error {
	return errors.New(field + ": " + reason)
}
