// Команда h2server — HTTPS-сервер (HTTP/2 через ALPN) и HTTP/1.1 без TLS для лабораторных 0b.
// Сертификат выпускает собственный CA при старте; ca.pem пишется для клиентов.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"labs/0b/internal/certs"
)

const (
	maxBytes = 10 << 20         // предел /bytes: 10 МБ
	maxSlow  = 10 * time.Second // предел /slow
)

type config struct {
	addr     string
	httpAddr string
	names    string
	caOut    string
	ttl      time.Duration
	grace    time.Duration
}

func parseFlags(args []string) (config, error) {
	var c config
	fs := flag.NewFlagSet("h2server", flag.ContinueOnError)
	fs.StringVar(&c.addr, "addr", ":8443", "адрес HTTPS (HTTP/2 и HTTP/1.1 через ALPN); пусто — не поднимать")
	fs.StringVar(&c.httpAddr, "http-addr", ":8080", "адрес HTTP/1.1 без TLS; пусто — не поднимать")
	fs.StringVar(&c.names, "names", "api.messenger.example,server", "имена в сертификате через запятую")
	fs.StringVar(&c.caOut, "ca-out", "ca.pem", "куда записать сертификат CA для клиентов")
	fs.DurationVar(&c.ttl, "ttl", 24*time.Hour, "срок действия CA и сертификата сервера")
	fs.DurationVar(&c.grace, "grace", 15*time.Second, "сколько ждать активные запросы при остановке")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if err := c.validate(); err != nil {
		return config{}, err
	}
	return c, nil
}

func (c config) validate() error {
	var errs []error
	if c.addr == "" && c.httpAddr == "" {
		errs = append(errs, errors.New("нужен хотя бы один из -addr и -http-addr"))
	}
	if len(splitNames(c.names)) == 0 {
		errs = append(errs, fmt.Errorf("-names: нужно хотя бы одно имя, получено %q", c.names))
	}
	if c.caOut == "" {
		errs = append(errs, errors.New("-ca-out не должен быть пустым"))
	}
	if c.ttl <= 0 {
		errs = append(errs, fmt.Errorf("-ttl должен быть больше нуля, получено %v", c.ttl))
	}
	if c.grace <= 0 {
		errs = append(errs, fmt.Errorf("-grace должен быть больше нуля, получено %v", c.grace))
	}
	return errors.Join(errs...)
}

func splitNames(s string) []string {
	var names []string
	for n := range strings.SplitSeq(s, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	return names
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
		log.Error("h2server", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config, log *slog.Logger) error {
	ca, err := certs.NewCA("lab0b-ca", cfg.ttl)
	if err != nil {
		return err
	}
	cert, err := ca.Issue(splitNames(cfg.names), cfg.ttl)
	if err != nil {
		return err
	}
	// Сертификат CA публичен: 0644 — чтобы клиенты в других контейнерах могли его прочитать.
	if err := writeFileAtomic(cfg.caOut, ca.PEM(), 0o644); err != nil {
		return fmt.Errorf("write CA: %w", err)
	}
	log.Info("сертификат CA записан", "path", cfg.caOut, "names", cfg.names)

	h := newHandler(log)
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	var eps []endpoint
	closeAll := func() {
		for _, ep := range eps {
			_ = ep.ln.Close() // serve ещё не запущен: закрываем то, что успели открыть
		}
	}
	var lc net.ListenConfig
	for _, l := range []struct {
		addr string
		tls  *tls.Config
	}{{cfg.addr, tlsCfg}, {cfg.httpAddr, nil}} {
		if l.addr == "" {
			continue
		}
		ln, err := lc.Listen(ctx, "tcp", l.addr)
		if err != nil {
			closeAll()
			return fmt.Errorf("listen %s: %w", l.addr, err)
		}
		eps = append(eps, endpoint{srv: newServer(h, l.tls, log), ln: ln})
		log.Info("слушаю", "addr", ln.Addr().String(), "tls", l.tls != nil)
	}
	return serve(ctx, cfg.grace, log, eps...)
}

// writeFileAtomic пишет во временный файл рядом и переименовывает его: rename в пределах
// одного каталога атомарен, поэтому клиент, читающий ca.pem, видит либо старый файл
// целиком, либо новый целиком — но не наполовину записанный PEM.
func writeFileAtomic(path string, data []byte, perm os.FileMode) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			_ = f.Close()      // уже закрыт или запись провалилась — важна исходная ошибка
			_ = os.Remove(tmp) // не оставляем мусор рядом с ca.pem
		}
	}()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := f.Chmod(perm); err != nil { // CreateTemp создаёт файл с правами 0600
		return fmt.Errorf("chmod %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename to %s: %w", path, err)
	}
	return nil
}

// #region server

