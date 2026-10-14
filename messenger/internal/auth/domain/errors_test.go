package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"messenger/internal/auth/domain"
)

func TestValidationErrorIsErrInvalid(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("sign up: %w", &domain.ValidationError{Field: "password", Reason: "коротко"})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("ValidationError должна быть равна ErrInvalid для errors.Is")
	}
	if errors.Is(domain.ErrInvalidCredentials, domain.ErrInvalid) {
		t.Fatal("неверные учётные данные — не ошибка валидации")
	}
	if got := (&domain.ValidationError{Field: "password", Reason: "коротко"}).Error(); got != "password: коротко" {
		t.Fatalf("Error() = %q", got)
	}
}
