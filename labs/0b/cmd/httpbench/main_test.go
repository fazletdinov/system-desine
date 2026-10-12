package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"labs/0b/internal/certs"
)

const testName = "api.messenger.example"

func TestValidate(t *testing.T) {
	ok := config{url: "https://x/", n: 1, c: 1, maxIdlePerHost: 1, timeout: time.Second}
	tests := []struct {
		name    string
		mutate  func(*config)
		wantErr bool
	}{
		{"корректная конфигурация", func(*config) {}, false},
		{"http без TLS", func(c *config) { c.url = "http://x:8080/" }, false},
		{"n равно нулю", func(c *config) { c.n = 0 }, true},
		{"c равно нулю", func(c *config) { c.c = 0 }, true},
		{"нулевой таймаут", func(c *config) { c.timeout = 0 }, true},
		{"max-idle-per-host ноль", func(c *config) { c.maxIdlePerHost = 0 }, true},
		{"схема не http(s)", func(c *config) { c.url = "ftp://x/" }, true},
		{"без хоста", func(c *config) { c.url = "https:///" }, true},
		{"мусор вместо URL", func(c *config) { c.url = "::" }, true},
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

func TestClassify(t *testing.T) {
	wrap := func(err error) error { return &url.Error{Op: "Get", URL: "https://x/", Err: err} }
	tests := []struct {
		name string
		err  error
		want errClass
	}{
		{"дедлайн контекста", wrap(context.DeadlineExceeded), classTimeout},
		{"дедлайн сокета", wrap(&net.OpError{Op: "read", Err: os.ErrDeadlineExceeded}), classTimeout},
		{"отказ соединения", wrap(&net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}), classRefused},
		{"неизвестный CA", wrap(&tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}), classTLS},
		{"чужое имя", wrap(x509.HostnameError{Host: "evil.example"}), classTLS},
		{"просрочен", wrap(x509.CertificateInvalidError{Reason: x509.Expired}), classTLS},
		{"HTTP-статус", &statusError{code: 503}, classStatus},
		{"прочее", wrap(errors.New("boom")), classOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(tt.err); got != tt.want {
				t.Fatalf("classify(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

// labServer — TLS-сервер с сертификатом от нашего CA; предлагает h2 и http/1.1, как h2server.
func labServer(t *testing.T, h http.Handler) (srvURL, caPath string) {
	t.Helper()
	ca, err := certs.NewCA("test-ca", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := ca.Issue([]string{testName}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	caPath = filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, ca.PEM(), 0o600); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(h)
	ts.EnableHTTP2 = true
	ts.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"h2", "http/1.1"}}
	ts.StartTLS()
	t.Cleanup(ts.Close)
	return ts.URL + "/", caPath
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "ok") // тестовый сервер: ошибка записи проявится у клиента
	})
}

func TestBenchCountsConnections(t *testing.T) {
	u, caPath := labServer(t, okHandler())
	tests := []struct {
		name         string
		n, c         int
		keepalive    bool
		maxIdle      int
		http2        bool
		wantProto    string
		wantNewConns func(int) bool
		want         string
	}{
		{"без keep-alive — соединение на запрос", 10, 1, false, 2, false, "HTTP/1.1", func(n int) bool { return n == 10 }, "10"},
		{"keep-alive, один воркер — одно соединение", 10, 1, true, 2, false, "HTTP/1.1", func(n int) bool { return n == 1 }, "1"},
		{"keep-alive, пул вмещает всех воркеров", 50, 5, true, 5, false, "HTTP/1.1", func(n int) bool { return n >= 1 && n <= 5 }, "1..5"},
		{"HTTP/2 мультиплексирует в одно соединение", 50, 5, true, 2, true, "HTTP/2.0", func(n int) bool { return n == 1 }, "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config{url: u, ca: caPath, serverName: testName, n: tt.n, c: tt.c, keepalive: tt.keepalive,
				maxIdlePerHost: tt.maxIdle, timeout: 5 * time.Second, http2: tt.http2}
			rep, err := bench(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			if rep.requests != tt.n || rep.failed() != 0 {
				t.Fatalf("requests=%d errors=%v", rep.requests, rep.errors)
			}
			if !tt.wantNewConns(rep.newConns) {
				t.Fatalf("new_conns = %d, want %s", rep.newConns, tt.want)
			}
			if got := rep.proto(); got != tt.wantProto {
				t.Fatalf("proto = %q, want %q", got, tt.wantProto)
			}
			if len(rep.latencies) != tt.n {
				t.Fatalf("latencies = %d, want %d", len(rep.latencies), tt.n)
			}
			// new_conns — нижняя граница: TCP-соединений установлено не меньше. TLS-рукопожатий
			// бывает меньше, чем TCP: ненужный уже набор транспорт может бросить посреди TLS.
			if len(rep.connect) < rep.newConns || len(rep.tls) > len(rep.connect) {
				t.Fatalf("tcp_conns=%d tls=%d new_conns=%d", len(rep.connect), len(rep.tls), rep.newConns)
			}
		})
	}
}

