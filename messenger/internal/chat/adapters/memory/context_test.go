package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"messenger/internal/chat/adapters/memory"
	"messenger/internal/chat/domain"
)

// #region ctx-test
func TestCanceledContextWritesNothing(t *testing.T) {
	t.Parallel()
	store := memory.NewStore()
	chat := direct(t, 1, 1, 2)

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

// Отменённый контекст останавливает каждый метод хранилища, и ни один не меняет данные.
func TestCanceledContextEveryMethod(t *testing.T) {
	t.Parallel()
	store := memory.NewStore()
	chat := direct(t, 1, 1, 2)
	if err := store.CreateChat(context.Background(), chat); err != nil {
		t.Fatal(err)
	}
	expected := chat.Version()
	text, err := domain.NewText("не дойдёт")
	if err != nil {
		t.Fatal(err)
	}
	msgID, err := domain.ParseMessageID("00000000-0000-7000-a000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	msg, err := chat.Post(msgID, chat.Members()[0], text, t0)
	if err != nil {
		t.Fatal(err)
	}

	other := direct(t, 2, 1, 3)
	a, b := uid(t, 1), uid(t, 2)

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name string
		call func(ctx context.Context) error
	}{
		{name: "CreateChat", call: func(ctx context.Context) error { return store.CreateChat(ctx, other) }},
		{name: "FindChat", call: func(ctx context.Context) error {
			_, err := store.FindChat(ctx, chat.ID())
			return err
		}},
		{name: "FindDirectChat", call: func(ctx context.Context) error {
			_, err := store.FindDirectChat(ctx, a, b)
			return err
		}},
		{name: "AppendMessage", call: func(ctx context.Context) error { return store.AppendMessage(ctx, chat, msg, expected) }},
		// limit 0 — проверка контекста идёт раньше проверки лимита.
		{name: "ListMessages с limit 0", call: func(ctx context.Context) error {
			_, err := store.ListMessages(ctx, chat.ID(), 0, 0)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(canceled); !errors.Is(err, context.Canceled) {
				t.Fatalf("ждали context.Canceled, получили %v", err)
			}
		})
	}

	ctx := context.Background()
	if _, err := store.FindChat(ctx, other.ID()); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("отменённый CreateChat сохранил чат: %v", err)
	}
	msgs, err := store.ListMessages(ctx, chat.ID(), 0, domain.ListLimitMax)
	if err != nil || len(msgs) != 0 {
		t.Fatalf("отменённый AppendMessage записал %d сообщений, %v", len(msgs), err)
	}
	stored, err := store.FindChat(ctx, chat.ID())
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version() != expected {
		t.Fatalf("отменённый AppendMessage изменил версию чата: %d, ждали %d", stored.Version(), expected)
	}
}
