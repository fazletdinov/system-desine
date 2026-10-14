// Package ident — общее ядро (shared kernel): как выглядит идентификатор в нашей системе.
// Это знание одно на все контексты, поэтому живёт в одном месте. Ядро намеренно крошечное
// и без зависимостей: менять его можно только по согласию всех контекстов (Эванс).
package ident

// #region is-uuid
// IsUUID — канонический текстовый вид UUID: 36 символов, дефисы на позициях 8, 13, 18, 23,
// остальное — шестнадцатеричные цифры в нижнем регистре. Версию не проверяет: id других
// систем (и клиентские client_message_id) бывают UUIDv4.
func IsUUID(s string) bool {
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

// #endregion is-uuid
