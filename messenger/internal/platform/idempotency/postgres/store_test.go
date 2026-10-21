package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"messenger/internal/platform/idempotency"
	"messenger/internal/platform/idempotency/postgres"
	"messenger/internal/platform/idempotency/storetest"
	"messenger/internal/platform/pgtest"
	"messenger/internal/platform/pgtx"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Start(m)) }

func newStore(t *testing.T) *postgres.Store {
	t.Helper()
	s, err := postgres.New(pgtest.Pool(t), storetest.ReservationTTL)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Без t.Parallel: проверки делят базу пакета, и каждая восстанавливает её из снимка.
func TestStoreContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) idempotency.Store { return newStore(t) })
}

func TestStoreSweeper(t *testing.T) {
	storetest.RunSweeper(t, func(t *testing.T) storetest.SweepStore { return newStore(t) })
}

func TestNewRejectsBadArguments(t *testing.T) {
	if _, err := postgres.New(nil, time.Second); err == nil {
		t.Error("nil-пул: ждали ошибку")
	}
	pool := pgtest.Pool(t)
	if _, err := postgres.New(pool, 0); err == nil {
		t.Error("нулевой срок резервации: ждали ошибку")
	}
}

// Резервация внутри транзакции откатывается вместе с ней: хранилище пишет через pgtx.Q.
// Сегодня middleware зовёт его вне транзакции, но запись ключа в транзакции операции —
// следующий шаг (ADR-0005), и он не должен молча идти мимо неё.
func TestReserveRollsBackWithTx(t *testing.T) {
	pool := pgtest.Pool(t)
	s, err := postgres.New(pool, storetest.ReservationTTL)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pgtx.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	rec := idempotency.Record{ExpiresAt: now.Add(idempotency.TTL)}
	errAbort := errors.New("операция не удалась")
	err = tx.WithinTx(t.Context(), func(ctx context.Context) error {
		if _, created, err := s.Reserve(ctx, "u1", "k", rec, now); err != nil || !created {
			t.Fatalf("Reserve в транзакции: created=%v, %v", created, err)
		}
		return errAbort
	})
	if !errors.Is(err, errAbort) {
		t.Fatalf("WithinTx: ждали ошибку операции, получили %v", err)
	}
	if _, created, err := s.Reserve(t.Context(), "u1", "k", rec, now); err != nil || !created {
		t.Fatalf("после отката ключ должен быть свободен: created=%v, %v", created, err)
	}
}
