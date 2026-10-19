package lifecycle_test

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"messenger/internal/platform/lifecycle"
)

// journal — общий журнал событий компонентов: по нему видно, в каком порядке они
// останавливались. Пишут его горутины компонентов, читает тест.
type journal struct {
	mu     sync.Mutex
	events []string
}

func (j *journal) add(e string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.events = append(j.events, e)
}

func (j *journal) all() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return slices.Clone(j.events)
}

// fake — компонент с управляемым поведением: run получает ctx компонента и журнал.
type fake struct {
	name string
	run  func(ctx context.Context) error
}

func (f fake) Name() string                  { return f.name }
func (f fake) Run(ctx context.Context) error { return f.run(ctx) }

// drainDelay — сколько фейковый HTTP-сервер «дренирует» после отмены: виртуальное
// время synctest, тест не ждёт его по-настоящему.
const drainDelay = 5 * time.Second

// httpFake ведёт себя как HTTP-сервер из 1.17: после отмены ещё drainDelay
// обслуживает запросы и только потом возвращает nil.
func httpFake(j *journal) fake {
	return fake{name: "http", run: func(ctx context.Context) error {
		<-ctx.Done()
		j.add("http: отмена")
		<-time.After(drainDelay)
		j.add("http: дренаж завершён")
		return nil
	}}
}

// backFake — фоновый компонент: работает до отмены и выходит с nil.
func backFake(name string, j *journal) fake {
	return fake{name: name, run: func(ctx context.Context) error {
		<-ctx.Done()
		j.add(name + ": отмена")
		return nil
	}}
}

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// runAsync запускает lifecycle.Run в горутине пузыря; результат — в канале.
func runAsync(ctx context.Context, s lifecycle.Stages) <-chan error {
	done := make(chan error, 1)
	go func() { done <- lifecycle.Run(ctx, discard(), s) }()
	return done
}

func TestRunStopsFrontBeforeBack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		j := &journal{}
		ctx, cancel := context.WithCancel(t.Context())
		done := runAsync(ctx, lifecycle.Stages{
			Front: []lifecycle.Component{httpFake(j)},
			Back:  []lifecycle.Component{backFake("sweeper", j), backFake("janitor", j)},
		})
		synctest.Wait()

		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Run: %v, want nil", err)
		}
		events := j.all()
		drained := slices.Index(events, "http: дренаж завершён")
		for _, name := range []string{"sweeper", "janitor"} {
			if i := slices.Index(events, name+": отмена"); i < drained || drained < 0 {
				t.Fatalf("%s получил отмену раньше, чем HTTP закончил дренаж: %v", name, events)
			}
		}
	})
}

// #region back-failure-test
func TestRunBackFailureStopsFrontWithDrain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		j := &journal{}
		boom := errors.New("boom")
		failing := fake{name: "sweeper", run: func(context.Context) error {
			<-time.After(3 * time.Second)
			return boom
		}}
		start := time.Now()

		err := lifecycle.Run(t.Context(), discard(), lifecycle.Stages{
			Front: []lifecycle.Component{httpFake(j)},
			Back:  []lifecycle.Component{failing, backFake("janitor", j)},
		})

		if !errors.Is(err, boom) || !strings.Contains(err.Error(), "sweeper") {
			t.Fatalf("Run: %v, want boom с именем компонента", err)
		}
		// Сбой фонового компонента останавливает процесс так же, как сигнал: HTTP
		// проходит полный дренаж, и только потом останавливаются остальные Back.
		want := []string{"http: отмена", "http: дренаж завершён", "janitor: отмена"}
		if got := j.all(); !slices.Equal(got, want) {
			t.Fatalf("журнал %v, ждали %v", got, want)
		}
		if elapsed := time.Since(start); elapsed != 3*time.Second+drainDelay {
			t.Fatalf("Run вернулся через %v, ждали сбой + дренаж = %v", elapsed, 3*time.Second+drainDelay)
		}
	})
}

// #endregion back-failure-test

func TestRunFrontFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		j := &journal{}
		listenErr := errors.New("порт занят")
		front := fake{name: "http", run: func(context.Context) error { return listenErr }}

		err := lifecycle.Run(t.Context(), discard(), lifecycle.Stages{
			Front: []lifecycle.Component{front},
			Back:  []lifecycle.Component{backFake("sweeper", j)},
		})

		if !errors.Is(err, listenErr) || !strings.Contains(err.Error(), "http") {
			t.Fatalf("Run: %v, want ошибку с именем http", err)
		}
		if got := j.all(); !slices.Equal(got, []string{"sweeper: отмена"}) {
			t.Fatalf("журнал %v: Back не получил отмену", got)
		}
	})
}

func TestRunBackStopTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		j := &journal{}
		// stuck игнорирует отмену; release отпускает его после проверки, чтобы
		// пузырь synctest завершился без живых горутин.
		release := make(chan struct{})
		stuck := fake{name: "stuck", run: func(context.Context) error {
			<-release
			return nil
		}}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		start := time.Now()

		err := lifecycle.Run(ctx, discard(), lifecycle.Stages{
			Front: []lifecycle.Component{backFake("http", j)},
			Back:  []lifecycle.Component{stuck, backFake("sweeper", j)},
		})

		if err == nil || !strings.Contains(err.Error(), "stuck") || !strings.Contains(err.Error(), "2s") {
			t.Fatalf("Run: %v, want ошибку с именем stuck и сроком 2s", err)
		}
		if strings.Contains(err.Error(), "sweeper") {
			t.Fatalf("Run: %v — sweeper остановился вовремя, его в ошибке быть не должно", err)
		}
		if elapsed := time.Since(start); elapsed != lifecycle.BackStopTimeout {
			t.Fatalf("Run вернулся через %v, ждали BackStopTimeout = %v", elapsed, lifecycle.BackStopTimeout)
		}
		close(release)
	})
}

func TestRunNilAfterCleanStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		j := &journal{}
		ctx, cancel := context.WithCancel(t.Context())
		done := runAsync(ctx, lifecycle.Stages{
			Front: []lifecycle.Component{backFake("http", j)},
			Back:  []lifecycle.Component{backFake("sweeper", j)},
		})
		synctest.Wait()

		cancel()

		// ctx.Err() != nil, но отмена — штатная остановка, а не сбой.
		if err := <-done; err != nil {
			t.Fatalf("Run: %v, want nil", err)
		}
		if ctx.Err() == nil {
			t.Fatal("ctx не отменён — тест проверяет не то")
		}
	})
}

// Компонент, вернувший context.Canceled после своей отмены (идиома «return
// ctx.Err()»), остановился штатно; та же ошибка до отмены — сбой.
func TestRunCanceledAfterStopIsClean(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		idiom := fake{name: "sweeper", run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}}
		ctx, cancel := context.WithCancel(t.Context())
		done := runAsync(ctx, lifecycle.Stages{Front: []lifecycle.Component{idiom}, Back: []lifecycle.Component{idiom}})
		synctest.Wait()

		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Run: %v, want nil — context.Canceled после отмены не сбой", err)
		}
	})
	synctest.Test(t, func(t *testing.T) {
		early := fake{name: "sweeper", run: func(context.Context) error { return context.Canceled }}

		err := lifecycle.Run(t.Context(), discard(), lifecycle.Stages{Back: []lifecycle.Component{early}})

		if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "sweeper") {
			t.Fatalf("Run: %v, want сбой sweeper — его ctx не отменяли", err)
		}
	})
}

// Без Front фоновые компоненты всё равно работают до отмены ctx, а не
// останавливаются сразу «после выхода всех Front».
func TestRunWithoutFrontRunsBackUntilCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		j := &journal{}
		ctx, cancel := context.WithCancel(t.Context())
		done := runAsync(ctx, lifecycle.Stages{Back: []lifecycle.Component{backFake("sweeper", j)}})

		synctest.Sleep(time.Hour)
		if got := j.all(); len(got) != 0 {
			t.Fatalf("Back остановлен без отмены: %v", got)
		}
		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Run: %v", err)
		}
		if got := j.all(); !slices.Equal(got, []string{"sweeper: отмена"}) {
			t.Fatalf("журнал %v", got)
		}
	})
}

// Компонент, который вышел сам до отмены, перестал делать свою работу молча:
// это сбой, а не штатная остановка.
func TestRunEarlyReturnIsFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		j := &journal{}
		quitter := fake{name: "sweeper", run: func(context.Context) error { return nil }}

		err := lifecycle.Run(t.Context(), discard(), lifecycle.Stages{
			Front: []lifecycle.Component{httpFake(j)},
			Back:  []lifecycle.Component{quitter},
		})

		if !errors.Is(err, lifecycle.ErrStoppedEarly) || !strings.Contains(err.Error(), "sweeper") {
			t.Fatalf("Run: %v, want ErrStoppedEarly с именем sweeper", err)
		}
	})
}
