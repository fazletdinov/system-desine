package usecase

import (
	"context"
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
	// Если чата нет — domain.ErrChatNotFound.
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
