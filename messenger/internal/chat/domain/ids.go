package domain

// ChatID, MessageID и UserID — обёртки-структуры по образцу user/domain.UserID:
// получить значение можно только через Parse…, а не приведением строки.
type ChatID struct{ value string }

func ParseChatID(s string) (ChatID, error) {
	if !isUUID(s) {
		return ChatID{}, invalid("chat_id", "должен быть UUID в нижнем регистре")
	}
	return ChatID{value: s}, nil
}

func (id ChatID) String() string { return id.value }
func (id ChatID) IsZero() bool   { return id.value == "" }

type MessageID struct{ value string }

func ParseMessageID(s string) (MessageID, error) {
	if !isUUID(s) {
		return MessageID{}, invalid("message_id", "должен быть UUID в нижнем регистре")
	}
	return MessageID{value: s}, nil
}

func (id MessageID) String() string { return id.value }
func (id MessageID) IsZero() bool   { return id.value == "" }

// ClientMessageID — id, который клиент генерирует сам, до отправки, и повторяет при
// каждой попытке. По нему сервер узнаёт повтор (как random_id в Telegram, client_msg_id в Slack).
type ClientMessageID struct{ value string }

func ParseClientMessageID(s string) (ClientMessageID, error) {
	if !isUUID(s) {
		return ClientMessageID{}, invalid("client_message_id", "должен быть UUID в нижнем регистре")
	}
	return ClientMessageID{value: s}, nil
}

func (id ClientMessageID) String() string { return id.value }
func (id ClientMessageID) IsZero() bool   { return id.value == "" }

// UserID — пользователь глазами контекста «чаты»: участник или автор. Свой тип, а не
// user/domain.UserID: контекст chat не импортирует контекст user (урок 1.1).
type UserID struct{ value string }

func ParseUserID(s string) (UserID, error) {
	if !isUUID(s) {
		return UserID{}, invalid("user_id", "должен быть UUID в нижнем регистре")
	}
	return UserID{value: s}, nil
}

func (id UserID) String() string { return id.value }
func (id UserID) IsZero() bool   { return id.value == "" }

// isUUID — копия из user/domain — сознательная: контексты не делят код, чтобы в модуле 6
// разойтись в разные сервисы. Третья копия — сигнал вынести в общий пакет (правило трёх).
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
