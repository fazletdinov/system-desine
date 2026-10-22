package postgres

import (
	"fmt"

	"messenger/internal/platform/pgerr"
	"messenger/internal/user/domain"
)

// #region create-user-error
// usernameKey — UNIQUE (username) из миграции 00001_init.sql. Имя ограничения — часть
// контракта схемы: по нему нарушение уникальности становится «имя занято».
const usernameKey = "users_username_key"

// createUserError переводит ошибку INSERT в ошибку порта UserCreator. Дубль первичного
// ключа сюда не относится: id выдаёт генератор, и повтор — сбой сервера, а не занятое имя.
func createUserError(err error) error {
	if pgerr.IsUniqueViolation(err, usernameKey) {
		return domain.ErrUsernameTaken
	}
	return fmt.Errorf("postgres: сохранение пользователя: %w", err)
}

// #endregion create-user-error
