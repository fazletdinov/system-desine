package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/user/domain"
)

type FindByUsernameQuery struct {
	Username string
}

// FindByUsername — поиск пользователя по имени. Сейчас его зовёт вход (через мост
// userbridge контекста auth), позже — поиск собеседника.
type FindByUsername struct {
	users UserByUsernameFinder
}

func NewFindByUsername(users UserByUsernameFinder) (*FindByUsername, error) {
	if users == nil {
		return nil, errors.New("find by username: nil-зависимость")
	}
	return &FindByUsername{users: users}, nil
}

func (uc *FindByUsername) Execute(ctx context.Context, q FindByUsernameQuery) (domain.User, error) {
	username, err := domain.NewUsername(q.Username)
	if err != nil {
		return domain.User{}, err
	}
	user, err := uc.users.FindUserByUsername(ctx, username)
	if err != nil {
		return domain.User{}, fmt.Errorf("find by username: %w", err)
	}
	return user, nil
}
