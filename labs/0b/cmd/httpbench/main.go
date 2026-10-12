// Команда httpbench — нагрузочный HTTP-клиент: n запросов в c потоков, считает новые
// соединения через httptrace и показывает, как keep-alive, пул и HTTP/2 меняют задержку.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"labs/0b/internal/stats"
)

type config struct {
	url            string
	ca             string
	serverName     string
	n              int
	c              int
	keepalive      bool
	maxIdlePerHost int
	timeout        time.Duration
	http2          bool
}

func parseFlags(args []string) (config, error) {
	var c config
	fs := flag.NewFlagSet("httpbench", flag.ContinueOnError)
	fs.StringVar(&c.url, "url", "https://172.28.0.10:8443/", "адрес запроса (http или https)")
	fs.StringVar(&c.ca, "ca", "", "PEM с доверенным CA; пусто — системные корни")
	fs.StringVar(&c.serverName, "server-name", "", "имя для SNI и проверки сертификата; пусто — хост из -url")
	fs.IntVar(&c.n, "n", 100, "сколько запросов всего")
	fs.IntVar(&c.c, "c", 10, "сколько запросов одновременно (воркеров)")
	fs.BoolVar(&c.keepalive, "keepalive", true, "переиспользовать соединения")
	fs.IntVar(&c.maxIdlePerHost, "max-idle-per-host", 2, "сколько простаивающих соединений к хосту держать в пуле")
	fs.DurationVar(&c.timeout, "timeout", 5*time.Second, "таймаут запроса целиком, от подключения до конца тела")
	fs.BoolVar(&c.http2, "http2", false, "разрешить HTTP/2 (ALPN h2)")
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
	if u, err := url.Parse(c.url); err != nil {
		errs = append(errs, fmt.Errorf("-url: %w", err))
	} else if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, fmt.Errorf("-url: нужен адрес вида https://host:port/, получено %q", c.url))
	}
	if c.n <= 0 {
		errs = append(errs, fmt.Errorf("-n должен быть больше нуля, получено %d", c.n))
	}
	if c.c <= 0 {
		errs = append(errs, fmt.Errorf("-c должен быть больше нуля, получено %d", c.c))
	}
	// Ноль транспорт понимает как «по умолчанию, 2» — явная ошибка понятнее скрытой подмены.
	if c.maxIdlePerHost <= 0 {
		errs = append(errs, fmt.Errorf("-max-idle-per-host должен быть больше нуля, получено %d", c.maxIdlePerHost))
	}
	if c.timeout <= 0 {
		errs = append(errs, fmt.Errorf("-timeout должен быть больше нуля, получено %v", c.timeout))
	}
	return errors.Join(errs...)
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

	if err := run(ctx, cfg, os.Stdout); err != nil {
		log.Error("httpbench", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config, out io.Writer) error {
	rep, err := bench(ctx, cfg)
	if rep.requests > 0 {
		printReport(out, rep) // и при отмене: частичный результат тоже полезен
	}
	return err
}

func loadRoots(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil // nil в tls.Config.RootCAs — системные корни
	}
	pemData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemData) {
		return nil, fmt.Errorf("read CA: в %s нет PEM-сертификатов", path)
	}
	return pool, nil
}

// #region transport

func newTransport(cfg config, roots *x509.CertPool) *http.Transport {
	tr := &http.Transport{
		Proxy:       http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{Timeout: cfg.timeout, KeepAlive: 30 * time.Second}).DialContext,
		// Пул: всего простаивающих соединений и на один хост. Если воркеров больше, чем
		// MaxIdleConnsPerHost, лишние соединения после ответа закрываются, и следующий
		// запрос платит новое рукопожатие.
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: cfg.maxIdlePerHost,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
		// Время от отправки запроса до заголовков ответа.
		ResponseHeaderTimeout: cfg.timeout,
		DisableKeepAlives:     !cfg.keepalive,
		TLSClientConfig: &tls.Config{
			RootCAs:    roots,
			ServerName: cfg.serverName, // пусто — берётся хост из URL
			MinVersion: tls.VersionTLS12,
		},
		// Со своим TLSClientConfig транспорт сам HTTP/2 не включает — просим явно.
		ForceAttemptHTTP2: cfg.http2,
	}
	if !cfg.http2 {
		// Пустая, но не nil карта — документированный способ запретить HTTP/2:
		// клиент не предложит h2 в ALPN, даже если сервер его умеет. Иначе сравнение
		// с HTTP/1.1 было бы нечестным.
		tr.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	}
	return tr
}

