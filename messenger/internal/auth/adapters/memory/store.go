package memory

import (
	"context"
	"fmt"
	"sync"

	"messenger/internal/auth/domain"
)

// domain.Credentials — значение без срезов и указателей: копия при присваивании полная,
// снаружи хранилище не изменить.
type Store struct {
	mu   sync.RWMutex // защищает byID
	byID map[domain.UserID]domain.Credentials
}

func NewStore() *Store {
	return &Store{byID: make(map[domain.UserID]domain.Credentials)}
}

func (s *Store) SaveCredentials(ctx context.Context, c domain.Credentials) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("memory: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Проверка и запись под одной блокировкой: из двух одновременных сохранений
	// для одного пользователя пройдёт ровно одно.
	if _, ok := s.byID[c.UserID()]; ok {
		return domain.ErrCredentialsExist
	}
	s.byID[c.UserID()] = c
	return nil
}

func (s *Store) FindCredentials(ctx context.Context, id domain.UserID) (domain.Credentials, error) {
	if err := ctx.Err(); err != nil {
		return domain.Credentials{}, fmt.Errorf("memory: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.byID[id]
	if !ok {
		return domain.Credentials{}, domain.ErrNoCredentials
	}
	return c, nil
}
