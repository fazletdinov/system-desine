package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"messenger/internal/chat/adapters/postgres"
	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase/storetest"
	"messenger/internal/platform/pgtest"
	"messenger/internal/platform/pgtx"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Start(m)) }

// Без t.Parallel: проверки делят базу пакета, и каждая восстанавливает её из снимка.
func TestStoreContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) storetest.Store {
		pool := pgtest.Pool(t)
		seedUsers(t, pool, storetest.UserIDs()...)
		return newStore(t, pool)
	})
}

var (
	t0       = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	errAbort = errors.New("следующий шаг не удался")
)

// Запись внутри транзакции use case'а откатывается вместе с ней: каждый пишущий метод
// идёт через pgtx.Q, а не мимо транзакции через пул.
func TestCreateChatRollsBackWithTx(t *testing.T) {
	s, tx, pool := setup(t)
	c := newDirect(t)
	err := tx.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := s.CreateChat(ctx, c); err != nil {
			return err
		}
		return errAbort
	})
	if !errors.Is(err, errAbort) {
		t.Fatalf("WithinTx: ждали ошибку шага, получили %v", err)
	}
	if _, err := s.FindChat(t.Context(), c.ID()); !errors.Is(err, domain.ErrChatNotFound) {
		t.Fatalf("после отката чат остался: %v", err)
	}
	if n := count(t, pool, `SELECT count(*) FROM chat.members`); n != 0 {
		t.Fatalf("после отката осталось участников: %d", n)
	}
}

// #region rollback-test
func TestAppendMessageRollsBackWithTx(t *testing.T) {
	s, tx, _ := setup(t)
	c := newDirect(t)
	if err := s.CreateChat(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	err := tx.WithinTx(t.Context(), func(ctx context.Context) error {
		cur, err := s.FindChat(ctx, c.ID())
		if err != nil {
			return err
		}
		m := post(t, cur)
		if err := s.AppendMessage(ctx, cur, m, 0); err != nil {
			return err
		}
		return errAbort
	})
	if !errors.Is(err, errAbort) {
		t.Fatalf("WithinTx: ждали ошибку шага, получили %v", err)
	}
	msgs, err := s.ListMessagesBefore(t.Context(), c.ID(), 0, domain.ListLimitMax)
	if err != nil || len(msgs) != 0 {
		t.Fatalf("после отката в истории %d сообщений (%v)", len(msgs), err)
	}
	// Счётчик и версия откатились вместе с сообщением: пропуска в номерах не будет.
	if got, err := s.FindChat(t.Context(), c.ID()); err != nil || got.Version() != 0 || got.LastSeq() != 0 {
		t.Fatalf("после отката чат изменён: %v", err)
	}
}

// #endregion rollback-test

// Повтор client_message_id внутри транзакции: ошибка — ErrDuplicateMessage, транзакция
// оборвана (до ROLLBACK сервер не выполнит в ней ничего), откат возвращает счётчик.
// Так работает SendMessage: сохранённое сообщение он читает уже после отката.
func TestDuplicateInTxAbortsAttempt(t *testing.T) {
	s, tx, _ := setup(t)
	c := newDirect(t)
	if err := s.CreateChat(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	cur, err := s.FindChat(t.Context(), c.ID())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendMessage(t.Context(), cur, post(t, cur), 0); err != nil {
		t.Fatal(err)
	}
	var inTx error
	err = tx.WithinTx(t.Context(), func(ctx context.Context) error {
		again, err := s.FindChat(ctx, c.ID())
		if err != nil {
			return err
		}
		v := again.Version()
		if err := s.AppendMessage(ctx, again, post(t, again), v); err != nil {
			// Следующий запрос в той же транзакции — отказ 25P02: прочитать дубль здесь нельзя.
			_, inTx = s.FindChat(ctx, c.ID())
			return err
		}
		return nil
	})
	if !errors.Is(err, domain.ErrDuplicateMessage) {
		t.Fatalf("ждали ErrDuplicateMessage, получили %v", err)
	}
	if inTx == nil {
		t.Fatal("запрос после нарушения уникальности выполнился в оборванной транзакции")
	}
	if got, err := s.FindChat(t.Context(), c.ID()); err != nil || got.Version() != 1 || got.LastSeq() != 1 {
		t.Fatalf("после отката повтора чат изменён: %v", err)
	}
}

func setup(t *testing.T) (*postgres.Store, *pgtx.Transactor, *pgxpool.Pool) {
	t.Helper()
	pool := pgtest.Pool(t)
	seedUsers(t, pool, user1, user2)
	tx, err := pgtx.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	return newStore(t, pool), tx, pool
}

const (
	user1 = "00000000-0000-7000-8000-000000000001"
	user2 = "00000000-0000-7000-8000-000000000002"
)

func newStore(t *testing.T, pool *pgxpool.Pool) *postgres.Store {
	t.Helper()
	s, err := postgres.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// seedUsers заводит пользователей, на которых ссылается внешний ключ
// chat.members → users.users. Профиль — забота контекста user, поэтому строки пишем SQL
// напрямую, а не его адаптером: chat не импортирует user даже в тестах.
func seedUsers(t *testing.T, pool *pgxpool.Pool, ids ...string) {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
INSERT INTO users.users (id, username, display_name, created_at)
SELECT id, 'user_' || n, 'Пользователь ' || n, now()
FROM unnest($1::uuid[]) WITH ORDINALITY AS u(id, n)`, ids)
	if err != nil {
		t.Fatalf("пользователи для внешнего ключа: %v", err)
	}
}

func count(t *testing.T, pool *pgxpool.Pool, sql string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), sql).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func parse[T any](t *testing.T, f func(string) (T, error), s string) T {
	t.Helper()
	v, err := f(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func newDirect(t *testing.T) *domain.Chat {
	t.Helper()
	c, err := domain.NewDirectChat(parse(t, domain.ParseChatID, "00000000-0000-7000-9000-000000000001"),
		parse(t, domain.ParseUserID, user1), parse(t, domain.ParseUserID, user2), t0)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// post — сообщение от user1 с одним и тем же client_message_id: второй post того же
// чата — повтор.
func post(t *testing.T, c *domain.Chat) domain.Message {
	t.Helper()
	text, err := domain.NewText("привет")
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.Post(parse(t, domain.ParseMessageID, fmt.Sprintf("00000000-0000-7000-a000-%012d", c.LastSeq()+1)),
		parse(t, domain.ParseClientMessageID, "00000000-0000-7000-e000-000000000001"),
		parse(t, domain.ParseUserID, user1), text, t0)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
