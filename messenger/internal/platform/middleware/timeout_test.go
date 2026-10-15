package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"messenger/internal/platform/middleware"
)

const testTimeout = 50 * time.Millisecond

// timeoutChain — Timeout с onTimeout, который отвечает 503 и считает вызовы.
func timeoutChain(h http.Handler) (http.Handler, *atomic.Int32) {
	var calls atomic.Int32
	onTimeout := func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	return middleware.Chain(h, middleware.Timeout(testTimeout, onTimeout)), &calls
}

func serveTimeout(t *testing.T, h http.Handler, ctx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/chats", nil))
	return rec
}

func TestTimeoutHandlerRespectsContext(t *testing.T) {
	// Обработчик, как и наши use case'ы, бросает работу по ctx.Done и ничего не пишет.
	h, calls := timeoutChain(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))

	start := time.Now()
	rec := serveTimeout(t, h, t.Context())

	if rec.Code != http.StatusServiceUnavailable || calls.Load() != 1 {
		t.Fatalf("статус %d, вызовов onTimeout %d", rec.Code, calls.Load())
	}
	if d := time.Since(start); d > 10*testTimeout {
		t.Fatalf("ответ через %v при сроке %v", d, testTimeout)
	}
}

func TestTimeoutDeadlineVisibleToHandler(t *testing.T) {
	var deadline time.Time
	var ok bool
	h, _ := timeoutChain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok = r.Context().Deadline()
		w.WriteHeader(http.StatusNoContent)
	}))

	serveTimeout(t, h, t.Context())

	if !ok || time.Until(deadline) > testTimeout {
		t.Fatalf("срок у обработчика: %v, %v", deadline, ok)
	}
}

func TestTimeoutFastHandlerUntouched(t *testing.T) {
	h, calls := timeoutChain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	rec := serveTimeout(t, h, t.Context())

	if rec.Code != http.StatusCreated || calls.Load() != 0 {
		t.Fatalf("статус %d, вызовов onTimeout %d", rec.Code, calls.Load())
	}
}

func TestTimeoutLateWriteKeepsHandlerResponse(t *testing.T) {
	// Обработчик не смотрит на ctx и отвечает после срока. Перехватить ответ уже
	// нельзя: Timeout не прерывает обработчик, а второй ответ поверх начатого — ошибка.
	h, calls := timeoutChain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * testTimeout)
		w.WriteHeader(http.StatusOK)
	}))

	rec := serveTimeout(t, h, t.Context())

	if rec.Code != http.StatusOK || calls.Load() != 0 {
		t.Fatalf("статус %d, вызовов onTimeout %d", rec.Code, calls.Load())
	}
}

func TestTimeoutHandlerWroteTimeoutItself(t *testing.T) {
	// Адаптер сам перевёл DeadlineExceeded в 503 problem+json — Timeout не дублирует.
	h, calls := timeoutChain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	rec := serveTimeout(t, h, t.Context())

	if rec.Code != http.StatusServiceUnavailable || calls.Load() != 0 {
		t.Fatalf("статус %d, вызовов onTimeout %d", rec.Code, calls.Load())
	}
}

func TestTimeoutClientGoneNoResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	h, calls := timeoutChain(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		cancel() // клиент ушёл раньше срока
		<-r.Context().Done()
	}))

	serveTimeout(t, h, ctx)

	if calls.Load() != 0 {
		t.Fatal("клиент ушёл, а Timeout пытается ему ответить")
	}
}

func TestTimeoutNoWriteWithoutDeadline(t *testing.T) {
	// Обработчик вернулся молча до срока: это не таймаут, отвечает net/http (200).
	h, calls := timeoutChain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	serveTimeout(t, h, t.Context())

	if calls.Load() != 0 {
		t.Fatal("onTimeout без истечения срока")
	}
}
