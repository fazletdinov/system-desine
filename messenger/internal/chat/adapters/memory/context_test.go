package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"messenger/internal/chat/adapters/memory"
	"messenger/internal/chat/domain"
)

// Тот же случай есть в контракте (storetest, «отменённый контекст»), но для всех методов.
// Этот короткий тест остаётся как пример из урока 1.5.

// #region ctx-test
func TestCanceledContextWritesNothing(t *testing.T) {
	t.Parallel()
	store := memory.NewStore()
	chat := direct(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.CreateChat(ctx, chat); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateChat: ждали context.Canceled, получили %v", err)
	}
	if _, err := store.FindChat(context.Background(), chat.ID()); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("после отменённого CreateChat чат не должен появиться, получили %v", err)
	}

	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	if _, err := store.FindChat(expired, chat.ID()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("FindChat: ждали DeadlineExceeded, получили %v", err)
	}
}

// #endregion ctx-test

func direct(t *testing.T) *domain.Chat {
	t.Helper()
	id, err := domain.ParseChatID("00000000-0000-7000-9000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	a, err := domain.ParseUserID("00000000-0000-7000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	b, err := domain.ParseUserID("00000000-0000-7000-8000-000000000002")
	if err != nil {
		t.Fatal(err)
	}
	c, err := domain.NewDirectChat(id, a, b, time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return c
}
