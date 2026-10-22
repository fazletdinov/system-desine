// Package storetest — контрактные тесты хранилища ключей идемпотентности: одна батарея
// проверок для любой реализации idempotency.Store (в памяти и Postgres).
package storetest

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"messenger/internal/platform/idempotency"
)

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// ReservationTTL — срок резервации, с которым newStore обязан создать хранилище: проверки
// перехвата зависшей резервации считают время от него.
const ReservationTTL = 15 * time.Second

// Run прогоняет контракт против хранилища, которое создаёт newStore. Каждая проверка
// получает своё, пустое хранилище. Идти ли проверкам параллельно, решает newStore
// (t.Parallel): независимые хранилища в памяти — да, а Postgres-хранилище делит базу
// тестового пакета, которую каждая проверка восстанавливает из снимка, — нет.
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
		{"зависшая резервация перехватывается", testReserveTakesOverStaleInProgress},
		{"свежая резервация не перехватывается", testReserveKeepsFreshInProgress},
		{"готовый ответ не перехватывается", testReserveKeepsDone},
		{"одновременный перехват", testConcurrentTakeOver},
		{"чужой Release не снимает резервацию", testReleaseForeignOwner},
		{"чужой Complete — ErrReservationLost", testCompleteForeignOwner},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.fn(t, newStore(t)) })
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

// done — готовый ответ на резервацию reserved, какой его сохраняет middleware.
func done(reserved idempotency.Record) idempotency.Record {
	reserved.Done, reserved.Status, reserved.Body = true, http.StatusCreated, []byte(`{"id":"1"}`)
	reserved.Header = http.Header{"Location": {"/v1/chats/1"}}
	return reserved
}