// #endregion transport

// #region trace

// connStats собирает события соединений со всех запросов. Колбэки httptrace вызываются
// из горутин транспорта, в том числе из горутины установки соединения, которая может
// пережить сам запрос, — поэтому всё под мьютексом.
type connStats struct {
	mu                 sync.Mutex
	newConns           int
	dns, connect, tlsH []time.Duration
}

// newTrace возвращает трассировку для одного запроса: начало каждой фазы хранится
// в замыкании, а длительности складываются в общий connStats.
func (s *connStats) newTrace() *httptrace.ClientTrace {
	// Одно начало на фазу: для IP-адреса или имени с одним адресом попытка соединения одна.
	// Если имя даёт и IPv4, и IPv6, Happy Eyeballs может начать две — замер тогда приблизителен.
	var dnsStart, connStart, tlsStart time.Time
	return &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) { s.mark(&dnsStart) },
		DNSDone: func(info httptrace.DNSDoneInfo) {
			s.since(&s.dns, &dnsStart, info.Err)
		},
		ConnectStart: func(string, string) { s.mark(&connStart) },
		ConnectDone: func(_, _ string, err error) {
			s.since(&s.connect, &connStart, err)
		},
		TLSHandshakeStart: func() { s.mark(&tlsStart) },
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			s.since(&s.tlsH, &tlsStart, err)
		},
		// GotConn вызывается для каждого запроса: Reused == false — соединение новое.
		GotConn: func(info httptrace.GotConnInfo) {
			if info.Reused {
				return
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			s.newConns++
		},
	}
}

func (s *connStats) mark(t *time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	*t = time.Now()
}

// since записывает длительность успешной фазы; неудачную покажет ошибка запроса.
func (s *connStats) since(dst *[]time.Duration, start *time.Time, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil && !start.IsZero() {
		*dst = append(*dst, time.Since(*start))
	}
}

// #endregion trace

// errClass — вид ошибки: по нему видно, что лечить — сеть, сертификат или сервер.
type errClass string

const (
	classTimeout errClass = "timeout"
	classRefused errClass = "refused"
	classTLS     errClass = "tls"
	classStatus  errClass = "status"
	classOther   errClass = "other"
)

var errClasses = []errClass{classTimeout, classRefused, classTLS, classStatus, classOther}

// statusError — ответ пришёл, но не 2xx.
type statusError struct{ code int }

func (e *statusError) Error() string { return fmt.Sprintf("HTTP %d", e.code) }

func classify(err error) errClass {
	var (
		status    *statusError
		verify    *tls.CertificateVerificationError
		unknownCA x509.UnknownAuthorityError
		hostname  x509.HostnameError
		invalid   x509.CertificateInvalidError
		alert     tls.AlertError
		netErr    net.Error
	)
	switch {
	case errors.As(err, &status):
		return classStatus
	case errors.As(err, &verify), errors.As(err, &unknownCA), errors.As(err, &hostname),
		errors.As(err, &invalid), errors.As(err, &alert):
		return classTLS
	case errors.Is(err, syscall.ECONNREFUSED):
		return classRefused
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, os.ErrDeadlineExceeded),
		errors.As(err, &netErr) && netErr.Timeout():
		return classTimeout
	default:
		return classOther
	}
}

// result — исход одного запроса; done == false — до запроса дело не дошло (отмена).
type result struct {
	done     bool
	canceled bool
	latency  time.Duration
	proto    string
	err      error
}

type report struct {
	requests  int // завершённые запросы, успешные и нет; отменённые не считаются
	errors    map[errClass]int
	firstErr  map[errClass]error // по одному примеру на вид — чтобы было что читать
	newConns  int
	protos    map[string]int
	latencies []time.Duration // только успешные запросы
	dns       []time.Duration
	connect   []time.Duration
	tls       []time.Duration
	total     time.Duration
}

func (r report) failed() int {
	var n int
	for _, v := range r.errors {
		n += v
	}
	return n
}

func (r report) proto() string {
	if len(r.protos) == 0 {
		return "-"
	}
	return strings.Join(slices.Sorted(maps.Keys(r.protos)), ",")
}

