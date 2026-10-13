package httpserver_test

import (
	"context"
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
