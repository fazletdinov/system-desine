package memory

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
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

func (s *Store) CreateChat(ctx context.Context, chat *domain.Chat) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("memory: %w", err)
	}
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

func (s *Store) FindChat(ctx context.Context, id domain.ChatID) (*domain.Chat, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("memory: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.chats[id]
	if !ok {
		return nil, domain.ErrChatNotFound
	}
	return domain.RestoreChat(snap), nil
}

func (s *Store) FindDirectChat(ctx context.Context, a, b domain.UserID) (*domain.Chat, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("memory: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.direct[domain.DirectKeyFor(a, b)]
	if !ok {
		return nil, domain.ErrChatNotFound
	}
	return domain.RestoreChat(s.chats[id]), nil
}

// #region append
func (s *Store) AppendMessage(ctx context.Context, chat *domain.Chat, msg domain.Message, expectedVersion int64) error {
	// #region ctx-check
	// Отменённый запрос не пишет: клиент ушёл, и ответ о записи некому прочитать.
	// Проверка — только на входе: начатую под замком работу она не прерывает.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("memory: %w", err)
	}
	// #endregion ctx-check
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

// #region list-before
func (s *Store) ListMessagesBefore(ctx context.Context, chatID domain.ChatID, beforeSeq int64, limit int) ([]domain.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("memory: %w", err)
	}
	// Порт обещает пустую страницу на limit < 1; при отрицательном limit make ниже
	// без этой проверки запаниковал бы.
	if limit < 1 {
		return []domain.Message{}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := s.messages[chatID] // по возрастанию seq
	to := len(all)
	if beforeSeq > 0 {
		// Двоичный поиск первого сообщения с seq >= beforeSeq: всё левее — старше ключа.
		to = sort.Search(len(all), func(i int) bool { return all[i].Seq() >= beforeSeq })
	}
	from := max(0, to-limit)
	page := make([]domain.Message, 0, to-from)
	for i := to - 1; i >= from; i-- {
		page = append(page, all[i])
	}
	return page, nil
}

// #endregion list-before

// ListChatsByMember перебирает все чаты: в памяти их немного. В Postgres (модуль 2a) это
// индекс по участнику и (last_activity_at, id) и запрос «WHERE (at, id) < ($1, $2)».
func (s *Store) ListChatsByMember(ctx context.Context, member domain.UserID, after *usecase.ChatPosition, limit int) ([]*domain.Chat, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("memory: %w", err)
	}
	if limit < 1 {
		return []*domain.Chat{}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var found []*domain.Chat
	for _, snap := range s.chats {
		c := domain.RestoreChat(snap)
		if !c.IsMember(member) {
			continue
		}
		if after != nil && comparePosition(position(c), *after) >= 0 {
			continue
		}
		found = append(found, c)
	}
	slices.SortFunc(found, func(a, b *domain.Chat) int { return comparePosition(position(b), position(a)) })
	if len(found) > limit {
		found = found[:limit]
	}
	return found, nil
}

func position(c *domain.Chat) usecase.ChatPosition {
	return usecase.ChatPosition{LastActivityAt: c.LastActivityAt(), ID: c.ID()}
}

// comparePosition — порядок «по возрастанию»: сначала время, при равном — id.
// UUIDv7 растёт со временем, поэтому сравнение строк id тоже осмысленно.
func comparePosition(a, b usecase.ChatPosition) int {
	if c := a.LastActivityAt.Compare(b.LastActivityAt); c != 0 {
		return c
	}
	return strings.Compare(a.ID.String(), b.ID.String())
}
