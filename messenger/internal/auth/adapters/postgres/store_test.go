package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"messenger/internal/auth/adapters/postgres"
	"messenger/internal/auth/domain"
	"messenger/internal/auth/usecase/storetest"
	"messenger/internal/platform/pgtest"
	"messenger/internal/platform/pgtx"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Start(m)) }

// Без t.Parallel: проверки делят базу пакета, и каждая восстанавливает её из снимка.
func TestStoreContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) storetest.Store {
		pool := pgtest.Pool(t)
		seedUsers(t, pool, storetest.UserIDs())
		return newStore(t, pool)
	})
}

// Запись внутри транзакции use case'а откатывается вместе с ней: адаптер пишет через
// pgtx.Q, а не мимо транзакции через пул.
func TestSaveCredentialsRollsBackWithTx(t *testing.T) {
	const userID = "00000000-0000-7000-a000-000000000001"
	pool := pgtest.Pool(t)
	seedUsers(t, pool, []string{userID})
	s := newStore(t, pool)
	tx, err := pgtx.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	c := newCredentials(t, userID)
	errAbort := errors.New("следующий шаг не удался")
	err = tx.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := s.SaveCredentials(ctx, c); err != nil {
			return err
		}
		return errAbort
	})
	if !errors.Is(err, errAbort) {
		t.Fatalf("WithinTx: ждали ошибку шага, получили %v", err)
	}
	if _, err := s.FindCredentials(t.Context(), c.UserID()); !errors.Is(err, domain.ErrNoCredentials) {
		t.Fatalf("после отката учётные данные остались: %v", err)
	}
}

func newStore(t *testing.T, pool *pgxpool.Pool) *postgres.Store {
	t.Helper()
	s, err := postgres.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// seedUsers заводит пользователей, на которых ссылается внешний ключ
// auth.credentials → users.users. Профиль — забота контекста user, поэтому строки
// пишем SQL напрямую, а не его адаптером: auth не импортирует user даже в тестах.
func seedUsers(t *testing.T, pool *pgxpool.Pool, ids []string) {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
INSERT INTO users.users (id, username, display_name, created_at)
SELECT id, 'user_' || n, 'Пользователь ' || n, now()
FROM unnest($1::uuid[]) WITH ORDINALITY AS u(id, n)`, ids)
	if err != nil {
		t.Fatalf("пользователи для внешнего ключа: %v", err)
	}
}

func newCredentials(t *testing.T, userID string) domain.Credentials {
	t.Helper()
	id, err := domain.ParseUserID(userID)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := domain.NewPasswordHash("hash")
	if err != nil {
		t.Fatal(err)
	}
	c, err := domain.NewCredentials(id, hash)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
