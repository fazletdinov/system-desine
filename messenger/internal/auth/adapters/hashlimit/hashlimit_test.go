package hashlimit_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"

	"messenger/internal/auth/adapters/hashlimit"
	"messenger/internal/auth/domain"
	"messenger/internal/auth/usecase"
)

// fakeHasher — «хеш», который держит каждый вызов, пока тест не закроет release:
// так видно, сколько вызовов одновременно внутри. Канал создаётся внутри пузыря
// synctest, поэтому ожидание на нём — «устойчивая блокировка», и synctest.Wait
// возвращается, когда все вызовы дошли до своего места.
type fakeHasher struct {
	release chan struct{}

	mu        sync.Mutex
	active    int
	peak      int
	calls     int
	panicNext bool
}

func newFake() *fakeHasher { return &fakeHasher{release: make(chan struct{})} }

func (f *fakeHasher) enter() (panics bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.active++
	f.peak = max(f.peak, f.active)
	panics, f.panicNext = f.panicNext, false
	return panics
}

func (f *fakeHasher) leave() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active--
}

func (f *fakeHasher) work() {
	panics := f.enter()
	defer f.leave()
	if panics {
		panic("сбой внутри хешера")
	}
	<-f.release
}

func (f *fakeHasher) Hash(_ context.Context, _ domain.Password) (domain.PasswordHash, error) {
	f.work()
	return domain.NewPasswordHash("h")
}

func (f *fakeHasher) Verify(_ context.Context, _ string, _ domain.PasswordHash) (bool, error) {
	f.work()
	return true, nil
}

func (f *fakeHasher) stats() (active, peak, calls int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active, f.peak, f.calls
}

func newLimiter(t *testing.T, next usecase.PasswordHasher, concurrency, queue int) *hashlimit.Limiter {
	t.Helper()
	l, err := hashlimit.New(next, concurrency, queue)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func password(t *testing.T) domain.Password {
	t.Helper()
	p, err := domain.NewPassword("правильный-пароль-длинный")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// goHash запускает Hash в горутине; ошибка придёт в буферизованный канал, так что
// горутина не зависнет, даже если тест её не прочитает.
func goHash(ctx context.Context, l *hashlimit.Limiter, p domain.Password) <-chan error {
	errc := make(chan error, 1)
	go func() {
		_, err := l.Hash(ctx, p)
		errc <- err
	}()
	return errc
}

// done — вернулся ли вызов к этому моменту (после synctest.Wait — без гонки).
func done(errc <-chan error) (bool, error) {
	select {
	case err := <-errc:
		return true, err
	default:
		return false, nil
	}
}

func TestLimiterRunsAtMostN(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := newFake()
		l := newLimiter(t, fake, 2, 4)
		pw := password(t)
		var calls []<-chan error
		for range 6 {
			calls = append(calls, goHash(t.Context(), l, pw))
		}
		synctest.Wait()
		if active, _, _ := fake.stats(); active != 2 {
			t.Fatalf("внутри хешера %d вызовов, ждали ровно 2: остальные 4 — в очереди", active)
		}
		close(fake.release)
		for i, errc := range calls {
			if err := <-errc; err != nil {
				t.Fatalf("вызов %d: %v", i, err)
			}
		}
		if _, peak, calls := fake.stats(); peak != 2 || calls != 6 {
			t.Fatalf("пик %d (ждали 2), вызовов %d (ждали 6)", peak, calls)
		}
	})
}

func TestLimiterRejectsOverQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := newFake()
		l := newLimiter(t, fake, 2, 4)
		pw := password(t)
		for range 6 { // 2 считают, 4 ждут — мест больше нет
			goHash(t.Context(), l, pw)
		}
		synctest.Wait()
		seventh := goHash(t.Context(), l, pw)
		synctest.Wait()
		// Хешер ещё никого не отпустил, а седьмой уже получил ответ: отказ — сразу,
		// без ожидания в очереди.
		returned, err := done(seventh)
		if !returned {
			t.Fatal("седьмой вызов ждёт, хотя очередь полна")
		}
		if !errors.Is(err, usecase.ErrOverloaded) {
			t.Fatalf("ждали ErrOverloaded, получили %v", err)
		}
		close(fake.release)
	})
}

func TestLimiterZeroQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := newFake()
		l := newLimiter(t, fake, 1, 0)
		pw := password(t)
		first := goHash(t.Context(), l, pw)
		synctest.Wait()
		if _, err := l.Hash(t.Context(), pw); !errors.Is(err, usecase.ErrOverloaded) {
			t.Fatalf("очередь 0: ждали ErrOverloaded, получили %v", err)
		}
		close(fake.release)
		if err := <-first; err != nil {
			t.Fatal(err)
		}
	})
}

// #region cancel-test
func TestLimiterCancelReleasesSlot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := newFake()
		l := newLimiter(t, fake, 1, 1)
		pw := password(t)
		first := goHash(t.Context(), l, pw) // считает
		synctest.Wait()
		ctx, cancel := context.WithCancel(t.Context())
		second := goHash(ctx, l, pw) // ждёт в очереди
		synctest.Wait()

		cancel() // клиент ушёл или истёк срок запроса
		synctest.Wait()
		returned, err := done(second)
		if !returned || !errors.Is(err, context.Canceled) {
			t.Fatalf("ждали context.Canceled сразу после отмены, получили %v (вернулся: %t)", err, returned)
		}

		// Место в очереди освободилось: третий встаёт в очередь, а не получает отказ.
		third := goHash(t.Context(), l, pw)
		synctest.Wait()
		if returned, err := done(third); returned {
			t.Fatalf("третий вернулся, не дождавшись места: %v — место отменённого утекло", err)
		}
		close(fake.release)
		for _, errc := range []<-chan error{first, third} {
			if err := <-errc; err != nil {
				t.Fatal(err)
			}
		}
		if _, _, calls := fake.stats(); calls != 2 {
			t.Fatalf("хешер вызван %d раз, ждали 2: отменённый вызов не должен до него дойти", calls)
		}
	})
}

// #endregion cancel-test

// Паника в хешере обычно роняет процесс, но если её перехватят выше (middleware.Recover
// перехватывает), ограничитель не должен навсегда потерять место.
func TestLimiterReleasesOnPanic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := newFake()
		fake.panicNext = true
		l := newLimiter(t, fake, 1, 0)
		pw := password(t)
		recovered := make(chan any, 1)
		go func() {
			defer func() { recovered <- recover() }()
			_, _ = l.Hash(t.Context(), pw)
		}()
		if r := <-recovered; r == nil {
			t.Fatal("ждали панику из хешера")
		}
		next := goHash(t.Context(), l, pw)
		synctest.Wait()
		if returned, err := done(next); returned {
			t.Fatalf("после паники ограничитель отказал: %v — место не освободилось", err)
		}
		close(fake.release)
		if err := <-next; err != nil {
			t.Fatal(err)
		}
	})
}

// Вход (Verify) стоит столько же, сколько регистрация, и места у них общие:
// перебор паролей не должен обходить ограничение.
func TestLimiterVerifyLimitedToo(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := newFake()
		l := newLimiter(t, fake, 1, 0)
		pw := password(t)
		h, err := domain.NewPasswordHash("h")
		if err != nil {
			t.Fatal(err)
		}
		first := goHash(t.Context(), l, pw)
		synctest.Wait()
		if _, err := l.Verify(t.Context(), "пароль", h); !errors.Is(err, usecase.ErrOverloaded) {
			t.Fatalf("Verify при занятом месте: ждали ErrOverloaded, получили %v", err)
		}
		close(fake.release)
		if err := <-first; err != nil {
			t.Fatal(err)
		}
		if ok, err := l.Verify(t.Context(), "пароль", h); err != nil || !ok {
			t.Fatalf("Verify на свободном месте: %v, %v", ok, err)
		}
	})
}

func TestNewRejectsBadArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		next               usecase.PasswordHasher
		concurrency, queue int
	}{
		{"нет хешера", nil, 1, 0},
		{"concurrency 0", newFake(), 0, 4},
		{"concurrency < 0", newFake(), -1, 4},
		{"queue < 0", newFake(), 1, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := hashlimit.New(tt.next, tt.concurrency, tt.queue); err == nil {
				t.Fatalf("New(%d, %d): ждали ошибку", tt.concurrency, tt.queue)
			}
		})
	}
}
