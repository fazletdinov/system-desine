package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"messenger/internal/user/domain"
)

func TestValidationErrorIsAndAs(t *testing.T) {
	t.Parallel()
	_, err := domain.NewUsername("Ivan")
	wrapped := fmt.Errorf("register user: %w", err)

	if !errors.Is(wrapped, domain.ErrInvalid) {
		t.Fatalf("errors.Is(%v, ErrInvalid) = false", wrapped)
	}
	var ve *domain.ValidationError
	if !errors.As(wrapped, &ve) {
		t.Fatalf("errors.As не нашёл *ValidationError в %v", wrapped)
	}
	if ve.Field != "username" {
		t.Fatalf("Field = %q, ждали username", ve.Field)
	}
	if errors.Is(wrapped, domain.ErrUsernameTaken) {
		t.Fatal("ошибка валидации не должна совпадать с ErrUsernameTaken")
	}
}

func TestPercentVBreaksChain(t *testing.T) {
	t.Parallel()
	// %v превращает ошибку в текст: цепочка обрывается, errors.Is её больше не видит.
	lost := fmt.Errorf("register user: %v", domain.ErrUserNotFound) //nolint:errorlint // обрыв цепочки — предмет теста
	if errors.Is(lost, domain.ErrUserNotFound) {
		t.Fatalf("errors.Is(%v, ErrUserNotFound) = true, а %%v должен был оборвать цепочку", lost)
	}
}
