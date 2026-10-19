package main

import (
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
)

// exitRecorder подменяет os.Exit: запоминает коды вместо выхода из процесса.
type exitRecorder struct {
	mu    sync.Mutex
	codes []int
}

func (e *exitRecorder) exit(code int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.codes = append(e.codes, code)
}

func (e *exitRecorder) calls() []int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]int(nil), e.codes...)
}

// watcher запускает exitOnSecondSignal в горутине пузыря. Как в run: sig получает
// каждый сигнал, включая первый, — first закрывает NotifyContext. stop закрывает sig и
// ждёт выхода горутины.
type watcher struct {
	sig   chan os.Signal
	first chan struct{}
	exits *exitRecorder
	logs  *syncBuffer
	done  chan struct{}
}

func watch() *watcher {
	w := &watcher{
		sig:   make(chan os.Signal, 2),
		first: make(chan struct{}),
		exits: &exitRecorder{},
		logs:  &syncBuffer{},
		done:  make(chan struct{}),
	}
	log := slog.New(slog.NewJSONHandler(w.logs, nil))
	go func() {
		defer close(w.done)
		exitOnSecondSignal(w.sig, w.first, w.exits.exit, log)
	}()
	return w
}

func (w *watcher) stop() {
	close(w.sig)
	<-w.done
}

func TestExitOnSecondSignal(t *testing.T) {
	// Копия первого сигнала доходит до sig то раньше, то позже, чем NotifyContext
	// закроет first: signal.Notify раздаёт сигнал каналам в произвольном порядке.
	cases := []struct {
		name  string
		steps func(w *watcher)
	}{
		{"копия первого до закрытия first", func(w *watcher) {
			w.sig <- os.Interrupt
			synctest.Wait()
			close(w.first)
		}},
		{"копия первого после закрытия first", func(w *watcher) {
			close(w.first)
			synctest.Wait()
			w.sig <- os.Interrupt
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := watch()
				c.steps(w)
				synctest.Wait()
				if got := w.exits.calls(); len(got) != 0 {
					t.Fatalf("exit(%v) на первом сигнале: остановку начал NotifyContext, выходить рано", got)
				}

				w.sig <- os.Interrupt // второй сигнал
				synctest.Wait()

				if got := w.exits.calls(); len(got) != 1 || got[0] != 1 {
					t.Fatalf("exit вызван с %v, ждали один раз с кодом 1", got)
				}
				if !strings.Contains(w.logs.String(), "второй сигнал") {
					t.Fatalf("в журнале нет «второй сигнал»: %s", w.logs.String())
				}
				w.stop()
			})
		})
	}
}

func TestNoExitBeforeFirst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := watch()

		w.sig <- os.Interrupt // его обработал NotifyContext; first ещё не закрыт
		synctest.Wait()

		if got := w.exits.calls(); len(got) != 0 {
			t.Fatalf("exit(%v) до закрытия first", got)
		}
		// Остановка штатная: run закрывает канал, горутина выходит — иначе пузырь
		// synctest не завершится.
		w.stop()
		if got := w.exits.calls(); len(got) != 0 {
			t.Fatalf("exit(%v) при закрытии канала", got)
		}
	})
}
