package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalid = errors.New("некорректные данные")
	// ErrInvalidCredentials — одна ошибка на «нет такого пользователя» и «не тот пароль»:
	// разные ответы рассказали бы атакующему, какие имена существуют.
	ErrInvalidCredentials = errors.New("неверное имя пользователя или пароль")
	ErrUsernameTaken      = errors.New("имя пользователя занято")
	// Причины отказа во входе — внутренние: Login заворачивает их в ErrInvalidCredentials
	// (errors.Is видит обе), клиенту уходит только она, а причина — в журнал.
	ErrNoSuchUser       = errors.New("пользователь не найден")
	ErrNoCredentials    = errors.New("учётные данные не найдены")
	ErrWrongPassword    = errors.New("пароль не совпал")
	ErrPasswordTooLong  = errors.New("пароль длиннее допустимого")
	ErrCredentialsExist = errors.New("учётные данные уже есть")
)

// ValidationError несёт поле и причину — их HTTP-адаптер отдаст клиенту. Is делает
// любую ValidationError равной ErrInvalid, как в контекстах user и chat.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Reason) }

func (e *ValidationError) Is(target error) bool { return target == ErrInvalid }

func invalid(field, reason string) error { return &ValidationError{Field: field, Reason: reason} }
