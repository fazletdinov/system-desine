// Package storetest — контрактные тесты хранилища ключей идемпотентности: одна батарея
// проверок для любой реализации idempotency.Store (сейчас — в памяти, в модуле 2a — Postgres).
package storetest

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"messenger/internal/platform/idempotency"
)

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// Run прогоняет контракт против хранилища, которое создаёт newStore. Каждая проверка
// получает своё, пустое хранилище.
func Run(t *testing.T, newStore func(t *testing.T) idempotency.Store) {
	t.Helper()
	tests := []struct {
		name string
		fn   func(t *testing.T, s idempotency.Store)
	}{
		{"новый ключ занимается", testReserveNew},
		{"занятый ключ — существующая запись", testReserveExisting},
		{"области не пересекаются", testScopes},
		{"готовый ответ виден повтору", testComplete},
		{"освобождённый ключ занимается снова", testRelease},
		{"просроченная запись — как её нет", testExpired},
		{"одновременный Reserve", testConcurrentReserve},
		{"запись — копия", testReturnsCopy},
		{"отменённый контекст", testCanceledContext},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.fn(t, newStore(t))
		})
	}
}

func fp(s string) [sha256.Size]byte { return sha256.Sum256([]byte(s)) }

// pending — запись «в работе», какой её создаёт middleware.
func pending(body string) idempotency.Record {
	return idempotency.Record{Fingerprint: fp(body), ExpiresAt: t0.Add(idempotency.TTL)}
}

func reserve(t *testing.T, s idempotency.Store, scope, key string, rec idempotency.Record, now time.Time) (idempotency.Record, bool) {
	t.Helper()
	got, created, err := s.Reserve(context.Background(), scope, key, rec, now)
	if err != nil {
		t.Fatal(err)
	}
	return got, created
}

func testReserveNew(t *testing.T, s idempotency.Store) {
	got, created := reserve(t, s, "u1", "k", pending("A"), t0)
	if !created || got.Done || got.Fingerprint != fp("A") {
		t.Fatalf("новый ключ: created=%v, запись %+v", created, got)
	}
}

func testReserveExisting(t *testing.T, s idempotency.Store) {
	reserve(t, s, "u1", "k", pending("A"), t0)
	got, created := reserve(t, s, "u1", "k", pending("B"), t0.Add(time.Second))
	if created || got.Fingerprint != fp("A") || got.Done {
		t.Fatalf("занятый ключ: created=%v, отпечаток первого запроса — %v", created, got.Fingerprint == fp("A"))
	}
}

func testScopes(t *testing.T, s idempotency.Store) {
	reserve(t, s, "u1", "k", pending("A"), t0)
	if _, created := reserve(t, s, "u2", "k", pending("A"), t0); !created {
		t.Fatal("ключ другого пользователя занял чужую запись")
	}
}

func complete(t *testing.T, s idempotency.Store, scope, key string) idempotency.Record {
	t.Helper()
	done := pending("A")
	done.Done, done.Status, done.Body = true, http.StatusCreated, []byte(`{"id":"1"}`)
	done.Header = http.Header{"Location": {"/v1/chats/1"}}
	if err := s.Complete(context.Background(), scope, key, done); err != nil {
		t.Fatal(err)
	}
	return done
}

func testComplete(t *testing.T, s idempotency.Store) {
	reserve(t, s, "u1", "k", pending("A"), t0)
	want := complete(t, s, "u1", "k")
	got, created := reserve(t, s, "u1", "k", pending("A"), t0.Add(time.Minute))
	if created || !got.Done || got.Status != want.Status || string(got.Body) != string(want.Body) || got.Header.Get("Location") != "/v1/chats/1" {
		t.Fatalf("после Complete: created=%v, запись %+v", created, got)
	}
}

func testRelease(t *testing.T, s idempotency.Store) {
	reserve(t, s, "u1", "k", pending("A"), t0)
	if err := s.Release(context.Background(), "u1", "k"); err != nil {
		t.Fatal(err)
	}
	if _, created := reserve(t, s, "u1", "k", pending("A"), t0); !created {
		t.Fatal("после Release ключ должен заниматься заново")
	}
	// Release незанятого ключа — не ошибка: освобождать нечего.
	if err := s.Release(context.Background(), "u1", "нет такого"); err != nil {
		t.Fatalf("Release незанятого ключа: %v", err)
	}
}

func testExpired(t *testing.T, s idempotency.Store) {
	reserve(t, s, "u1", "k", pending("A"), t0)
	complete(t, s, "u1", "k")
	expiry := t0.Add(idempotency.TTL)
	if _, created := reserve(t, s, "u1", "k", pending("B"), expiry.Add(-time.Nanosecond)); created {
		t.Fatal("за наносекунду до ExpiresAt запись ещё жива")
	}
	got, created := reserve(t, s, "u1", "k", pending("B"), expiry)
	if !created || got.Done || got.Fingerprint != fp("B") {
		t.Fatalf("в момент ExpiresAt запись просрочена: created=%v, запись %+v", created, got)
	}
}

// Гонка за ключ: из одновременных Reserve ровно один занимает ключ, остальные видят его запись.
func testConcurrentReserve(t *testing.T, s idempotency.Store) {
	const n = 50
	var (
		wg       sync.WaitGroup
		won      atomic.Int32
		errs     atomic.Int32
		mismatch atomic.Int32
	)
	start := make(chan struct{})
	for range n {
		wg.Go(func() {
			<-start
			got, created, err := s.Reserve(context.Background(), "u1", "k", pending("A"), t0)
			switch {
			case err != nil:
				errs.Add(1)
			case created:
				won.Add(1)
			case got.Fingerprint != fp("A") || got.Done:
				mismatch.Add(1)
			}
		})
	}
	close(start)
	wg.Wait()
	if won.Load() != 1 || errs.Load() != 0 || mismatch.Load() != 0 {
		t.Fatalf("заняли ключ %d раз (ждали 1), ошибок %d, чужих записей %d", won.Load(), errs.Load(), mismatch.Load())
	}
}

// Запись хранит срез и карту: правка возвращённой записи не должна менять хранилище.
func testReturnsCopy(t *testing.T, s idempotency.Store) {
	reserve(t, s, "u1", "k", pending("A"), t0)
	complete(t, s, "u1", "k")
	got, _ := reserve(t, s, "u1", "k", pending("A"), t0)
	got.Body[0] = 'X'
	got.Header.Set("Location", "/чужой")
	again, _ := reserve(t, s, "u1", "k", pending("A"), t0)
	if string(again.Body) != `{"id":"1"}` || again.Header.Get("Location") != "/v1/chats/1" {
		t.Fatalf("хранилище изменилось через возвращённую запись: %s, %v", again.Body, again.Header)
	}
}

func testCanceledContext(t *testing.T, s idempotency.Store) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := s.Reserve(ctx, "u1", "k", pending("A"), t0); !errors.Is(err, context.Canceled) {
		t.Errorf("Reserve: ждали context.Canceled, получили %v", err)
	}
	if err := s.Complete(ctx, "u1", "k", pending("A")); !errors.Is(err, context.Canceled) {
		t.Errorf("Complete: ждали context.Canceled, получили %v", err)
	}
	if err := s.Release(ctx, "u1", "k"); !errors.Is(err, context.Canceled) {
		t.Errorf("Release: ждали context.Canceled, получили %v", err)
	}
	// Отменённые вызовы ничего не записали: ключ свободен.
	if _, created := reserve(t, s, "u1", "k", pending("A"), t0); !created {
		t.Fatal("отменённый вызов занял ключ")
	}
}
