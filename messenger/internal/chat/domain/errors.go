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
	// Повтор — не ошибка клиента: use case вернёт сохранённое сообщение. Переиспользованный
	// ключ — ошибка: тот же client_message_id пришёл с другим текстом.
	ErrDuplicateMessage      = errors.New("сообщение с этим client_message_id уже сохранено")
	ErrClientMessageIDReused = errors.New("client_message_id уже использован для другого сообщения")
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

// #region duplicate
// DuplicateMessageError — хранилище уже видело этот client_message_id от этого автора
// в этом чате. Existing — сохранённое тогда сообщение, если хранилище может его
// приложить (в памяти — может). Postgres не может: нарушение уникальности прерывает
// транзакцию, поэтому use case читает сохранённое отдельно, через свой порт поиска.
type DuplicateMessageError struct {
	Existing Message
}

func (e *DuplicateMessageError) Error() string { return ErrDuplicateMessage.Error() }

func (e *DuplicateMessageError) Is(target error) bool { return target == ErrDuplicateMessage }

// #endregion duplicate
