// Команда echo — TCP-эхо по строкам: сервер (-serve) и клиент, меряющий RTT.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"labs/0b/internal/stats"
)

type config struct {
	serve    bool
	addr     string
	idle     time.Duration
	shutdown time.Duration
	n        int
	interval time.Duration
	size     int
	dial     time.Duration
	timeout  time.Duration
	nodelay  bool
}

func parseFlags(args []string) (config, error) {
	var c config
	fs := flag.NewFlagSet("echo", flag.ContinueOnError)
	fs.BoolVar(&c.serve, "serve", false, "запустить сервер")
	fs.StringVar(&c.addr, "addr", ":7000", "адрес: для сервера — где слушать, для клиента — куда подключаться")
	fs.DurationVar(&c.idle, "idle", 60*time.Second, "сервер: закрыть соединение, если клиент молчит столько")
	fs.DurationVar(&c.shutdown, "shutdown", 5*time.Second, "сервер: сколько ждать активные соединения при остановке")
	fs.IntVar(&c.n, "n", 20, "клиент: сколько строк отправить")
	fs.DurationVar(&c.interval, "interval", 200*time.Millisecond, "клиент: пауза между строками")
	fs.IntVar(&c.size, "size", 32, "клиент: длина строки в байтах без перевода строки")
	fs.DurationVar(&c.dial, "dial", 5*time.Second, "клиент: таймаут установки соединения")
	fs.DurationVar(&c.timeout, "timeout", 5*time.Second, "клиент: таймаут ожидания ответа")
	fs.BoolVar(&c.nodelay, "nodelay", true, "клиент: TCP_NODELAY (false включает алгоритм Нейгла)")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	return c, nil
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, log); err != nil {
		log.Error("echo", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config, log *slog.Logger) error {
	if cfg.serve {
		var lc net.ListenConfig
		ln, err := lc.Listen(ctx, "tcp", cfg.addr)
		if err != nil {
			return fmt.Errorf("listen %s: %w", cfg.addr, err)
		}
		log.Info("слушаю", "addr", ln.Addr().String())
		return serve(ctx, ln, cfg.idle, cfg.shutdown, log)
	}
	return runClient(ctx, cfg, os.Stdout)
}

// #region server

// serve принимает соединения до отмены ctx, потом ждёт активные не дольше shutdown
// и закрывает оставшиеся. Владелец каждой горутины-обработчика — wg.
func serve(ctx context.Context, ln net.Listener, idle, shutdown time.Duration, log *slog.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		conns = map[net.Conn]struct{}{}
	)
	// Закрытие листенера — единственный способ прервать Accept.
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		ln.Close()
	}()

	var acceptErr error
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() == nil {
				acceptErr = fmt.Errorf("accept: %w", err)
				cancel() // останавливает горутину, закрывающую листенер
			}
			break
		}
		mu.Lock()
		conns[conn] = struct{}{}
		mu.Unlock()

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				mu.Lock()
				delete(conns, conn)
				mu.Unlock()
			}()
			lines, err := handle(conn, idle)
			log.Info("соединение закрыто", "client", conn.RemoteAddr().String(), "lines", lines, "reason", closeReason(err))
		}()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(shutdown):
		log.Warn("таймаут остановки, закрываю соединения", "after", shutdown)
		mu.Lock()
		for c := range conns {
			c.Close()
		}
		mu.Unlock()
		<-done
	}
	cancel()
	<-stopped
	return acceptErr
}

// handle возвращает эхом каждую строку; простаивающее дольше idle соединение закрывает.
func handle(conn net.Conn, idle time.Duration) (lines int, err error) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(idle)); err != nil {
			return lines, fmt.Errorf("set deadline: %w", err)
		}
		line, err := r.ReadString('\n')
		if err != nil {
			return lines, err
		}
		if _, err := io.WriteString(conn, line); err != nil {
			return lines, fmt.Errorf("write: %w", err)
		}
		lines++
	}
}

// #endregion server

func closeReason(err error) string {
	switch {
	case errors.Is(err, io.EOF):
		return "клиент закрыл соединение"
	case errors.Is(err, os.ErrDeadlineExceeded):
		return "простой"
	default:
		return err.Error()
	}
}

func runClient(ctx context.Context, cfg config, out io.Writer) error {
	d := net.Dialer{Timeout: cfg.dial}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", cfg.addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", cfg.addr, err)
	}
	defer conn.Close()
	connect := time.Since(start)

	if tcp, ok := conn.(*net.TCPConn); ok {
		if err := tcp.SetNoDelay(cfg.nodelay); err != nil {
			return fmt.Errorf("set nodelay: %w", err)
		}
	}

	rtts, err := pingLoop(ctx, conn, cfg)
	fmt.Fprintf(out, "connect: %v\n", connect)
	if len(rtts) > 0 {
		s := stats.Summary(rtts)
		fmt.Fprintf(out, "rtt: n=%d min=%v p50=%v p99=%v max=%v\n", s.N, s.Min, s.P50, s.P99, s.Max)
	}
	return err
}

// #region client

// pingLoop шлёт n строк и по одной читает ответы, замеряя RTT каждой.
func pingLoop(ctx context.Context, conn net.Conn, cfg config) ([]time.Duration, error) {
	r := bufio.NewReader(conn)
	rtts := make([]time.Duration, 0, cfg.n)
	for i := range cfg.n {
		if i > 0 {
			select {
			case <-ctx.Done():
				return rtts, ctx.Err()
			case <-time.After(cfg.interval):
			}
		}
		line := makeLine(i, cfg.size)
		rtt, err := roundTrip(conn, r, line, cfg.timeout)
		if err != nil {
			return rtts, fmt.Errorf("seq %d: %w", i, err)
		}
		rtts = append(rtts, rtt)
	}
	return rtts, nil
}

// #endregion client

func roundTrip(conn net.Conn, r *bufio.Reader, line string, timeout time.Duration) (time.Duration, error) {
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return 0, fmt.Errorf("set deadline: %w", err)
	}
	start := time.Now()
	if _, err := io.WriteString(conn, line); err != nil {
		return 0, fmt.Errorf("write: %w", err)
	}
	got, err := r.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("read: %w", err)
	}
	rtt := time.Since(start)
	if got != line {
		return 0, fmt.Errorf("эхо не совпало: отправили %q, получили %q", line, got)
	}
	return rtt, nil
}

// makeLine строит "seq=<i> xxx…\n" длиной size байт без перевода строки (если префикс короче).
func makeLine(seq, size int) string {
	prefix := fmt.Sprintf("seq=%d ", seq)
	pad := max(0, size-len(prefix))
	return prefix + strings.Repeat("x", pad) + "\n"
}
