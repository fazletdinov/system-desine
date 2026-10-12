package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"labs/0b/internal/certs"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestValidate(t *testing.T) {
	ok := config{addr: ":8443", httpAddr: ":8080", names: "server", caOut: "ca.pem", ttl: time.Hour, grace: time.Second}
	tests := []struct {
		name    string
		mutate  func(*config)
		wantErr bool
	}{
		{"корректная конфигурация", func(*config) {}, false},
		{"без имён", func(c *config) { c.names = " , " }, true},
		{"пустой путь ca.pem", func(c *config) { c.caOut = "" }, true},
		{"нулевой ttl", func(c *config) { c.ttl = 0 }, true},
		{"нулевой grace", func(c *config) { c.grace = 0 }, true},
		{"оба адреса пустые", func(c *config) { c.addr, c.httpAddr = "", "" }, true},
		{"только HTTPS", func(c *config) { c.httpAddr = "" }, false},
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

func TestSplitNames(t *testing.T) {
	got := splitNames(" api.messenger.example, server ,,")
	if strings.Join(got, "|") != "api.messenger.example|server" {
		t.Fatalf("splitNames = %q", got)
	}
}

func TestBytesHandler(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantLen    int
	}{
		{"ноль байт", "n=0", http.StatusOK, 0},
		{"килобайт", "n=1024", http.StatusOK, 1024},
		{"ровно предел", "n=" + strconv.Itoa(maxBytes), http.StatusOK, maxBytes},
		{"больше предела", "n=" + strconv.Itoa(maxBytes+1), http.StatusBadRequest, -1},
		{"отрицательное", "n=-1", http.StatusBadRequest, -1},
		{"не число", "n=abc", http.StatusBadRequest, -1},
		{"без параметра", "", http.StatusBadRequest, -1},
	}
	h := newHandler(discard)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/bytes?"+tt.query, nil))
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantLen < 0 {
				return
			}
			if rec.Body.Len() != tt.wantLen {
				t.Fatalf("len(body) = %d, want %d", rec.Body.Len(), tt.wantLen)
			}
			if cl := rec.Header().Get("Content-Length"); cl != strconv.Itoa(tt.wantLen) {
				t.Fatalf("Content-Length = %q, want %d", cl, tt.wantLen)
			}
		})
	}
}

func TestSlowHandlerValidatesDelay(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"короткая пауза", "ms=20", http.StatusOK},
		{"ноль", "ms=0", http.StatusOK},
		{"больше предела", "ms=" + strconv.Itoa(int(maxSlow/time.Millisecond)+1), http.StatusBadRequest},
		{"отрицательная", "ms=-5", http.StatusBadRequest},
		{"не число", "ms=x", http.StatusBadRequest},
	}
	h := newHandler(discard)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			start := time.Now()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/slow?"+tt.query, nil))
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.query == "ms=20" && time.Since(start) < 20*time.Millisecond {
				t.Fatalf("ответ раньше паузы: %v", time.Since(start))
			}
		})
	}
}

func TestSlowHandlerStopsWhenClientGoesAway(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/slow?ms=10000", nil)
	time.AfterFunc(20*time.Millisecond, cancel)
	start := time.Now()
	newHandler(discard).ServeHTTP(httptest.NewRecorder(), req)
	if el := time.Since(start); el > time.Second {
		t.Fatalf("обработчик не заметил отмену запроса: %v", el)
	}
}

func TestRootReportsProtocol(t *testing.T) {
	tests := []struct {
		name      string
		http2     bool
		wantProto string
	}{
		{"ALPN выбирает HTTP/2", true, "HTTP/2.0"},
		{"без HTTP/2 остаётся HTTP/1.1", false, "HTTP/1.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewUnstartedServer(newHandler(discard))
			ts.EnableHTTP2 = tt.http2
			ts.StartTLS()
			defer ts.Close()
			body := get(t, ts.Client(), ts.URL+"/")
			if !strings.HasPrefix(body, "proto="+tt.wantProto+" remote=127.0.0.1:") {
				t.Fatalf("body = %q", body)
			}
		})
	}
}