// bench выполняет cfg.n запросов в min(cfg.c, cfg.n) воркеров. Воркерами владеет bench:
// он ждёт их через WaitGroup; отмена ctx прерывает запросы в полёте и останавливает выдачу новых.
func bench(ctx context.Context, cfg config) (report, error) {
	roots, err := loadRoots(cfg.ca)
	if err != nil {
		return report{}, err
	}
	tr := newTransport(cfg, roots)
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: cfg.timeout}

	var st connStats
	jobs := make(chan int, cfg.n)
	for i := range cfg.n {
		jobs <- i
	}
	close(jobs)
	results := make([]result, cfg.n) // воркер пишет только в свой индекс — без гонок

	start := time.Now()
	var wg sync.WaitGroup
	for range min(cfg.c, cfg.n) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					return
				}
				results[i] = do(ctx, client, cfg.url, &st)
			}
		}()
	}
	wg.Wait()

	rep := aggregate(results)
	rep.total = time.Since(start)
	st.mu.Lock()
	rep.newConns = st.newConns
	rep.dns, rep.connect, rep.tls = slices.Clone(st.dns), slices.Clone(st.connect), slices.Clone(st.tlsH)
	st.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return rep, fmt.Errorf("прервано после %d запросов: %w", rep.requests, err)
	}
	if rep.failed() == rep.requests {
		return rep, fmt.Errorf("все %d запросов завершились ошибкой", rep.requests)
	}
	return rep, nil
}

func do(ctx context.Context, client *http.Client, target string, st *connStats) result {
	ctx = httptrace.WithClientTrace(ctx, st.newTrace())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return result{done: true, err: fmt.Errorf("new request: %w", err)}
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return result{done: true, canceled: ctx.Err() != nil, err: err}
	}
	// Тело читаем до EOF и закрываем. Только так HTTP/1.1-транспорт знает, что ответ
	// закончился, и возвращает соединение в пул; недочитанное тело заставит закрыть
	// соединение, и следующий запрос заплатит за новые TCP- и TLS-рукопожатия.
	_, err = io.Copy(io.Discard, resp.Body)
	err = errors.Join(err, resp.Body.Close())
	latency := time.Since(start) // задержка — до последнего байта тела, а не до заголовков
	if err != nil {
		return result{done: true, canceled: ctx.Err() != nil, err: fmt.Errorf("read body: %w", err)}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return result{done: true, proto: resp.Proto, err: &statusError{code: resp.StatusCode}}
	}
	return result{done: true, latency: latency, proto: resp.Proto}
}

func aggregate(results []result) report {
	rep := report{errors: map[errClass]int{}, firstErr: map[errClass]error{}, protos: map[string]int{}}
	for _, r := range results {
		if !r.done || r.canceled {
			continue
		}
		rep.requests++
		if r.proto != "" {
			rep.protos[r.proto]++
		}
		if r.err != nil {
			c := classify(r.err)
			rep.errors[c]++
			if rep.firstErr[c] == nil {
				rep.firstErr[c] = r.err
			}
			continue
		}
		rep.latencies = append(rep.latencies, r.latency)
	}
	return rep
}

func printReport(out io.Writer, r report) {
	fmt.Fprintf(out, "requests=%d errors=%d new_conns=%d proto=%s\n", r.requests, r.failed(), r.newConns, r.proto())
	if s := stats.Summary(r.latencies); s.N > 0 {
		fmt.Fprintf(out, "latency: n=%d min=%v p50=%v p99=%v max=%v\n", s.N, round(s.Min), round(s.P50), round(s.P99), round(s.Max))
	}
	if r.failed() > 0 {
		parts := make([]string, 0, len(errClasses))
		for _, c := range errClasses {
			parts = append(parts, fmt.Sprintf("%s=%d", c, r.errors[c]))
		}
		fmt.Fprintf(out, "errors: %s\n", strings.Join(parts, " "))
		for _, c := range errClasses {
			if err := r.firstErr[c]; err != nil {
				fmt.Fprintf(out, "  %s: %v\n", c, err)
			}
		}
	}
	for _, ph := range []struct {
		name string
		ds   []time.Duration
	}{{"dns", r.dns}, {"connect", r.connect}, {"tls", r.tls}} {
		if s := stats.Summary(ph.ds); s.N > 0 {
			fmt.Fprintf(out, "%s: n=%d min=%v p50=%v max=%v\n", ph.name, s.N, round(s.Min), round(s.P50), round(s.Max))
		}
	}
	fmt.Fprintf(out, "total=%v\n", round(r.total))
}

// round до 10 мкс: точнее сетевой замер всё равно не значим, а читать легче.
func round(d time.Duration) time.Duration { return d.Round(10 * time.Microsecond) }
