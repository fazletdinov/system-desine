package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"labs/0b/internal/certs"
)

const testName = "api.messenger.example"

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantCmd string
		wantErr bool
	}{
		{"get", []string{"get", "-url", "https://x:8443/"}, "get", false},
		{"inspect", []string{"inspect", "-addr", "127.0.0.1:8443", "-server-name", "x"}, "inspect", false},
		{"без подкоманды", nil, "", true},
		{"неизвестная подкоманда", []string{"post"}, "", true},
		{"get без https", []string{"get", "-url", "http://x/"}, "", true},
		{"get без хоста", []string{"get", "-url", "https:///"}, "", true},
		{"inspect без порта", []string{"inspect", "-addr", "127.0.0.1"}, "", true},
		{"нулевой таймаут", []string{"get", "-url", "https://x/", "-timeout", "0s"}, "", true},
		{"лишние аргументы", []string{"get", "-url", "https://x/", "extra"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs(%q) err = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if err == nil && cfg.cmd != tt.wantCmd {
				t.Fatalf("cmd = %q, want %q", cfg.cmd, tt.wantCmd)
			}
		})
	}
}

func TestExplainTLS(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string // подстрока; "" — не ошибка проверки
	}{
		{"неизвестный CA", &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}, "неизвестным CA"},
		{"чужое имя", x509.HostnameError{Host: "evil.example", Certificate: &x509.Certificate{DNSNames: []string{testName}}}, `"evil.example"`},
		{"просрочен", x509.CertificateInvalidError{Reason: x509.Expired}, "срок"},
		{"не TLS", errors.New("connection refused"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := explainTLS(fmt.Errorf("get: %w", tt.err))
			if tt.want == "" && got != "" || !strings.Contains(got, tt.want) {
				t.Fatalf("explainTLS = %q, want contains %q", got, tt.want)
			}
		})
	}
}

// labServer — TLS-сервер с сертификатом от нашего CA; предлагает h2 и http/1.1, как h2server.
func labServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	ca, err := certs.NewCA("test-ca", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := ca.Issue([]string{testName}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "proto=%s\n", r.Proto) // тестовый сервер: ошибку записи увидит клиент
	}))
	ts.EnableHTTP2 = true
	ts.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"h2", "http/1.1"}}
	ts.StartTLS()
	t.Cleanup(ts.Close)
	return ts, writeCA(t, ca)
}

func writeCA(t *testing.T, ca *certs.CA) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(p, ca.PEM(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGetPrintsConnectionAndChain(t *testing.T) {
	ts, caPath := labServer(t)
	var out strings.Builder
	cfg := config{cmd: "get", url: ts.URL + "/", ca: caPath, serverName: testName, timeout: 5 * time.Second}
	if err := runGet(context.Background(), cfg, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"версия=TLS 1.3", "ALPN=h2",
		"0: subject=CN=api.messenger.example issuer=CN=test-ca",
		"1: subject=CN=test-ca issuer=CN=test-ca",
		"ответ: HTTP/2.0 200 OK", "proto=HTTP/2.0",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("в выводе нет %q:\n%s", want, out.String())
		}
	}
}

func TestGetRejectsBadCertificate(t *testing.T) {
	ts, caPath := labServer(t)
	other, err := certs.NewCA("other-ca", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, ca, serverName, want string
	}{
		{"чужое имя", caPath, "evil.example", `"evil.example"`},
		{"неизвестный CA", writeCA(t, other), testName, "неизвестным CA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config{cmd: "get", url: ts.URL + "/", ca: tt.ca, serverName: tt.serverName, timeout: 5 * time.Second}
			err := runGet(context.Background(), cfg, &strings.Builder{})
			if err == nil {
				t.Fatal("ожидалась ошибка проверки сертификата")
			}
			if got := explainTLS(err); !strings.Contains(got, tt.want) {
				t.Fatalf("explainTLS = %q, want contains %q (err %v)", got, tt.want, err)
			}
		})
	}
}

func TestInspectResumesSecondSession(t *testing.T) {
	ts, caPath := labServer(t)
	var out strings.Builder
	cfg := config{cmd: "inspect", addr: ts.Listener.Addr().String(), ca: caPath, serverName: testName, timeout: 5 * time.Second}
	if err := runInspect(context.Background(), cfg, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("ожидались две строки, получено:\n%s", out.String())
	}
	for i, want := range []string{"DidResume=false", "DidResume=true"} {
		if !strings.Contains(lines[i], want) || !strings.Contains(lines[i], "версия=TLS 1.3") || !strings.Contains(lines[i], "статус=200") {
			t.Errorf("строка %d: нет %q:\n%s", i+1, want, lines[i])
		}
	}
}
