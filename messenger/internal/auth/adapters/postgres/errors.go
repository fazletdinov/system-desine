package postgres

import (
	"fmt"

	"messenger/internal/auth/domain"
	"messenger/internal/platform/pgerr"
)

// credentialsKey — первичный ключ auth.credentials (user_id) из миграции 00001_init.sql:
// учётные данные у пользователя одни, второй INSERT для него нарушает этот ключ.
const credentialsKey = "credentials_pkey"

func saveCredentialsError(err error) error {
	if pgerr.IsUniqueViolation(err, credentialsKey) {
		return domain.ErrCredentialsExist
	}
	// Нарушение внешнего ключа (нет такого пользователя) сюда не относится: SignUp пишет
	// учётные данные только для профиля, который только что создал, и его отсутствие —
	// сбой сервера.
	return fmt.Errorf("postgres: сохранение учётных данных: %w", err)
}
