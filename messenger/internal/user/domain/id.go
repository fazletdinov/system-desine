package domain

import "messenger/internal/kernel/ident"

// #region user-id
// UserID — идентификатор пользователя. Обёртка-структура, а не `type UserID string`:
// строку можно привести к именованному строковому типу из любого пакета
// (`domain.UserID("мусор")`), а структуру с неэкспортируемым полем — нет.
// Получить UserID можно только через ParseUserID.
type UserID struct{ value string }

// ParseUserID принимает UUID в каноническом виде: 8-4-4-4-12, шестнадцатеричные цифры
// в нижнем регистре. Версию UUID домен не проверяет — генерация id не его забота.
func ParseUserID(s string) (UserID, error) {
	if !ident.IsUUID(s) {
		return UserID{}, invalid("user_id", "должен быть UUID в нижнем регистре")
	}
	return UserID{value: s}, nil
}

func (id UserID) String() string { return id.value }

// IsZero сообщает, что значение не было создано через ParseUserID (нулевое значение структуры).
func (id UserID) IsZero() bool { return id.value == "" }

// #endregion user-id
