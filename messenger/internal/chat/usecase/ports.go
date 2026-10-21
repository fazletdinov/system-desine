package usecase

import (
	"context"
	"errors"
	"time"

	"messenger/internal/chat/domain"
)

// #region ports
// Порты объявлены здесь, у потребителя, и узкие — по одному методу:
// use case просит ровно то, что ему нужно, и не знает, кто это реализует.

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}

// UserChecker отвечает, кого из пользователей не существует.
// Реализация — адаптер над контекстом user (урок 1.7), в модуле 6 — gRPC-клиент.
type UserChecker interface {
	MissingUsers(ctx context.Context, ids []domain.UserID) ([]domain.UserID, error)
}

type ChatCreator interface {
	// CreateChat сохраняет новый чат. Для личного чата, если для пары он уже есть,
	// возвращает domain.ErrDirectChatExists.
	CreateChat(ctx context.Context, chat *domain.Chat) error
}

type ChatFinder interface {
	// FindChat возвращает domain.ErrChatNotFound, если чата нет.
	FindChat(ctx context.Context, id domain.ChatID) (*domain.Chat, error)
}

type DirectChatFinder interface {
	// FindDirectChat возвращает domain.ErrChatNotFound, если личного чата для пары нет.
	FindDirectChat(ctx context.Context, a, b domain.UserID) (*domain.Chat, error)
}

type MessageAppender interface {
	// AppendMessage сохраняет сообщение и новое состояние чата, только если версия
	// чата в хранилище всё ещё expectedVersion; иначе — domain.ErrConcurrentUpdate.
	// Если чата нет — domain.ErrChatNotFound. Если сообщение с тем же (чат, автор,
	// client_message_id) уже есть — ошибка, равная domain.ErrDuplicateMessage по errors.Is
	// (само сообщение use case читает через MessageByClientIDFinder).
	AppendMessage(ctx context.Context, chat *domain.Chat, msg domain.Message, expectedVersion int64) error
}

type MessageLister interface {
	// ListMessagesBefore возвращает до limit сообщений с seq < beforeSeq по убыванию seq;
	// beforeSeq == 0 — начиная с самого нового. Границы лимита проверяет use case
	// (domain.ListLimitMax); на limit < 1 хранилище отвечает пустой страницей, а не ошибкой.
	ListMessagesBefore(ctx context.Context, chatID domain.ChatID, beforeSeq int64, limit int) ([]domain.Message, error)
}

// ChatPosition — место чата в списке «по последней активности», ключ keyset-пагинации.
// Время одно на многих — поэтому в ключе ещё и id: порядок должен быть строгим, иначе
// чаты с одинаковым временем на границе страницы потеряются или повторятся.
type ChatPosition struct {
	LastActivityAt time.Time
	ID             domain.ChatID
}

type ChatLister interface {
	// ListChatsByMember — чаты, где member участник, по убыванию (LastActivityAt, ID),
	// строго после after (nil — с начала), не больше limit. На limit < 1 — пустая страница.
	ListChatsByMember(ctx context.Context, member domain.UserID, after *ChatPosition, limit int) ([]*domain.Chat, error)
}

// #endregion ports

// #region message-finder
// ErrMessageNotFound — часть контракта MessageByClientIDFinder: сообщения с таким ключом нет.
var ErrMessageNotFound = errors.New("сообщение не найдено")

// MessageByClientIDFinder находит сохранённое сообщение по естественному ключу. Нужен
// повтору отправки: AppendMessage отвечает только «такое уже есть», потому что в
// Postgres нарушение уникальности обрывает транзакцию попытки и прочитать в ней
// сохранённое сообщение нельзя — use case читает его уже после отката.
type MessageByClientIDFinder interface {
	// FindMessageByClientID — ErrMessageNotFound, если сообщения нет.
	FindMessageByClientID(ctx context.Context, chatID domain.ChatID, author domain.UserID, clientID domain.ClientMessageID) (domain.Message, error)
}

// #endregion message-finder

// #region transactor
// Transactor задаёт границу транзакции: попытка отправки читает чат и записывает
// сообщение с новым состоянием чата — целиком или никак (ADR-0015). Use case не знает,
// чем транзакция обеспечена: в Postgres — platform/pgtx, в тестах — фейк.
type Transactor interface {
	// WithinTx выполняет fn в одной транзакции: ошибка fn — откат. fn не запускает
	// горутины с переданным ctx: транзакция pgx не для одновременного использования.
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// #endregion transactor
