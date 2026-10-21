package pgtx_test

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"messenger/internal/platform/pgtest"
	"messenger/internal/platform/pgtx"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Start(m)) }

// Тестам транзакций нужна любая таблица с данными — берём users.users из схемы мессенджера.
func insertUser(ctx context.Context, q pgtx.Querier, username string) error {
	_, err := q.Exec(ctx,
		`INSERT INTO users.users (id, username, display_name, created_at) VALUES (gen_random_uuid(), $1, $1, now())`,
		username)
	return err
}

// usersNamed считает строки через пул — то есть из другого соединения, вне транзакции
// теста: видно только зафиксированное.
func usersNamed(t *testing.T, pool *pgxpool.Pool, username string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM users.users WHERE username = $1`, username).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func newTransactor(t *testing.T) (*pgtx.Transactor, *pgxpool.Pool) {
	t.Helper()
	pool := pgtest.Pool(t)
	tr, err := pgtx.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	return tr, pool
}

// newLeakTolerantTransactor — для тестов, проверяющих, что соединение вернулось в пул.
// Пул свой: если откат не случится, соединение останется занятым, и pool.Close в
// очистке pgtest ждал бы его вечно — тест завис бы вместо понятного провала. Утёкшее
// соединение оборвёт Restore следующего теста (DROP DATABASE … WITH (FORCE)).
func newLeakTolerantTransactor(t *testing.T) (*pgtx.Transactor, *pgxpool.Pool) {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), pgtest.Pool(t).Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if pool.Stat().AcquiredConns() == 0 {
			pool.Close()
		}
	})
	tr, err := pgtx.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	return tr, pool
}

func TestNewRejectsNilPool(t *testing.T) {
	t.Parallel()
	if _, err := pgtx.New(nil); err == nil {
		t.Fatal("ждали ошибку на nil-пул")
	}
}

func TestWithinTxCommits(t *testing.T) {
	tr, pool := newTransactor(t)
	err := tr.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := insertUser(ctx, pgtx.Q(ctx, pool), "alice"); err != nil {
			return err
		}
		// До фиксации другие соединения строку не видят: запись — внутри транзакции.
		if n := usersNamed(t, pool, "alice"); n != 0 {
			t.Errorf("незафиксированная строка видна снаружи: %d", n)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := usersNamed(t, pool, "alice"); n != 1 {
		t.Fatalf("после фиксации строк %d, ждали 1", n)
	}
}

func TestWithinTxRollsBackOnError(t *testing.T) {
	tr, pool := newTransactor(t)
	errBoom := errors.New("бум")
	err := tr.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := insertUser(ctx, pgtx.Q(ctx, pool), "alice"); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("ждали ошибку fn, получили %v", err)
	}
	if n := usersNamed(t, pool, "alice"); n != 0 {
		t.Fatalf("после ошибки строк %d, ждали откат", n)
	}
}

func TestWithinTxRollsBackOnPanic(t *testing.T) {
	tr, pool := newLeakTolerantTransactor(t)
	type boom struct{}
	func() {
		defer func() {
			if p := recover(); p != (boom{}) {
				t.Fatalf("ждали ту же панику дальше, получили %v", p)
			}
		}()
		_ = tr.WithinTx(t.Context(), func(ctx context.Context) error {
			if err := insertUser(ctx, pgtx.Q(ctx, pool), "alice"); err != nil {
				t.Error(err)
			}
			panic(boom{})
		})
	}()
	if n := usersNamed(t, pool, "alice"); n != 0 {
		t.Fatalf("после паники строк %d, ждали откат", n)
	}
	if n := pool.Stat().AcquiredConns(); n != 0 {
		t.Fatalf("после паники занято соединений: %d — транзакция не вернула его в пул", n)
	}
}

// Клиент ушёл посреди транзакции: ctx отменён, а откат всё равно выполняется, и
// соединение возвращается в пул живым, а не закрывается.
func TestWithinTxRollsBackWithCanceledContext(t *testing.T) {
	tr, pool := newTransactor(t)
	ctx, cancel := context.WithCancel(t.Context())
	err := tr.WithinTx(ctx, func(ctx context.Context) error {
		if err := insertUser(ctx, pgtx.Q(ctx, pool), "alice"); err != nil {
			return err
		}
		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ждали context.Canceled, получили %v", err)
	}
	if n := usersNamed(t, pool, "alice"); n != 0 {
		t.Fatalf("строк %d, ждали откат", n)
	}
	// Подсчёт выше взял соединение из пула. Если откат с отменённым ctx его закрыл,
	// пулу пришлось открыть второе.
	if n := pool.Stat().NewConnsCount(); n != 1 {
		t.Fatalf("пул открыл соединений: %d, ждали 1 — откат закрыл соединение", n)
	}
}

func TestNestedWithinTxReusesOuter(t *testing.T) {
	tr, pool := newTransactor(t)
	errBoom := errors.New("бум")
	var outerXID, innerXID int64
	err := tr.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := pgtx.Q(ctx, pool).QueryRow(ctx, `SELECT txid_current()`).Scan(&outerXID); err != nil {
			return err
		}
		err := tr.WithinTx(ctx, func(ctx context.Context) error {
			if err := pgtx.Q(ctx, pool).QueryRow(ctx, `SELECT txid_current()`).Scan(&innerXID); err != nil {
				return err
			}
			return insertUser(ctx, pgtx.Q(ctx, pool), "alice")
		})
		if err != nil {
			return err
		}
		// Вложенный вызов закончился, но не зафиксировал: фиксирует только внешний.
		if n := usersNamed(t, pool, "alice"); n != 0 {
			t.Errorf("вложенный WithinTx зафиксировал сам: строк снаружи %d", n)
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("ждали ошибку внешнего fn, получили %v", err)
	}
	if outerXID == 0 || innerXID != outerXID {
		t.Fatalf("txid внешней %d, вложенной %d — ждали одну транзакцию", outerXID, innerXID)
	}
	if n := usersNamed(t, pool, "alice"); n != 0 {
		t.Fatalf("откат внешней не откатил вложенную запись: строк %d", n)
	}
}

// runtime.Goexit (так завершает горутину t.Fatal) — не ошибка и не паника, но
// транзакция всё равно откатывается, и соединение возвращается в пул.
func TestWithinTxRollsBackOnGoexit(t *testing.T) {
	tr, pool := newLeakTolerantTransactor(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = tr.WithinTx(t.Context(), func(ctx context.Context) error {
			if err := insertUser(ctx, pgtx.Q(ctx, pool), "alice"); err != nil {
				t.Error(err)
			}
			runtime.Goexit()
			return nil
		})
		t.Error("WithinTx вернулся после runtime.Goexit")
	}()
	<-done
	if n := pool.Stat().AcquiredConns(); n != 0 {
		t.Fatalf("после Goexit занято соединений: %d — транзакция не вернула его в пул", n)
	}
	if n := usersNamed(t, pool, "alice"); n != 0 {
		t.Fatalf("после Goexit строк %d, ждали откат", n)
	}
}

// Ошибка вложенного вызова, возвращённая из внешнего fn, откатывает всю транзакцию:
// и запись вложенного шага, и то, что внешний успел сделать до него.
func TestNestedWithinTxErrorRollsBackOuter(t *testing.T) {
	tr, pool := newTransactor(t)
	errBoom := errors.New("бум")
	err := tr.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := insertUser(ctx, pgtx.Q(ctx, pool), "outer"); err != nil {
			return err
		}
		return tr.WithinTx(ctx, func(ctx context.Context) error {
			if err := insertUser(ctx, pgtx.Q(ctx, pool), "inner"); err != nil {
				return err
			}
			return errBoom
		})
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("ждали ошибку вложенного fn, получили %v", err)
	}
	if n := usersNamed(t, pool, "outer") + usersNamed(t, pool, "inner"); n != 0 {
		t.Fatalf("строк %d, ждали откат всей транзакции", n)
	}
}

// Почему ошибку вложенного вызова нельзя проглатывать: после ошибки Postgres транзакция
// прервана, и следующая команда получает 25P02, а не выполняется.
func TestSwallowedNestedPgErrorLeavesTxAborted(t *testing.T) {
	tr, pool := newTransactor(t)
	err := tr.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := insertUser(ctx, pgtx.Q(ctx, pool), "alice"); err != nil {
			return err
		}
		nestedErr := tr.WithinTx(ctx, func(ctx context.Context) error {
			return insertUser(ctx, pgtx.Q(ctx, pool), "alice") // нарушение UNIQUE (username)
		})
		var pgErr *pgconn.PgError
		if !errors.As(nestedErr, &pgErr) || pgErr.Code != "23505" {
			t.Errorf("вложенный вызов: ждали 23505, получили %v", nestedErr)
		}
		// Ошибку «проглотили» и пишем дальше — сервер отказывает.
		err := insertUser(ctx, pgtx.Q(ctx, pool), "bob")
		if !errors.As(err, &pgErr) || pgErr.Code != "25P02" {
			t.Errorf("после проглоченной ошибки ждали 25P02, получили %v", err)
		}
		return nil
	})
	// COMMIT прерванной транзакции — это откат: pgx сообщает об этом ошибкой.
	if err == nil {
		t.Fatal("ждали ошибку фиксации прерванной транзакции")
	}
	if n := usersNamed(t, pool, "alice"); n != 0 {
		t.Fatalf("строк %d, ждали откат", n)
	}
}

func TestQOutsideTxUsesPool(t *testing.T) {
	tr, pool := newTransactor(t)
	if q, ok := pgtx.Q(t.Context(), pool).(*pgxpool.Pool); !ok || q != pool {
		t.Fatalf("вне транзакции Q вернул %T, ждали тот же пул", q)
	}
	// Запрос через пул — своя неявная транзакция: результат виден сразу.
	if err := insertUser(t.Context(), pgtx.Q(t.Context(), pool), "alice"); err != nil {
		t.Fatal(err)
	}
	if n := usersNamed(t, pool, "alice"); n != 1 {
		t.Fatalf("строк %d, ждали 1", n)
	}
	err := tr.WithinTx(t.Context(), func(ctx context.Context) error {
		if _, isPool := pgtx.Q(ctx, pool).(*pgxpool.Pool); isPool {
			t.Error("внутри WithinTx Q вернул пул, а не транзакцию")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
