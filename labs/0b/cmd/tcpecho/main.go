// Команда tcpecho — TCP-эхо по строкам: сервер (-serve) и клиент, меряющий RTT.
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

// maxLine — предел длины строки (вместе с переводом строки); длиннее — клиент нарушает протокол.
const maxLine = 4096

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
	writes   int
}

func parseFlags(args []string) (config, error) {
	var c config
	fs := flag.NewFlagSet("tcpecho", flag.ContinueOnError)
	fs.BoolVar(&c.serve, "serve", false, "запустить сервер")
	fs.StringVar(&c.addr, "addr", ":7000", "адрес: для сервера — где слушать, для клиента — куда подключаться")
	fs.DurationVar(&c.idle, "idle", 60*time.Second, "сервер: закрыть соединение, если клиент молчит (или не читает) столько")
	fs.DurationVar(&c.shutdown, "shutdown", 5*time.Second, "сервер: сколько ждать активные соединения при остановке")
	fs.IntVar(&c.n, "n", 20, "клиент: сколько строк отправить")
	fs.DurationVar(&c.interval, "interval", 200*time.Millisecond, "клиент: пауза между строками")
	fs.IntVar(&c.size, "size", 32, "клиент: длина строки в байтах без перевода строки")
	fs.DurationVar(&c.dial, "dial", 5*time.Second, "клиент: таймаут установки соединения")
	fs.DurationVar(&c.timeout, "timeout", 5*time.Second, "клиент: таймаут ожидания ответа")
	fs.BoolVar(&c.nodelay, "nodelay", true, "клиент: TCP_NODELAY (false включает алгоритм Нейгла)")
	fs.IntVar(&c.writes, "writes", 1, "клиент: сколько вызовов Write на строку, 1 или 2 (2 — демонстрация Нейгла)")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if err := c.validate(); err != nil {
		return config{}, err
	}
	return c, nil
}

func (c config) validate() error {
	positive := func(name string, d time.Duration) error {
		if d <= 0 {
			return fmt.Errorf("-%s должен быть больше нуля, получено %v", name, d)
		}
		return nil
	}
	if c.serve {
		return errors.Join(positive("idle", c.idle), positive("shutdown", c.shutdown))
	}
	err := errors.Join(positive("interval", c.interval), positive("dial", c.dial), positive("timeout", c.timeout))
	if c.n <= 0 {
		err = errors.Join(err, fmt.Errorf("-n должен быть больше нуля, получено %d", c.n))
	}
	if c.size < 1 || c.size >= maxLine {
		err = errors.Join(err, fmt.Errorf("-size должен быть в диапазоне 1..%d, получено %d", maxLine-1, c.size))
	}
	if c.writes != 1 && c.writes != 2 {
		err = errors.Join(err, fmt.Errorf("-writes должен быть 1 или 2, получено %d", c.writes))
	}
	return err
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Error("флаги", "err", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, log); err != nil {
		log.Error("tcpecho", "err", err)
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

// server хранит активные соединения, чтобы при остановке разбудить простаивающие обработчики.
type server struct {
	idle time.Duration
	log  *slog.Logger

	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	closing bool
}

func (s *server) add(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[conn] = struct{}{}
	if s.closing { // принят в момент остановки: будить некого, но и ждать idle незачем
		s.wake(conn)
	}
}

func (s *server) remove(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, conn)
}

func (s *server) isClosing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closing
}

// beginShutdown будит всех, кто ждёт строку: чтение с дедлайном «сейчас» завершается сразу.
func (s *server) beginShutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closing = true
	for c := range s.conns {
		s.wake(c)
	}
}

// wake вызывается под s.mu.
func (s *server) wake(c net.Conn) {
	if err := c.SetReadDeadline(time.Now()); err != nil {
		s.log.Warn("не смог разбудить соединение", "client", c.RemoteAddr().String(), "err", err)
	}
}

// armRead выставляет дедлайн ожидания строки под тем же замком, что и beginShutdown:
// иначе обработчик мог бы затереть дедлайн остановки значением idle.
func (s *server) armRead(c net.Conn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return c.SetReadDeadline(time.Now())
	}
	return c.SetReadDeadline(time.Now().Add(s.idle))
}

func (s *server) forceClose() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.conns {
		_ = c.Close() // обработчик сам закроет соединение и залогирует причину
	}
}

// #region server