// newServer настраивает таймауты: без них медленный или зависший клиент держит
// соединение и горутину сервера сколько угодно. tlsCfg == nil — HTTP/1.1 без TLS.
func newServer(h http.Handler, tlsCfg *tls.Config, log *slog.Logger) *http.Server {
	return &http.Server{
		Handler: h,
		// HTTP/2 включается сам: ServeTLS добавляет "h2" в ALPN, если TLSNextProto не задан.
		TLSConfig: tlsCfg,
		// Заголовки должны прийти за 5 с — защита от Slowloris, который шлёт их по байту.
		ReadHeaderTimeout: 5 * time.Second,
		// Весь запрос вместе с телом.
		ReadTimeout: 30 * time.Second,
		// От конца чтения заголовков до конца записи ответа: клиент, который не читает
		// ответ, не держит обработчик вечно. В HTTP/2 Read/WriteTimeout действуют на поток.
		WriteTimeout: 30 * time.Second,
		// Сколько держать keep-alive-соединение без запросов.
		IdleTimeout: 90 * time.Second,
		// Ошибки самого сервера (например, провал TLS-рукопожатия) — в тот же slog.
		ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
}

// #endregion server

// endpoint — сервер и его листенер. TLS включён, если у сервера задан TLSConfig.
type endpoint struct {
	srv *http.Server
	ln  net.Listener
}

// serve обслуживает все endpoint'ы до отмены ctx или до падения любого из них, затем
// останавливает все. Горутинами Serve владеет wg; serve возвращается только после их выхода.
func serve(ctx context.Context, grace time.Duration, log *slog.Logger, eps ...endpoint) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, len(eps))
	var wg sync.WaitGroup
	for _, ep := range eps {
		wg.Go(func() {
			var err error
			if ep.srv.TLSConfig != nil {
				err = ep.srv.ServeTLS(ep.ln, "", "") // сертификат уже в TLSConfig
			} else {
				err = ep.srv.Serve(ep.ln)
			}
			if !errors.Is(err, http.ErrServerClosed) {
				errs <- fmt.Errorf("serve %s: %w", ep.ln.Addr(), err)
				cancel() // упал один — останавливаем остальные
			}
		})
	}

	<-ctx.Done()
	log.Info("остановка: жду активные запросы", "grace", grace)
	shutdown(grace, log, eps)
	wg.Wait()
	close(errs)
	log.Info("остановлен")

	var err error
	for e := range errs {
		err = errors.Join(err, e)
	}
	return err
}

// shutdown останавливает серверы параллельно, чтобы grace был общим, а не на каждый.
// Контекст — от Background: родительский уже отменён, с ним Shutdown не ждал бы ни мгновения.
func shutdown(grace time.Duration, log *slog.Logger, eps []endpoint) {
	ctx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	var wg sync.WaitGroup
	for _, ep := range eps {
		wg.Go(func() {
			addr := ep.ln.Addr().String()
			// Shutdown закрывает листенер и простаивающие соединения, ждёт активные.
			err := ep.srv.Shutdown(ctx)
			if err == nil {
				return
			}
			log.Warn("не уложились в grace, закрываю соединения", "addr", addr, "err", err)
			if err := ep.srv.Close(); err != nil {
				log.Warn("принудительное закрытие", "addr", addr, "err", err)
			}
		})
	}
	wg.Wait()
}

// handlers — обработчики лабораторной; log нужен, чтобы не терять ошибки записи ответа.
type handlers struct {
	log *slog.Logger
}

func newHandler(log *slog.Logger) http.Handler {
	h := handlers{log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.root)
	mux.HandleFunc("GET /bytes", h.bytes)
	mux.HandleFunc("GET /slow", h.slow)
	return logRequests(mux, log)
}

func (h handlers) root(w http.ResponseWriter, r *http.Request) {
	h.reply(w, r, "proto=%s remote=%s\n", r.Proto, r.RemoteAddr)
}

// zeroChunk только читается: ответ /bytes пишется кусками, без буфера на весь размер.
var zeroChunk [32 << 10]byte

func (h handlers) bytes(w http.ResponseWriter, r *http.Request) {
	n, err := intParam(r, "n", maxBytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(n))
	for left := n; left > 0; {
		k := min(left, len(zeroChunk))
		if _, err := w.Write(zeroChunk[:k]); err != nil {
			h.log.Warn("ответ не дописан", "path", r.URL.Path, "sent", n-left, "of", n, "err", err)
			return
		}
		left -= k
	}
}

func (h handlers) slow(w http.ResponseWriter, r *http.Request) {
	ms, err := intParam(r, "ms", int(maxSlow/time.Millisecond))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-r.Context().Done():
		return // клиент ушёл или соединение закрыто: отвечать некому
	}
	h.reply(w, r, "slept=%dms\n", ms)
}

func (h handlers) reply(w http.ResponseWriter, r *http.Request, format string, args ...any) {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		h.log.Warn("ответ не записан", "path", r.URL.Path, "err", err)
	}
}

// intParam читает целый параметр из запроса в диапазоне 0..hi.
func intParam(r *http.Request, name string, hi int) (int, error) {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || v < 0 || v > hi {
		return 0, fmt.Errorf("параметр %s: ожидается целое от 0 до %d", name, hi)
	}
	return v, nil
}

// statusRecorder запоминает код ответа для журнала.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap даёт http.ResponseController добраться до исходного ResponseWriter.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func logRequests(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		attrs := []any{"method", r.Method, "path", r.URL.Path, "proto", r.Proto,
			"status", rec.status, "duration", time.Since(start)}
		// Контекст запроса отменяется, когда клиент закрыл соединение (или сервер закрыл
		// его принудительно). Ответ тогда никто не получил, и status=200 в журнале
		// без этой пометки вводил бы в заблуждение.
		if r.Context().Err() != nil {
			attrs = append(attrs, "aborted", true)
		}
		log.Info("запрос", attrs...)
	})
}