// Пул на одно соединение при 20 воркерах: вернувшиеся соединения не помещаются в пул
// и закрываются, и соединений устанавливается заметно больше, чем воркеров и чем с пулом
// на всех. Точных чисел тест не ждёт: даже с большим пулом бывают лишние наборы —
// запрос иногда приходит раньше, чем его прошлое соединение вернулось в пул.
// Пауза в обработчике разводит воркеров по фазе, чтобы пул успевал переполниться.
func TestPoolLimitForcesRedials(t *testing.T) {
	u, caPath := labServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Millisecond)
		_, _ = fmt.Fprint(w, "ok") // тестовый сервер: ошибка записи проявится у клиента
	}))
	const workers = 20
	dials := func(maxIdle int) int {
		cfg := config{url: u, ca: caPath, serverName: testName, n: 400, c: workers, keepalive: true,
			maxIdlePerHost: maxIdle, timeout: 5 * time.Second}
		rep, err := bench(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		return len(rep.connect)
	}
	small, full := dials(1), dials(workers)
	if small <= workers || small <= 2*full {
		t.Fatalf("tcp_conns: пул на 1 — %d, пул на %d — %d; ожидалось больше %d и больше вдвое", small, workers, full, workers)
	}
}

func TestBenchClassifiesFailures(t *testing.T) {
	u, caPath := labServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("mode") {
		case "500":
			http.Error(w, "fail", http.StatusInternalServerError)
		case "hang":
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		default:
			_, _ = fmt.Fprint(w, "ok")
		}
	}))
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	refusedURL := "https://" + closed.Addr().String() + "/"
	_ = closed.Close() // порт свободен — подключение получит RST

	tests := []struct {
		name       string
		url        string
		serverName string
		timeout    time.Duration
		want       errClass
	}{
		{"порт закрыт", refusedURL, testName, time.Second, classRefused},
		{"имя не совпадает с сертификатом", u, "evil.example", time.Second, classTLS},
		{"сервер отвечает 500", u + "?mode=500", testName, time.Second, classStatus},
		{"сервер не отвечает", u + "?mode=hang", testName, 100 * time.Millisecond, classTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config{url: tt.url, ca: caPath, serverName: tt.serverName, n: 4, c: 2, keepalive: true,
				maxIdlePerHost: 2, timeout: tt.timeout}
			rep, err := bench(context.Background(), cfg)
			if err == nil {
				t.Fatal("bench должен вернуть ошибку, когда все запросы провалились")
			}
			if rep.requests != 4 || rep.errors[tt.want] != 4 {
				t.Fatalf("requests=%d errors=%v, want 4 × %s", rep.requests, rep.errors, tt.want)
			}
			if rep.firstErr[tt.want] == nil {
				t.Fatal("пример ошибки не сохранён")
			}
		})
	}
}

func TestBenchStopsOnCancel(t *testing.T) {
	u, caPath := labServer(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // отвечает, только когда клиент ушёл
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	cfg := config{url: u, ca: caPath, serverName: testName, n: 100, c: 4, keepalive: true, maxIdlePerHost: 4, timeout: time.Minute}
	start := time.Now()
	rep, err := bench(ctx, cfg)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bench = %v, want context.DeadlineExceeded", err)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("воркеры не остановились по отмене: %v", el)
	}
	if rep.failed() != 0 {
		t.Fatalf("отменённые запросы не должны считаться ошибками: %v", rep.errors)
	}
}

func TestPrintReport(t *testing.T) {
	rep := report{
		requests:  3,
		errors:    map[errClass]int{classTimeout: 1},
		firstErr:  map[errClass]error{classTimeout: errors.New("deadline")},
		newConns:  2,
		protos:    map[string]int{"HTTP/1.1": 2},
		latencies: []time.Duration{40 * time.Millisecond, 41 * time.Millisecond},
		connect:   []time.Duration{20 * time.Millisecond},
		total:     time.Second,
	}
	var b strings.Builder
	if err := printReport(&b, rep); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{
		"requests=3 errors=1 tcp_conns=1 new_conns=2 proto=HTTP/1.1\n",
		"latency: n=2 min=40ms p50=40ms p99=41ms max=41ms\n",
		"errors: timeout=1 refused=0 tls=0 status=0 other=0\n",
		"  timeout: deadline\n",
		"connect: n=1",
		"total=1s\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("в выводе нет %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "dns:") {
		t.Errorf("строка dns без измерений лишняя:\n%s", got)
	}
}
