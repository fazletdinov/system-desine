package domain

import (
	"errors"
	"fmt"
)

// #region sentinels
// Сигнальные ошибки — то, что вызывающий код различает через errors.Is.
// Текст — для людей и логов; полагаться на него в коде нельзя.
var (
	ErrInvalid          = errors.New("некорректные данные")
	ErrChatNotFound     = errors.New("чат не найден")
	ErrNotMember        = errors.New("пользователь не участник чата")
	ErrConcurrentUpdate = errors.New("чат изменился одновременно с вами")
	ErrDirectChatExists = errors.New("личный чат для этой пары уже есть")
)

// #endregion sentinels

// #region validation-error
// ValidationError несёт поле и причину — их HTTP-адаптер отдаст клиенту.
// Метод Is делает любую ValidationError равной ErrInvalid для errors.Is:
// вызывающему, которому всё равно, какое поле, хватает одной проверки.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Reason) }

func (e *ValidationError) Is(target error) bool { return target == ErrInvalid }

// invalid — единая точка создания ошибок валидации: все конструкторы домена зовут её,
// поэтому смена типа ошибки здесь меняет сразу все ошибки валидации пакета.
func invalid(field, reason string) error {
	return &ValidationError{Field: field, Reason: reason}
}

// #endregion validation-error
