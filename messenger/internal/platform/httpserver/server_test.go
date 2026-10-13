package httpserver_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"messenger/internal/platform/httpserver"
)

func TestRunStopsOnCancel(t *testing.T) {
	srv := httpserver.New("127.0.0.1:0", http.NotFoundHandler(), slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- httpserver.Run(ctx, srv, time.Second) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v, want nil после отмены", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run не завершился после отмены контекста")
	}
}

func TestRunReturnsListenError(t *testing.T) {
	// Занимаем порт сами: второй ListenAndServe на нём должен упасть сразу.
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	srv := httpserver.New(ln.Addr().String(), http.NotFoundHandler(), slog.New(slog.DiscardHandler))

	if err := httpserver.Run(context.Background(), srv, time.Second); err == nil {
		t.Fatal("Run: want error, порт занят")
	}
}

// freeAddr находит свободный порт: ListenAndServe сам порт не сообщает.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

// startInFlight запускает Run с обработчиком, который ждёт release, и отправляет
// запрос; возвращает, когда обработчик уже работает.
func startInFlight(t *testing.T, shutdownTimeout time.Duration) (cancel context.CancelFunc, release chan struct{}, runErr, reqErr <-chan error) {
	t.Helper()
	entered, rel := make(chan struct{}), make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-rel
		w.WriteHeader(http.StatusNoContent)
	})
	addr := freeAddr(t)
	srv := httpserver.New(addr, h, slog.New(slog.DiscardHandler))
	ctx, cancelRun := context.WithCancel(context.Background())
	runc, reqc := make(chan error, 1), make(chan error, 1)
	go func() { runc <- httpserver.Run(ctx, srv, shutdownTimeout) }()
	go func() { reqc <- getUntilServed(t.Context(), "http://"+addr+"/") }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("запрос не дошёл до обработчика")
	}
	return cancelRun, rel, runc, reqc
}

// getUntilServed повторяет запрос, пока сервер не начнёт слушать порт.
func getUntilServed(ctx context.Context, url string) error {
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			return resp.Body.Close()
		}
		var opErr *net.OpError
		if !errors.As(err, &opErr) || opErr.Op != "dial" {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestRunWaitsForInFlightRequest(t *testing.T) {
	cancel, release, runErr, reqErr := startInFlight(t, 5*time.Second)

	cancel()
	select {
	case err := <-runErr:
		t.Fatalf("Run вернулся (%v), не дождавшись запроса", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)

	if err := <-reqErr; err != nil {
		t.Errorf("запрос оборвался: %v", err)
	}
	if err := <-runErr; err != nil {
		t.Errorf("Run: %v, want nil", err)
	}
}

func TestRunForceClosesAfterShutdownTimeout(t *testing.T) {
	cancel, release, runErr, reqErr := startInFlight(t, 50*time.Millisecond)
	defer close(release)

	cancel()

	if err := <-runErr; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run: %v, want DeadlineExceeded", err)
	}
	// Close оборвал соединение: клиент получил ошибку, а не ждёт вечно.
	select {
	case err := <-reqErr:
		if err == nil {
			t.Error("запрос завершился успешно, хотя соединение должно быть закрыто")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("соединение не закрыто после таймаута Shutdown")
	}
}