// serve принимает соединения до отмены ctx, потом ждёт активные не дольше shutdown
// и закрывает оставшиеся. Владелец каждой горутины-обработчика — wg.
func serve(ctx context.Context, ln net.Listener, idle, shutdown time.Duration, log *slog.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s := &server{idle: idle, log: log, conns: map[net.Conn]struct{}{}}

	// Закрытие листенера — единственный способ прервать Accept.
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		log.Info("остановка: больше не принимаю соединения", "grace", shutdown)
		if err := ln.Close(); err != nil {
			log.Warn("закрытие листенера", "err", err)
		}
		s.beginShutdown()
	}()

	var wg sync.WaitGroup
	var acceptErr error
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() == nil {
				acceptErr = fmt.Errorf("accept: %w", err)
				cancel() // останавливает горутину выше
			}
			break
		}
		s.add(conn)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer s.remove(conn)
			lines, err := handle(conn, s.armRead, idle)
			log.Info("соединение закрыто", "client", conn.RemoteAddr().String(), "lines", lines, "reason", s.closeReason(err))
		}()
	}
	cancel()
	<-stopped

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(shutdown):
		log.Warn("таймаут остановки, закрываю соединения", "after", shutdown)
		s.forceClose()
		<-done
	}
	return acceptErr
}

// handle возвращает эхом каждую строку. arm выставляет дедлайн ожидания следующей строки;
// на запись дедлайн idle ставится отдельно: клиент, который не читает ответы, не должен
// держать обработчик вечно.
func handle(conn net.Conn, arm func(net.Conn) error, idle time.Duration) (lines int, err error) {
	defer func() {
		if cerr := conn.Close(); cerr != nil && !errors.Is(cerr, net.ErrClosed) {
			err = errors.Join(err, fmt.Errorf("close: %w", cerr))
		}
	}()
	r := bufio.NewReaderSize(conn, maxLine)
	for {
		if err := arm(conn); err != nil {
			return lines, fmt.Errorf("set read deadline: %w", err)
		}
		line, err := r.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			return lines, fmt.Errorf("строка длиннее %d байт", maxLine)
		}
		if err != nil {
			return lines, err
		}
		if err := conn.SetWriteDeadline(time.Now().Add(idle)); err != nil {
			return lines, fmt.Errorf("set write deadline: %w", err)
		}
		if _, err := conn.Write(line); err != nil {
			return lines, fmt.Errorf("write: %w", err)
		}
		lines++
	}
}

// #endregion server

func (s *server) closeReason(err error) string {
	switch {
	case errors.Is(err, io.EOF):
		return "клиент закрыл соединение"
	case s.isClosing() && (errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, net.ErrClosed)):
		return "сервер остановлен"
	case errors.Is(err, os.ErrDeadlineExceeded):
		return "простой"
	case errors.Is(err, net.ErrClosed):
		return "сервер остановлен"
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
	defer func() { _ = conn.Close() }() // обмен окончен, ошибка закрытия ничего не меняет
	connect := time.Since(start)

	// Отмена ctx прерывает блокирующий ввод-вывод: дедлайн «сейчас».
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	defer stop()

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
		head, tail := makeLine(i, cfg.size)
		rtt, err := roundTrip(ctx, conn, r, head, tail, cfg.writes, cfg.timeout)
		if err != nil {
			if ctx.Err() != nil {
				return rtts, ctx.Err()
			}
			return rtts, fmt.Errorf("seq %d: %w", i, err)
		}
		rtts = append(rtts, rtt)
	}
	return rtts, nil
}

// roundTrip пишет строку и ждёт её эхо. При writes == 2 строка уходит двумя Write
// (write-write-read) — именно этот шаблон задерживает алгоритм Нейгла.
func roundTrip(ctx context.Context, conn net.Conn, r *bufio.Reader, head, tail string, writes int, timeout time.Duration) (time.Duration, error) {
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return 0, fmt.Errorf("set deadline: %w", err)
	}
	// Отмена могла прийти до SetDeadline и быть затёртой им; после проверки дедлайн «сейчас»
	// от AfterFunc уже перекроет наш.
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	start := time.Now()
	if writes == 2 {
		if _, err := io.WriteString(conn, head); err != nil {
			return 0, fmt.Errorf("write: %w", err)
		}
		if _, err := io.WriteString(conn, tail); err != nil {
			return 0, fmt.Errorf("write: %w", err)
		}
	} else if _, err := io.WriteString(conn, head+tail); err != nil {
		return 0, fmt.Errorf("write: %w", err)
	}
	got, err := r.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("read: %w", err)
	}
	rtt := time.Since(start)
	if want := head + tail; got != want {
		return 0, fmt.Errorf("эхо не совпало: отправили %q, получили %q", want, got)
	}
	return rtt, nil
}

// #endregion client

// makeLine строит строку "seq=<i> xxx…\n": head — префикс, tail — padding и перевод строки.
// Длина без перевода строки равна size, если префикс короче.
func makeLine(seq, size int) (head, tail string) {
	head = fmt.Sprintf("seq=%d ", seq)
	pad := max(0, size-len(head))
	return head, strings.Repeat("x", pad) + "\n"
}
