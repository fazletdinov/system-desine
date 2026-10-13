package usecase

import (
	"context"
	"time"

	"messenger/internal/user/domain"
)

// Clock и IDGenerator повторяют порты контекста chat сознательно: контексты не делят
// код, а порт принадлежит потребителю. Одну реализацию из platform подставляют в оба.

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}

type UserCreator interface {
	// CreateUser сохраняет нового пользователя; если имя занято — domain.ErrUsernameTaken.
	CreateUser(ctx context.Context, user domain.User) error
}

type UserFinder interface {
	// FindUser возвращает domain.ErrUserNotFound, если пользователя нет.
	FindUser(ctx context.Context, id domain.UserID) (domain.User, error)
}
