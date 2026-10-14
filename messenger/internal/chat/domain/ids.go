package domain

import "messenger/internal/kernel/ident"

// ChatID, MessageID и UserID — обёртки-структуры по образцу user/domain.UserID:
// получить значение можно только через Parse…, а не приведением строки. Формат id —
// общее знание всех контекстов, поэтому проверка одна: kernel/ident (правило трёх).
type ChatID struct{ value string }

func ParseChatID(s string) (ChatID, error) {
	if !ident.IsUUID(s) {
		return ChatID{}, invalid("chat_id", "должен быть UUID в нижнем регистре")
	}
	return ChatID{value: s}, nil
}

func (id ChatID) String() string { return id.value }
func (id ChatID) IsZero() bool   { return id.value == "" }

type MessageID struct{ value string }

func ParseMessageID(s string) (MessageID, error) {
	if !ident.IsUUID(s) {
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
	if !ident.IsUUID(s) {
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
	if !ident.IsUUID(s) {
		return UserID{}, invalid("user_id", "должен быть UUID в нижнем регистре")
	}
	return UserID{value: s}, nil
}

func (id UserID) String() string { return id.value }
func (id UserID) IsZero() bool   { return id.value == "" }
