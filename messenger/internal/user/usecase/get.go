package usecase

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/user/domain"
)

type GetUserQuery struct {
	ID string
}

type GetUser struct {
	users UserFinder
}

func NewGetUser(users UserFinder) (*GetUser, error) {
	if users == nil {
		return nil, errors.New("get user: nil-зависимость")
	}
	return &GetUser{users: users}, nil
}

func (uc *GetUser) Execute(ctx context.Context, q GetUserQuery) (domain.User, error) {
	id, err := domain.ParseUserID(q.ID)
	if err != nil {
		return domain.User{}, err
	}
	user, err := uc.users.FindUser(ctx, id)
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}
