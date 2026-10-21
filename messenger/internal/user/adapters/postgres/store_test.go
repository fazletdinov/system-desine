package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"messenger/internal/platform/pgtest"
	"messenger/internal/platform/pgtx"
	"messenger/internal/user/adapters/postgres"
	"messenger/internal/user/domain"
	"messenger/internal/user/usecase/storetest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Start(m)) }

func newStore(t *testing.T) *postgres.Store {
	t.Helper()
	s, err := postgres.New(pgtest.Pool(t))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Без t.Parallel: проверки делят базу пакета, и каждая восстанавливает её из снимка.
func TestStoreContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) storetest.Store { return newStore(t) })
}

// Запись внутри транзакции use case'а откатывается вместе с ней: адаптер пишет через
// pgtx.Q, а не мимо транзакции через пул.
func TestCreateUserRollsBackWithTx(t *testing.T) {
	pool := pgtest.Pool(t)
	s, err := postgres.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pgtx.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	u := newUser(t)
	errAbort := errors.New("следующий шаг не удался")
	err = tx.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := s.CreateUser(ctx, u); err != nil {
			return err
		}
		return errAbort
	})
	if !errors.Is(err, errAbort) {
		t.Fatalf("WithinTx: ждали ошибку шага, получили %v", err)
	}
	if _, err := s.FindUser(t.Context(), u.ID()); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("после отката пользователь остался: %v", err)
	}
}

func newUser(t *testing.T) domain.User {
	t.Helper()
	id, err := domain.ParseUserID("00000000-0000-7000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	name, err := domain.NewUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	display, err := domain.NewDisplayName("Алиса")
	if err != nil {
		t.Fatal(err)
	}
	return domain.NewUser(id, name, display, time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC))
}
