package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/user/domain"
)

type RegisterUserCommand struct {
	Username    string
	DisplayName string
}

type RegisterUser struct {
	users UserCreator
	clock Clock
	ids   IDGenerator
}

func NewRegisterUser(users UserCreator, clock Clock, ids IDGenerator) (*RegisterUser, error) {
	if users == nil || clock == nil || ids == nil {
		return nil, errors.New("register user: nil-зависимость")
	}
	return &RegisterUser{users: users, clock: clock, ids: ids}, nil
}

func (uc *RegisterUser) Execute(ctx context.Context, cmd RegisterUserCommand) (domain.User, error) {
	username, err := domain.NewUsername(cmd.Username)
	if err != nil {
		return domain.User{}, err
	}
	displayName, err := domain.NewDisplayName(cmd.DisplayName)
	if err != nil {
		return domain.User{}, err
	}
	raw := uc.ids.NewID()
	id, err := domain.ParseUserID(raw)
	if err != nil {
		// Без %w: сбой генератора — ошибка сервера, а не ValidationError клиента.
		return domain.User{}, fmt.Errorf("register user: генератор выдал не UUID: %q", raw)
	}
	user := domain.NewUser(id, username, displayName, uc.clock.Now())
	if err := uc.users.CreateUser(ctx, user); err != nil {
		return domain.User{}, fmt.Errorf("register user: %w", err)
	}
	return user, nil
}
