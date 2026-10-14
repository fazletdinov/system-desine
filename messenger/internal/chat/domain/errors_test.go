package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"messenger/internal/chat/domain"
)

// #region errors-tests
func TestValidationErrorIsAndAs(t *testing.T) {
	t.Parallel()
	_, err := domain.NewText("")
	wrapped := fmt.Errorf("send message: %w", err)

	if !errors.Is(wrapped, domain.ErrInvalid) {
		t.Fatalf("errors.Is(%v, ErrInvalid) = false", wrapped)
	}
	var ve *domain.ValidationError
	if !errors.As(wrapped, &ve) {
		t.Fatalf("errors.As не нашёл *ValidationError в %v", wrapped)
	}
	if ve.Field != "text" {
		t.Fatalf("Field = %q, ждали text", ve.Field)
	}
	if errors.Is(wrapped, domain.ErrChatNotFound) {
		t.Fatal("ошибка валидации не должна совпадать с ErrChatNotFound")
	}
}

func TestPercentVBreaksChain(t *testing.T) {
	t.Parallel()
	// %v превращает ошибку в текст: цепочка обрывается, errors.Is её больше не видит.
	lost := fmt.Errorf("send message: %v", domain.ErrChatNotFound) //nolint:errorlint // обрыв цепочки — предмет теста
	if errors.Is(lost, domain.ErrChatNotFound) {
		t.Fatalf("errors.Is(%v, ErrChatNotFound) = true, а %%v должен был оборвать цепочку", lost)
	}
}

// #endregion errors-tests

// Хранилище отвечает на повтор DuplicateMessageError с сохранённым сообщением; через
// errors.Is она равна ErrDuplicateMessage — так её узнает и тот, кому сообщение не нужно.
func TestDuplicateMessageErrorIs(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("append: %w", &domain.DuplicateMessageError{})
	if !errors.Is(err, domain.ErrDuplicateMessage) {
		t.Fatalf("errors.Is(%v, ErrDuplicateMessage) = false", err)
	}
	if errors.Is(err, domain.ErrClientMessageIDReused) || errors.Is(err, domain.ErrInvalid) {
		t.Fatal("повтор сообщения — не переиспользованный ключ и не ошибка валидации")
	}
}