// complete сохраняет готовый ответ на резервацию reserved (её вернул Reserve).
func complete(t *testing.T, s idempotency.Store, scope, key string, reserved idempotency.Record) idempotency.Record {
	t.Helper()
	rec := done(reserved)
	if err := s.Complete(context.Background(), scope, key, rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

// reserveDone занимает ключ и сразу сохраняет ответ: запись, которую Reserve не перехватит.
func reserveDone(t *testing.T, s idempotency.Store, scope, key string, rec idempotency.Record, now time.Time) {
	t.Helper()
	reserved, created := reserve(t, s, scope, key, rec, now)
	if !created {
		t.Fatalf("ключ %s уже занят", key)
	}
	complete(t, s, scope, key, reserved)
}

func testComplete(t *testing.T, s idempotency.Store) {
	reserved, _ := reserve(t, s, "u1", "k", pending("A"), t0)
	want := complete(t, s, "u1", "k", reserved)
	got, created := reserve(t, s, "u1", "k", pending("A"), t0.Add(time.Minute))
	if created || !got.Done || got.Status != want.Status || string(got.Body) != string(want.Body) || got.Header.Get("Location") != "/v1/chats/1" {
		t.Fatalf("после Complete: created=%v, запись %+v", created, got)
	}
}

func testRelease(t *testing.T, s idempotency.Store) {
	reserved, _ := reserve(t, s, "u1", "k", pending("A"), t0)
	if err := s.Release(context.Background(), "u1", "k", reserved.Owner); err != nil {
		t.Fatal(err)
	}
	if _, created := reserve(t, s, "u1", "k", pending("A"), t0); !created {
		t.Fatal("после Release ключ должен заниматься заново")
	}
	// Release незанятого ключа — не ошибка: освобождать нечего.
	if err := s.Release(context.Background(), "u1", "нет такого", reserved.Owner); err != nil {
		t.Fatalf("Release незанятого ключа: %v", err)
	}
}

func testExpired(t *testing.T, s idempotency.Store) {
	reserveDone(t, s, "u1", "k", pending("A"), t0)
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
	reserveDone(t, s, "u1", "k", pending("A"), t0)
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
	if err := s.Release(ctx, "u1", "k", uuid.New()); !errors.Is(err, context.Canceled) {
		t.Errorf("Release: ждали context.Canceled, получили %v", err)
	}
	// Отменённые вызовы ничего не записали: ключ свободен.
	if _, created := reserve(t, s, "u1", "k", pending("A"), t0); !created {
		t.Fatal("отменённый вызов занял ключ")
	}
}

// Резервация, ответ на которую так и не сохранили (процесс упал между операцией и
// Complete), держит ключ ReservationTTL, а не сутки: в момент срока повтор её перехватывает
// — с новым владельцем и своим отпечатком.
func testReserveTakesOverStaleInProgress(t *testing.T, s idempotency.Store) {
	stale, _ := reserve(t, s, "u1", "k", pending("A"), t0)
	got, created := reserve(t, s, "u1", "k", pending("B"), t0.Add(ReservationTTL))
	if !created || got.Done || got.Fingerprint != fp("B") {
		t.Fatalf("зависшая резервация: created=%v, запись %+v", created, got)
	}
	if got.Owner == stale.Owner {
		t.Fatal("перехват не сменил владельца резервации")
	}
	// Новый владелец сохраняет ответ как обычно, и повтор видит именно его.
	complete(t, s, "u1", "k", got)
	again, created := reserve(t, s, "u1", "k", pending("B"), t0.Add(ReservationTTL+time.Second))
	if created || !again.Done || again.Fingerprint != fp("B") {
		t.Fatalf("после перехвата и Complete: created=%v, запись %+v", created, again)
	}
}

func testReserveKeepsFreshInProgress(t *testing.T, s idempotency.Store) {
	first, _ := reserve(t, s, "u1", "k", pending("A"), t0)
	got, created := reserve(t, s, "u1", "k", pending("A"), t0.Add(ReservationTTL-time.Microsecond))
	if created || got.Done || got.Owner != first.Owner {
		t.Fatalf("до срока резервации: created=%v, запись %+v", created, got)
	}
}

// Срок резервации касается только записей «в работе»: готовый ответ живёт до ExpiresAt.
func testReserveKeepsDone(t *testing.T, s idempotency.Store) {
	reserveDone(t, s, "u1", "k", pending("A"), t0)
	got, created := reserve(t, s, "u1", "k", pending("A"), t0.Add(ReservationTTL+time.Hour))
	if created || !got.Done {
		t.Fatalf("готовый ответ после срока резервации: created=%v, запись %+v", created, got)
	}
}

// Гонка за зависшую резервацию: перехватывает ровно один из повторов.
func testConcurrentTakeOver(t *testing.T, s idempotency.Store) {
	reserve(t, s, "u1", "k", pending("A"), t0)
	const n = 20
	var (
		wg        sync.WaitGroup
		won, errs atomic.Int32
	)
	start := make(chan struct{})
	for range n {
		wg.Go(func() {
			<-start
			_, created, err := s.Reserve(context.Background(), "u1", "k", pending("B"), t0.Add(ReservationTTL))
			switch {
			case err != nil:
				errs.Add(1)
			case created:
				won.Add(1)
			}
		})
	}
	close(start)
	wg.Wait()
	if won.Load() != 1 || errs.Load() != 0 {
		t.Fatalf("перехватили %d раз (ждали 1), ошибок %d", won.Load(), errs.Load())
	}
}

// Запоздавший прежний владелец (его запрос шёл дольше резервации) освобождает ключ после
// сбоя — и не должен снять резервацию того, кто её перехватил.
func testReleaseForeignOwner(t *testing.T, s idempotency.Store) {
	stale, _ := reserve(t, s, "u1", "k", pending("A"), t0)
	now := t0.Add(ReservationTTL)
	current, _ := reserve(t, s, "u1", "k", pending("A"), now)
	if err := s.Release(context.Background(), "u1", "k", stale.Owner); err != nil {
		t.Fatalf("Release чужой резервации: %v", err)
	}
	got, created := reserve(t, s, "u1", "k", pending("A"), now)
	if created || got.Owner != current.Owner {
		t.Fatalf("чужой Release снял резервацию: created=%v, владелец сменился %v", created, got.Owner != current.Owner)
	}
}

func testCompleteForeignOwner(t *testing.T, s idempotency.Store) {
	stale, _ := reserve(t, s, "u1", "k", pending("A"), t0)
	now := t0.Add(ReservationTTL)
	current, _ := reserve(t, s, "u1", "k", pending("A"), now)
	if err := s.Complete(context.Background(), "u1", "k", done(stale)); !errors.Is(err, idempotency.ErrReservationLost) {
		t.Fatalf("Complete перехваченной резервации: ждали ErrReservationLost, получили %v", err)
	}
	got, _ := reserve(t, s, "u1", "k", pending("A"), now)
	if got.Done || got.Owner != current.Owner {
		t.Fatalf("чужой Complete перезаписал запись: %+v", got)
	}
	// Ключ, которого нет (освобождён или удалён очисткой), — тоже потерянная резервация.
	if err := s.Complete(context.Background(), "u1", "нет такого", done(stale)); !errors.Is(err, idempotency.ErrReservationLost) {
		t.Fatalf("Complete незанятого ключа: ждали ErrReservationLost, получили %v", err)
	}
}

// SweepStore — хранилище, которое умеет и ключи, и фоновую очистку. Псевдоним, а не
// новый тип: подходит и функция, возвращающая interface{ Store; Sweeper }.
type SweepStore = interface {
	idempotency.Store
	idempotency.Sweeper
}

// RunSweeper прогоняет контракт очистки. Каждая проверка получает своё, пустое хранилище;
// параллельность, как и в Run, решает newStore.
func RunSweeper(t *testing.T, newStore func(t *testing.T) SweepStore) {
	t.Helper()
	tests := []struct {
		name string
		fn   func(t *testing.T, s SweepStore)
	}{
		{"удаляет только просроченные", testSweepExpiredOnly},
		{"запись «в работе» живёт до TTL", testSweepPending},
		{"повторная очистка ничего не удаляет", testSweepRepeat},
		{"много просроченных — все удалены", testSweepMany},
		{"отменённый контекст", testSweepCanceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.fn(t, newStore(t)) })
	}
}

func sweep(t *testing.T, s idempotency.Sweeper, now time.Time) int {
	t.Helper()
	removed, err := s.Sweep(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	return removed
}

// alive — запись по ключу ещё есть: Reserve её не занимает, а возвращает. Проверять так
// можно только готовые записи или «в работе» до срока резервации — зависшую Reserve перехватит.
func alive(t *testing.T, s idempotency.Store, key string, now time.Time) bool {
	t.Helper()
	_, created := reserve(t, s, "u1", key, pending("probe"), now)
	return !created
}

func testSweepExpiredOnly(t *testing.T, s SweepStore) {
	reserveDone(t, s, "u1", "old", pending("A"), t0)
	later := t0.Add(time.Hour)
	fresh := pending("B")
	fresh.ExpiresAt = later.Add(idempotency.TTL)
	reserveDone(t, s, "u1", "fresh", fresh, later)
	expiry := t0.Add(idempotency.TTL)

	if n := sweep(t, s, expiry.Add(-time.Nanosecond)); n != 0 {
		t.Fatalf("за наносекунду до ExpiresAt удалено %d записей", n)
	}
	// Граница включительна — как у Reserve: в момент ExpiresAt запись просрочена.
	if n := sweep(t, s, expiry); n != 1 {
		t.Fatalf("в момент ExpiresAt удалено %d записей, ждали 1", n)
	}
	if !alive(t, s, "fresh", expiry) {
		t.Fatal("очистка удалила живую запись")
	}
}

func testSweepPending(t *testing.T, s SweepStore) {
	reserve(t, s, "u1", "k", pending("A"), t0) // Reserve без Complete — «в работе»
	if n := sweep(t, s, t0.Add(idempotency.TTL-time.Second)); n != 0 {
		t.Fatalf("до TTL удалено %d записей «в работе»", n)
	}
	if !alive(t, s, "k", t0) {
		t.Fatal("запись «в работе» пропала до TTL")
	}
	// После TTL «в работе» удаляется так же, как готовая: Reserve её всё равно перезаписал бы.
	if n := sweep(t, s, t0.Add(idempotency.TTL)); n != 1 {
		t.Fatalf("после TTL удалено %d записей «в работе», ждали 1", n)
	}
}

func testSweepRepeat(t *testing.T, s SweepStore) {
	reserve(t, s, "u1", "k", pending("A"), t0)
	expiry := t0.Add(idempotency.TTL)
	if n := sweep(t, s, expiry); n != 1 {
		t.Fatalf("удалено %d, ждали 1", n)
	}
	if n := sweep(t, s, expiry); n != 0 {
		t.Fatalf("повторная очистка удалила %d записей", n)
	}
}

// Записей больше, чем умещается в одну порцию любой разумной реализации.
func testSweepMany(t *testing.T, s SweepStore) {
	const expired, live = 2500, 300
	for i := range expired {
		reserve(t, s, "u1", fmt.Sprintf("old-%d", i), pending("A"), t0)
	}
	fresh := pending("B")
	fresh.ExpiresAt = t0.Add(2 * idempotency.TTL)
	for i := range live {
		reserveDone(t, s, "u1", fmt.Sprintf("live-%d", i), fresh, t0)
	}
	now := t0.Add(idempotency.TTL)
	if n := sweep(t, s, now); n != expired {
		t.Fatalf("удалено %d, ждали %d", n, expired)
	}
	for i := range live {
		if !alive(t, s, fmt.Sprintf("live-%d", i), now) {
			t.Fatalf("живая запись live-%d удалена", i)
		}
	}
}

func testSweepCanceled(t *testing.T, s SweepStore) {
	reserve(t, s, "u1", "k", pending("A"), t0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Sweep(ctx, t0.Add(idempotency.TTL)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Sweep: ждали context.Canceled, получили %v", err)
	}
	// Отменённый вызов ничего не удалил.
	if n := sweep(t, s, t0.Add(idempotency.TTL)); n != 1 {
		t.Fatalf("после отменённого Sweep удалено %d, ждали 1", n)
	}
}
