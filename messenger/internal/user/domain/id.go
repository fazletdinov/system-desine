package domain

// #region user-id
// UserID — идентификатор пользователя. Обёртка-структура, а не `type UserID string`:
// строку можно привести к именованному строковому типу из любого пакета
// (`domain.UserID("мусор")`), а структуру с неэкспортируемым полем — нет.
// Получить UserID можно только через ParseUserID.
type UserID struct{ value string }

// ParseUserID принимает UUID в каноническом виде: 8-4-4-4-12, шестнадцатеричные цифры
// в нижнем регистре. Версию UUID домен не проверяет — генерация id не его забота.
func ParseUserID(s string) (UserID, error) {
	if !isUUID(s) {
		return UserID{}, invalid("user_id", "должен быть UUID в нижнем регистре")
	}
	return UserID{value: s}, nil
}

func (id UserID) String() string { return id.value }

// IsZero сообщает, что значение не было создано через ParseUserID (нулевое значение структуры).
func (id UserID) IsZero() bool { return id.value == "" }

// #endregion user-id

func isUUID(s string) bool {
	const uuidLen = 36
	if len(s) != uuidLen {
		return false
	}
	for i := range len(s) {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return false
			}
		}
	}
	return true
}
