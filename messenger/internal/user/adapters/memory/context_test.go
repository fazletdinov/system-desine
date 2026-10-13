package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"messenger/internal/user/adapters/memory"
	"messenger/internal/user/domain"
)

func TestCanceledContextWritesNothing(t *testing.T) {
	t.Parallel()
	s := memory.NewStore()
	u := user(t, 1, "alice")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.CreateUser(ctx, u); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateUser: ждали context.Canceled, получили %v", err)
	}
	if _, err := s.FindUser(context.Background(), u.ID()); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("после отменённого CreateUser пользователь не должен появиться, получили %v", err)
	}

	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	if _, err := s.FindUser(expired, u.ID()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("FindUser: ждали DeadlineExceeded, получили %v", err)
	}
}
