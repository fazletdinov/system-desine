package memory_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"messenger/internal/user/adapters/memory"
	"messenger/internal/user/domain"
)

func user(t *testing.T, n int, username string) domain.User {
	t.Helper()
	id, err := domain.ParseUserID(fmt.Sprintf("00000000-0000-7000-8000-%012d", n))
	if err != nil {
		t.Fatal(err)
	}
	name, err := domain.NewUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	display, err := domain.NewDisplayName("Кто-то")
	if err != nil {
		t.Fatal(err)
	}
	return domain.NewUser(id, name, display, time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))
}

func TestCreateAndFindUser(t *testing.T) {
	t.Parallel()
	s := memory.NewStore()
	ctx := context.Background()
	u := user(t, 1, "alice")
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := s.FindUser(ctx, u.ID())
	if err != nil || got != u {
		t.Fatalf("FindUser: %+v, %v", got, err)
	}
	if _, err := s.FindUser(ctx, user(t, 2, "bob").ID()); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("ждали ErrUserNotFound, получили %v", err)
	}
}

func TestCreateUserTakenUsername(t *testing.T) {
	t.Parallel()
	s := memory.NewStore()
	ctx := context.Background()
	if err := s.CreateUser(ctx, user(t, 1, "alice")); err != nil {
		t.Fatal(err)
	}
	second := user(t, 2, "alice")
	if err := s.CreateUser(ctx, second); !errors.Is(err, domain.ErrUsernameTaken) {
		t.Fatalf("ждали ErrUsernameTaken, получили %v", err)
	}
	if _, err := s.FindUser(ctx, second.ID()); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("отклонённый пользователь не должен сохраниться: %v", err)
	}
}
