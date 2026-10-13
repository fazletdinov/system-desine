package userchecker

import (
	"context"
	"errors"
	"fmt"

	chatdomain "messenger/internal/chat/domain"
	userdomain "messenger/internal/user/domain"
	userusecase "messenger/internal/user/usecase"
)

// UserGetter — узкий порт на use case GetUser другого контекста. Объявлен здесь,
// у потребителя: контекст user не знает, что его use case кто-то так использует.
type UserGetter interface {
	Execute(ctx context.Context, q userusecase.GetUserQuery) (userdomain.User, error)
}

// Checker реализует chat/usecase.UserChecker.
type Checker struct{ users UserGetter }

func New(users UserGetter) (*Checker, error) {
	if users == nil {
		return nil, errors.New("userchecker: nil-зависимость")
	}
	return &Checker{users: users}, nil
}

// MissingUsers спрашивает пользователей по одному: в v0 они в памяти процесса, и вызов
// стоит наносекунды. Когда за портом окажется сеть (модуль 6), понадобится пакетный запрос.
func (c *Checker) MissingUsers(ctx context.Context, ids []chatdomain.UserID) ([]chatdomain.UserID, error) {
	var missing []chatdomain.UserID
	for _, id := range ids {
		_, err := c.users.Execute(ctx, userusecase.GetUserQuery{ID: id.String()})
		switch {
		case err == nil:
		case errors.Is(err, userdomain.ErrUserNotFound):
			missing = append(missing, id)
		default:
			// Ошибки чужого контекста наружу как доменные не выдаём: для chat это сбой
			// зависимости. %w сохраняет context.Canceled и DeadlineExceeded для адаптера.
			return nil, fmt.Errorf("userchecker: %w", err)
		}
	}
	return missing, nil
}
