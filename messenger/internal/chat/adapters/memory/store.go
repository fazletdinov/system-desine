package memory

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"

	"messenger/internal/chat/domain"
)

// #region store
type Store struct {
	mu       sync.RWMutex // защищает все три карты ниже
	chats    map[domain.ChatID]domain.ChatSnapshot
	direct   map[string]domain.ChatID // DirectKeyFor(a, b) → id личного чата
	messages map[domain.ChatID][]domain.Message
}

// #endregion store

func NewStore() *Store {
	return &Store{
		chats:    make(map[domain.ChatID]domain.ChatSnapshot),
		direct:   make(map[string]domain.ChatID),
		messages: make(map[domain.ChatID][]domain.Message),
	}
}

func (s *Store) CreateChat(_ context.Context, chat *domain.Chat) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.chats[chat.ID()]; ok {
		return fmt.Errorf("memory: чат %s уже существует", chat.ID())
	}
	key, isDirect := chat.DirectKey()
	if isDirect {
		if _, ok := s.direct[key]; ok {
			return domain.ErrDirectChatExists
		}
		s.direct[key] = chat.ID()
	}
	s.chats[chat.ID()] = chat.Snapshot()
	return nil
}

func (s *Store) FindChat(_ context.Context, id domain.ChatID) (*domain.Chat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.chats[id]
	if !ok {
		return nil, domain.ErrChatNotFound
	}
	return domain.RestoreChat(snap), nil
}

func (s *Store) FindDirectChat(_ context.Context, a, b domain.UserID) (*domain.Chat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.direct[domain.DirectKeyFor(a, b)]
	if !ok {
		return nil, domain.ErrChatNotFound
	}
	return domain.RestoreChat(s.chats[id]), nil
}

// #region append
func (s *Store) AppendMessage(_ context.Context, chat *domain.Chat, msg domain.Message, expectedVersion int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.chats[chat.ID()]
	if !ok {
		return domain.ErrChatNotFound
	}
	// Проверка версии и запись — под одной блокировкой: между ними никто не вклинится.
	if stored.Version != expectedVersion {
		return domain.ErrConcurrentUpdate
	}
	s.chats[chat.ID()] = chat.Snapshot()
	s.messages[chat.ID()] = append(s.messages[chat.ID()], msg)
	return nil
}

// #endregion append

func (s *Store) ListMessages(_ context.Context, chatID domain.ChatID, afterSeq int64, limit int) ([]domain.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := s.messages[chatID]
	// Сообщения лежат по возрастанию seq — ищем первое после afterSeq двоичным поиском.
	from := sort.Search(len(all), func(i int) bool { return all[i].Seq() > afterSeq })
	to := min(from+limit, len(all))
	return slices.Clone(all[from:to]), nil
}
