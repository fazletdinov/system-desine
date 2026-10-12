package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func idleArm(idle time.Duration) func(net.Conn) error {
	return func(c net.Conn) error { return c.SetReadDeadline(time.Now().Add(idle)) }
}

func TestMakeLine(t *testing.T) {
	tests := []struct {
		name      string
		seq, size int
		wantLen   int
	}{
		{"добивает до размера", 3, 32, 33},
		{"префикс длиннее размера не режется", 12345, 4, len("seq=12345 \n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			head, tail := makeLine(tt.seq, tt.size)
			got := head + tail
			if len(got) != tt.wantLen || !strings.HasSuffix(got, "\n") || !strings.HasPrefix(got, "seq=") {
				t.Fatalf("makeLine(%d,%d) = %q (len %d), want len %d", tt.seq, tt.size, got, len(got), tt.wantLen)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	ok := config{n: 1, interval: 1, size: 32, dial: 1, timeout: 1, writes: 1}
	tests := []struct {
		name    string
		mutate  func(*config)
		wantErr bool
	}{
		{"корректный клиент", func(*config) {}, false},
		{"n равно нулю", func(c *config) { c.n = 0 }, true},
		{"нулевой interval", func(c *config) { c.interval = 0 }, true},
		{"размер больше предела", func(c *config) { c.size = maxLine }, true},
		{"writes вне 1..2", func(c *config) { c.writes = 3 }, true},
		{"сервер с нулевым idle", func(c *config) { c.serve = true; c.shutdown = 1 }, true},
		{"корректный сервер", func(c *config) { c.serve = true; c.shutdown = 1; c.idle = 1 }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ok
			tt.mutate(&c)
			if err := c.validate(); (err != nil) != tt.wantErr {
				t.Fatalf("validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandleEchoesLinesUntilClientCloses(t *testing.T) {
	srv, cli := net.Pipe()
	type result struct {
		lines int
		err   error
	}
	done := make(chan result, 1)
	go func() {
		n, err := handle(srv, idleArm(time.Second), time.Second)
		done <- result{n, err}
	}()

	r := bufio.NewReader(cli)
	for _, msg := range []string{"a\n", "bb\n"} {
		go cli.Write([]byte(msg))
		got, err := r.ReadString('\n')
		if err != nil || got != msg {
			t.Fatalf("echo = %q, %v; want %q", got, err, msg)
		}
	}
	cli.Close()

	res := <-done
	// Закрытие конца net.Pipe даёт ErrClosedPipe, а не io.EOF как у TCP; важно, что цикл завершился.
	if res.lines != 2 || res.err == nil {
		t.Fatalf("lines=%d err=%v", res.lines, res.err)
	}
}

func TestHandleClosesIdleConnection(t *testing.T) {
	srv, cli := net.Pipe()
	defer cli.Close()
	_, err := handle(srv, idleArm(20*time.Millisecond), time.Second)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
}

func TestHandleDropsClientThatNeverReads(t *testing.T) {
	srv, cli := net.Pipe()
	defer cli.Close()
	go cli.Write([]byte("hello\n")) // net.Pipe без буфера: эхо блокируется, пока клиент не читает
	_, err := handle(srv, idleArm(time.Second), 20*time.Millisecond)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("err = %v, want write deadline exceeded", err)
	}
}

func TestHandleRejectsOverlongLine(t *testing.T) {
	srv, cli := net.Pipe()
	defer cli.Close()
	go cli.Write([]byte(strings.Repeat("x", maxLine+1)))
	_, err := handle(srv, idleArm(time.Second), time.Second)
	if err == nil || !strings.Contains(err.Error(), "длиннее") {
		t.Fatalf("err = %v, want overlong line error", err)
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		writes  int
		reply   string
		wantErr bool
	}{
		{"совпавшее эхо, один Write", 1, "hi\n", false},
		{"совпавшее эхо, два Write", 2, "hi\n", false},
		{"искажённое эхо", 1, "ho\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, cli := net.Pipe()
			defer cli.Close()
			go func() {
				defer srv.Close()
				r := bufio.NewReader(srv)
				r.ReadString('\n')
				srv.Write([]byte(tt.reply))
			}()
			_, err := roundTrip(context.Background(), cli, bufio.NewReader(cli), "h", "i\n", tt.writes, time.Second)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func startServer(t *testing.T, ln net.Listener, idle, shutdown time.Duration) (cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan error, 1)
	go func() { ch <- serve(ctx, ln, idle, shutdown, discard) }()
	t.Cleanup(cancel)
	return cancel, ch
}

func TestServeShutdownWakesIdleConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	const idle, shutdown = time.Minute, 5 * time.Second
	cancel, done := startServer(t, ln, idle, shutdown)

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Убеждаемся, что соединение принято и обработчик ждёт строку.
	if _, err := conn.Write([]byte("ping\n")); err != nil {
		t.Fatal(err)
	}
	if got, err := bufio.NewReader(conn).ReadString('\n'); err != nil || got != "ping\n" {
		t.Fatalf("echo = %q, %v", got, err)
	}

	start := time.Now()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve не вернулся: простаивающее соединение не разбужено")
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("остановка заняла %v, ждали << idle и shutdown", took)
	}
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("клиент должен увидеть EOF, получил %v", err)
	}
}

// gatedListener оборачивает соединения так, что Write ждёт сигнала: имитирует обмен «в полёте».
type gatedListener struct {
	net.Listener
	gate    chan struct{}
	entered chan struct{}
}

func (l gatedListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return gatedConn{Conn: c, gate: l.gate, entered: l.entered}, nil
}

type gatedConn struct {
	net.Conn
	gate    chan struct{}
	entered chan struct{}
}

func (c gatedConn) Write(b []byte) (int, error) {
	c.entered <- struct{}{}
	<-c.gate
	return c.Conn.Write(b)
}

func TestServeShutdownLetsInFlightExchangeFinish(t *testing.T) {
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln := gatedListener{Listener: inner, gate: make(chan struct{}), entered: make(chan struct{}, 1)}
	cancel, done := startServer(t, ln, time.Minute, 5*time.Second)

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("slow\n")); err != nil {
		t.Fatal(err)
	}
	<-ln.entered // сервер прочитал строку и застрял на записи эха

	cancel()
	select {
	case err := <-done:
		t.Fatalf("serve вернулся (%v) при незавершённом обмене", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(ln.gate)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if got, err := bufio.NewReader(conn).ReadString('\n'); err != nil || got != "slow\n" {
		t.Fatalf("эхо в полёте потеряно: %q, %v", got, err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve не вернулся после завершения обмена")
	}
}

func TestServeForcesCloseAfterShutdownTimeout(t *testing.T) {
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln := gatedListener{Listener: inner, gate: make(chan struct{}), entered: make(chan struct{}, 1)}
	cancel, done := startServer(t, ln, time.Minute, 100*time.Millisecond)

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("stuck\n")); err != nil {
		t.Fatal(err)
	}
	<-ln.entered // обработчик застрял на записи эха, обмен не завершится сам
	cancel()

	// Ворота закрыты: EOF клиент может увидеть только если сервер принудительно закрыл сокет.
	conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("клиент должен увидеть EOF от принудительного закрытия, получил %v", err)
	}

	close(ln.gate) // освобождаем зависшую запись, чтобы serve мог вернуться
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serve не вернулся после принудительного закрытия")
	}
}
