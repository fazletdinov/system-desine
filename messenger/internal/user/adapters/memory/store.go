package memory

import (
	"context"
	"fmt"
	"sync"

	"messenger/internal/user/domain"
)

// domain.User — значение без срезов и указателей, поэтому хранить и отдавать его
// можно как есть: копия при присваивании полная, снаружи хранилище не изменить.
type Store struct {
	mu         sync.RWMutex // защищает обе карты ниже
	byID       map[domain.UserID]domain.User
	byUsername map[string]domain.UserID
}

func NewStore() *Store {
	return &Store{
		byID:       make(map[domain.UserID]domain.User),
		byUsername: make(map[string]domain.UserID),
	}
}

func (s *Store) CreateUser(ctx context.Context, user domain.User) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("memory: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Проверка и запись под одной блокировкой: два одновременных запроса с одним именем
	// не пройдут проверку оба — второй увидит запись первого.
	if _, ok := s.byUsername[user.Username().String()]; ok {
		return domain.ErrUsernameTaken
	}
	if _, ok := s.byID[user.ID()]; ok {
		return fmt.Errorf("memory: пользователь %s уже существует", user.ID())
	}
	s.byID[user.ID()] = user
	s.byUsername[user.Username().String()] = user.ID()
	return nil
}

func (s *Store) FindUser(ctx context.Context, id domain.UserID) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, fmt.Errorf("memory: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.byID[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return user, nil
}

func (s *Store) FindUserByUsername(ctx context.Context, username domain.Username) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, fmt.Errorf("memory: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byUsername[username.String()]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return s.byID[id], nil
}