func TestUnknownPathIs404(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler(discard).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestServeHTTPSAndHTTPTogether(t *testing.T) {
	ca, err := certs.NewCA("test-ca", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := ca.Issue([]string{"api.messenger.example"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tlsLn, plainLn := listen(t), listen(t)
	h := newHandler(discard)
	eps := []endpoint{
		{srv: newServer(h, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, discard), ln: tlsLn},
		{srv: newServer(h, nil, discard), ln: plainLn},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	// Остановка займёт около секунды: HTTP/2-сервер шлёт GOAWAY и ждёт, пока клиент сам
	// закроет простаивающее соединение (goAwayTimeout в net/http), — это не таймаут grace.
	go func() { done <- serve(ctx, 5*time.Second, discard, eps...) }()

	tr := &http.Transport{
		TLSClientConfig:   &tls.Config{RootCAs: ca.Pool(), ServerName: "api.messenger.example"},
		ForceAttemptHTTP2: true,
	}
	defer tr.CloseIdleConnections()
	cli := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	if body := get(t, cli, "https://"+tlsLn.Addr().String()+"/"); !strings.HasPrefix(body, "proto=HTTP/2.0") {
		t.Errorf("HTTPS: body = %q", body)
	}
	if body := get(t, cli, "http://"+plainLn.Addr().String()+"/"); !strings.HasPrefix(body, "proto=HTTP/1.1") {
		t.Errorf("HTTP: body = %q", body)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve = %v", err)
	}
}

func TestServeShutdownLetsInFlightRequestFinish(t *testing.T) {
	ln := listen(t)
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		newHandler(discard).ServeHTTP(w, r)
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, 5*time.Second, discard, endpoint{srv: newServer(h, nil, discard), ln: ln}) }()

	type result struct {
		body string
		err  error
	}
	res := make(chan result, 1)
	go func() {
		body, err := tryGet(&http.Client{Timeout: 5 * time.Second}, "http://"+ln.Addr().String()+"/slow?ms=300")
		res <- result{body, err}
	}()
	<-started
	cancel() // остановка, пока запрос ещё внутри обработчика

	r := <-res
	if r.err != nil || !strings.HasPrefix(r.body, "slept=300ms") {
		t.Fatalf("запрос в полёте должен завершиться: body=%q err=%v", r.body, r.err)
	}
	if err := <-done; err != nil {
		t.Fatalf("serve = %v", err)
	}
	if _, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second); err == nil {
		t.Fatal("после остановки сервер не должен принимать соединения")
	}
}

func TestServeForcesCloseAfterGrace(t *testing.T) {
	ln := listen(t)
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release // обработчик, который не укладывается в grace
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, 100*time.Millisecond, discard, endpoint{srv: newServer(h, nil, discard), ln: ln})
	}()

	clientErr := make(chan error, 1)
	go func() {
		_, err := tryGet(&http.Client{Timeout: 5 * time.Second}, "http://"+ln.Addr().String()+"/")
		clientErr <- err
	}()
	<-started
	start := time.Now()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve = %v", err)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("serve ждал дольше grace: %v", el)
	}
	if err := <-clientErr; err == nil {
		t.Fatal("клиент должен получить ошибку: соединение закрыто принудительно")
	}
}

func TestServeReportsListenerFailure(t *testing.T) {
	ln := listen(t)
	_ = ln.Close() // Serve сразу получит ошибку Accept
	err := serve(context.Background(), time.Second, discard, endpoint{srv: newServer(newHandler(discard), nil, discard), ln: ln})
	if err == nil || !errors.Is(err, net.ErrClosed) {
		t.Fatalf("serve = %v, want net.ErrClosed", err)
	}
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func get(t *testing.T, c *http.Client, url string) string {
	t.Helper()
	body, err := tryGet(c, url)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func tryGet(c *http.Client, url string) (string, error) {
	resp, err := c.Get(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}
