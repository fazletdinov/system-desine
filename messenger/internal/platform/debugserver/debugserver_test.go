package debugserver_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"messenger/internal/platform/debugserver"
)

// fakeFlight — снимок без настоящего рекордера.
type fakeFlight struct{}

func (fakeFlight) WriteTo(w io.Writer) (int64, error) {
	n, err := io.WriteString(w, "flight-trace")
	return int64(n), err
}

// failingFlight — снимок, который ломается на середине записи.
type failingFlight struct{}

func (failingFlight) WriteTo(w io.Writer) (int64, error) {
	n, _ := io.WriteString(w, "flight-")
	return int64(n), errors.New("снимок оборвался")
}

// get отправляет запрос в обработчик напрямую, без сервера.
func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
	return rec
}

func TestHandlerServesPprofIndex(t *testing.T) {
	rec := get(t, debugserver.NewHandler(nil), "/debug/pprof/")

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "goroutine") {
		t.Fatalf("индекс: статус %d, тело %.200q", rec.Code, rec.Body.String())
	}
}

func TestHandlerNamedProfiles(t *testing.T) {
	h := debugserver.NewHandler(nil)
	for _, name := range []string{"heap", "goroutine", "goroutineleak", "mutex", "block", "allocs"} {
		t.Run(name, func(t *testing.T) {
			if rec := get(t, h, "/debug/pprof/"+name); rec.Code != http.StatusOK {
				t.Fatalf("статус %d, тело %.200q", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestProfileSecondsLimit(t *testing.T) {
	h := debugserver.NewHandler(nil)
	for _, path := range []string{"/debug/pprof/profile", "/debug/pprof/trace"} {
		for _, seconds := range []string{"61", "-1", "0", "abc", "NaN"} {
			t.Run(path+"?seconds="+seconds, func(t *testing.T) {
				rec := get(t, h, path+"?seconds="+seconds)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("статус %d, ждали 400", rec.Code)
				}
			})
		}
		t.Run(path+"?seconds=1 проходит проверку", func(t *testing.T) {
			// Запрос уже отменён: pprof не ждёт секунду, а сразу отдаёт то, что успел снять.
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, path+"?seconds=1", nil))
			if rec.Code == http.StatusBadRequest {
				t.Fatalf("seconds=1 отвергнут: %q", rec.Body.String())
			}
		})
	}
}

// Дельта-профиль (?seconds у именованного профиля) держит соединение так же долго.
func TestDeltaProfileSecondsLimit(t *testing.T) {
	if rec := get(t, debugserver.NewHandler(nil), "/debug/pprof/heap?seconds=600"); rec.Code != http.StatusBadRequest {
		t.Fatalf("статус %d, ждали 400", rec.Code)
	}
}

func TestFlightEndpoint(t *testing.T) {
	t.Run("с рекордером — снимок файлом", func(t *testing.T) {
		rec := get(t, debugserver.NewHandler(fakeFlight{}), "/debug/flight")

		if rec.Code != http.StatusOK || rec.Body.String() != "flight-trace" {
			t.Fatalf("статус %d, тело %q", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/octet-stream" {
			t.Errorf("Content-Type %q", ct)
		}
		if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="flight.trace"`) {
			t.Errorf("Content-Disposition %q", cd)
		}
	})
	t.Run("ошибка снимка — 500, а не обрезанный файл", func(t *testing.T) {
		rec := get(t, debugserver.NewHandler(failingFlight{}), "/debug/flight")
		if rec.Code != http.StatusInternalServerError || rec.Header().Get("Content-Disposition") != "" {
			t.Fatalf("статус %d, заголовки %v", rec.Code, rec.Header())
		}
	})
	t.Run("без рекордера — 404", func(t *testing.T) {
		if rec := get(t, debugserver.NewHandler(nil), "/debug/flight"); rec.Code != http.StatusNotFound {
			t.Fatalf("статус %d, ждали 404", rec.Code)
		}
	})
}

func TestEnableContentionProfiles(t *testing.T) {
	t.Cleanup(func() {
		runtime.SetMutexProfileFraction(0)
		runtime.SetBlockProfileRate(0)
	})

	debugserver.EnableContentionProfiles()

	// Отрицательное значение не меняет долю, а только возвращает текущую.
	if got := runtime.SetMutexProfileFraction(-1); got != debugserver.MutexProfileFraction {
		t.Fatalf("доля mutex-профиля %d, ждали %d", got, debugserver.MutexProfileFraction)
	}
}

func TestNewRejectsMissingLog(t *testing.T) {
	if _, err := debugserver.New(":6060", http.NotFoundHandler(), nil); err == nil {
		t.Fatal("ждали ошибку без журнала")
	}
	if _, err := debugserver.New(":6060", nil, slog.New(slog.DiscardHandler)); err == nil {
		t.Fatal("ждали ошибку без обработчика")
	}
}

// pipeListener — сеть в памяти для Serve (тот же приём, что в тестах httpserver): всё
// ожидание — на каналах пузыря synctest, время виртуальное.
type pipeListener struct {
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{conns: make(chan net.Conn), closed: make(chan struct{})}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

var errRefused = errors.New("pipe: соединение отклонено")

func (l *pipeListener) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	client, server := net.Pipe()
	select {
	case l.conns <- server:
		return client, nil
	case <-l.closed:
	case <-ctx.Done():
	}
	_, _ = client.Close(), server.Close()
	return nil, errRefused
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// TestServerStopsDuringLongProfile — идёт снятие профиля на 30 с; остановка не ждёт
// его, а обрывает через StopTimeout. Профиль — дельта-профиль кучи: CPU-профиль и
// трасса запускают горутины рантайма, которые synctest не считает «уснувшими», и
// виртуальное время в пузыре не пошло бы. Держит соединение он точно так же.
func TestServerStopsDuringLongProfile(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		srv, err := debugserver.New("", debugserver.NewHandler(nil), slog.New(slog.DiscardHandler))
		if err != nil {
			t.Fatal(err)
		}
		ln := newPipeListener()
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- srv.Serve(ctx, ln) }()

		client := &http.Client{Transport: &http.Transport{DialContext: ln.dial}}
		defer client.CloseIdleConnections()
		reqErr := make(chan error, 1)
		go func() {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://debug/debug/pprof/heap?seconds=30", nil)
			if err != nil {
				reqErr <- err
				return
			}
			resp, err := client.Do(req)
			if err == nil {
				_, err = io.ReadAll(resp.Body)
				_ = resp.Body.Close()
			}
			reqErr <- err
		}()
		synctest.Wait() // запрос внутри обработчика, тот ждёт 30 с
		start := time.Now()

		cancel()

		if err := <-done; err != nil {
			t.Fatalf("Serve: %v, want nil — оборванный профиль не сбой", err)
		}
		if elapsed := time.Since(start); elapsed > debugserver.StopTimeout {
			t.Fatalf("Serve вернулся через %v, ждали не позже StopTimeout = %v", elapsed, debugserver.StopTimeout)
		}
		if err := <-reqErr; err == nil {
			t.Error("профиль на 30 с завершился, хотя соединение должно быть оборвано")
		}
	})
}
