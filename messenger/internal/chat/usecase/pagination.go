package usecase

import (
	"fmt"

	"messenger/internal/chat/domain"
)

// pageLimit — одно правило размера страницы для всех списков: 0 — значение по умолчанию.
func pageLimit(n int) (int, error) {
	if n == 0 {
		return domain.ListLimitDefault, nil
	}
	if n < 1 || n > domain.ListLimitMax {
		return 0, &domain.ValidationError{Field: "limit", Reason: fmt.Sprintf("от 1 до %d", domain.ListLimitMax)}
	}
	return n, nil
}
