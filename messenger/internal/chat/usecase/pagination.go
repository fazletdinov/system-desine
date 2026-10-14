package usecase

import "messenger/internal/chat/domain"

// pageLimit — одно правило размера страницы для всех списков: 0 — значение по умолчанию,
// остальное — в границах домена.
func pageLimit(n int) (int, error) {
	if n == 0 {
		return domain.ListLimitDefault, nil
	}
	if err := domain.CheckListLimit(n); err != nil {
		return 0, err
	}
	return n, nil
}
