package periodic

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

const every = time.Minute

// calls — журнал вызовов Fn: моменты начала по виртуальным часам и пик одновременных.
type calls struct {
	mu     sync.Mutex
	starts []time.Time
	active int
	peak   int
}

func (c *calls) enter() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.starts = append(c.starts, time.Now())
	c.active++
	c.peak = max(c.peak, c.active)
}

func (c *calls) leave() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active--
}

func (c *calls) snapshot() (starts []time.Time, peak int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Time(nil), c.starts...), c.peak
}

// fn — Fn, которая пишет вызов в журнал, работает d и возвращает err.
func (c *calls) fn(d time.Duration, err error) func(context.Context) error {
	return func(context.Context) error {
		c.enter()
		defer c.leave()
		if d > 0 {
			<-time.After(d)
		}
		return err
	}
}

func newTask(t *testing.T, c Config) *Task {
	t.Helper()
	if c.Name == "" {
		c.Name = "sweeper"
	}
	if c.Every == 0 {
		c.Every = every
	}
	if c.Log == nil {
		c.Log = slog.New(slog.DiscardHandler)
	}
	task, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

// start запускает Run в горутине пузыря; stop отменяет и возвращает результат Run.
func start(t *testing.T, task *Task) (stop func() error) {
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- task.Run(ctx) }()
	return func() error {
		cancel()
		return <-done
	}
}

func TestNewRejectsBadConfig(t *testing.T) {
	fn := func(context.Context) error { return nil }
	log := slog.New(slog.DiscardHandler)
	cases := []struct {
		name string
		cfg  Config
	}{
		{"пустое имя", Config{Every: every, Fn: fn, Log: log}},
		{"нулевой период", Config{Name: "x", Fn: fn, Log: log}},
		{"отрицательный период", Config{Name: "x", Every: -time.Second, Fn: fn, Log: log}},
		{"джиттер меньше нуля", Config{Name: "x", Every: every, Jitter: -0.1, Fn: fn, Log: log}},
		{"джиттер 1", Config{Name: "x", Every: every, Jitter: 1, Fn: fn, Log: log}},
		{"без Fn", Config{Name: "x", Every: every, Log: log}},
		{"без журнала", Config{Name: "x", Every: every, Fn: fn}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := New(c.cfg); err == nil {
				t.Fatal("ждали ошибку")
			}
		})
	}
}

func TestTaskFirstRunAfterEvery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &calls{}
		stop := start(t, newTask(t, Config{Fn: c.fn(0, nil)}))

		synctest.Sleep(every - time.Nanosecond)
		if starts, _ := c.snapshot(); len(starts) != 0 {
			t.Fatalf("за %v до периода Fn вызвана %d раз: первая итерация — через Every, не сразу", time.Nanosecond, len(starts))
		}
		synctest.Sleep(time.Nanosecond)
		if starts, _ := c.snapshot(); len(starts) != 1 {
			t.Fatalf("через Every Fn вызвана %d раз, ждали 1", len(starts))
		}

		if err := stop(); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
}

func TestTaskJitterWithinBounds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			jitter = 0.1
			n      = 50
			// Границы интервала: Every ± 10 %.
			lo = every - every/10
			hi = every + every/10
		)
		c := &calls{}
		task := newTask(t, Config{Jitter: jitter, Fn: c.fn(0, nil)})
		task.rnd = rand.New(rand.NewPCG(1, 2)) // сид теста: прогон повторяем
		begin := time.Now()
		stop := start(t, task)

		// n интервалов заведомо укладываются в n·hi; лишние итерации не мешают.
		synctest.Sleep(n * hi)
		if err := stop(); err != nil {
			t.Fatalf("Run: %v", err)
		}

		starts, _ := c.snapshot()
		if len(starts) < n {
			t.Fatalf("итераций %d, ждали не меньше %d", len(starts), n)
		}
		prev, distinct := begin, map[time.Duration]bool{}
		for i, s := range starts[:n] {
			gap := s.Sub(prev)
			if gap < lo || gap > hi {
				t.Fatalf("интервал %d: %v вне [%v, %v]", i, gap, lo, hi)
			}
			distinct[gap] = true
			prev = s
		}
		// Джиттер действительно меняет интервал от итерации к итерации.
		if len(distinct) < n/2 {
			t.Fatalf("различных интервалов %d из %d: джиттер не работает", len(distinct), n)
		}
	})
}

// #region no-overlap-test
func TestTaskSlowIterationDoesNotOverlap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &calls{}
		stop := start(t, newTask(t, Config{Fn: c.fn(2*every, nil)}))

		synctest.Sleep(10 * every)
		if err := stop(); err != nil {
			t.Fatalf("Run: %v", err)
		}

		starts, peak := c.snapshot()
		if peak != 1 {
			t.Fatalf("одновременно %d вызовов Fn, ждали не больше 1", peak)
		}
		// Следующий период отсчитывается от конца итерации: Every + 2·Every на круг.
		for i := 1; i < len(starts); i++ {
			if gap := starts[i].Sub(starts[i-1]); gap != 3*every {
				t.Fatalf("между началами итераций %v, ждали 3·Every = %v", gap, 3*every)
			}
		}
	})
}

// #endregion no-overlap-test

func TestTaskErrorLoggedAndContinues(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logs bytes.Buffer // пишет только горутина Run, читаем после её выхода
		c := &calls{}
		stop := start(t, newTask(t, Config{
			Fn:  c.fn(0, errors.New("база недоступна")),
			Log: slog.New(slog.NewTextHandler(&logs, nil)),
		}))

		synctest.Sleep(3 * every)
		if err := stop(); err != nil {
			t.Fatalf("Run: %v — ошибка Fn не должна останавливать задачу", err)
		}

		if starts, _ := c.snapshot(); len(starts) != 3 {
			t.Fatalf("итераций %d, ждали 3: после ошибки задача продолжает работу", len(starts))
		}
		out := logs.String()
		if strings.Count(out, "level=WARN") != 3 || !strings.Contains(out, "база недоступна") || !strings.Contains(out, "sweeper") {
			t.Fatalf("журнал: %s", out)
		}
	})
}

func TestTaskStopsOnCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &calls{}
		stop := start(t, newTask(t, Config{Fn: c.fn(0, nil)}))
		synctest.Sleep(every / 2)

		if err := stop(); err != nil {
			t.Fatalf("Run после отмены: %v, want nil", err)
		}

		// Таймер остановлен, горутин не осталось: иначе synctest.Test не завершится.
		synctest.Sleep(10 * every)
		if starts, _ := c.snapshot(); len(starts) != 0 {
			t.Fatalf("после остановки Fn вызвана %d раз", len(starts))
		}
	})
}
