package postgres

import (
	"fmt"

	"messenger/internal/chat/domain"
	"messenger/internal/platform/pgerr"
)

// #region chat-errors
// Имена ограничений из миграции 00001_init.sql — часть контракта схемы: по ним
// нарушение уникальности становится доменной ошибкой.
const (
	// directKey — UNIQUE (direct_key): личный чат для пары уже есть.
	directKey = "chats_direct_key_key"
	// clientIDKey — UNIQUE (chat_id, author_id, client_message_id): повтор отправки.
	clientIDKey = "messages_client_id_key"
)

func createChatError(err error) error {
	if pgerr.IsUniqueViolation(err, directKey) {
		return domain.ErrDirectChatExists
	}
	// Дубль первичного ключа — сбой генератора id, нарушение внешнего ключа на
	// users.users — участник, которого нет, хотя CreateChat проверил всех через
	// UserChecker (или пользователя удалили между проверкой и записью): ошибки сервера.
	return fmt.Errorf("postgres: сохранение чата: %w", err)
}

// appendMessageError: повтор client_message_id — ErrDuplicateMessage без самого
// сообщения. Нарушение уникальности обрывает транзакцию, и прочитать сохранённое в ней
// нельзя; use case прочитает его после отката через FindMessageByClientID и сам сверит
// текст (ErrClientMessageIDReused).
func appendMessageError(err error) error {
	if pgerr.IsUniqueViolation(err, clientIDKey) {
		return domain.ErrDuplicateMessage
	}
	return fmt.Errorf("postgres: сохранение сообщения: %w", err)
}

// #endregion chat-errors
