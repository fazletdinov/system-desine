package health_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"messenger/internal/platform/health"
)

type checkerFunc func(ctx context.Context) error

func (f checkerFunc) Check(ctx context.Context) error { return f(ctx) }

var ok = checkerFunc(func(context.Context) error { return nil })

func serve(t *testing.T, p *health.Probes, method, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	p.Register(mux, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusMethodNotAllowed) })
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, path, nil))
	var body map[string]any
	if method != http.MethodHead && rec.Code != http.StatusMethodNotAllowed {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("тело %q: %v", rec.Body, err)
		}
	}
	return rec, body
}

func newProbes(t *testing.T, logs *bytes.Buffer, checks ...health.Check) *health.Probes {
	t.Helper()
	p, err := health.New(slog.New(slog.NewJSONHandler(logs, nil)), checks...)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLiveIgnoresDependencies(t *testing.T) {
	down := checkerFunc(func(context.Context) error { return errors.New("postgres: connection refused") })
	p := newProbes(t, &bytes.Buffer{}, health.Check{Name: "postgres", Checker: down})
	p.StartDraining()

	rec, body := serve(t, p, http.MethodGet, health.LivePath)

	if rec.Code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("livez: %d %v — живость не зависит ни от зависимостей, ни от дренажа", rec.Code, body)
	}
}

func TestReady(t *testing.T) {
	down := checkerFunc(func(context.Context) error { return errors.New("dial tcp 10.0.0.5:5432: password=секрет") })
	cases := []struct {
		name   string
		checks []health.Check
		drain  bool
		status int
		failed map[string]any
	}{
		{"проверки прошли", []health.Check{{Name: "postgres", Checker: ok}}, false, http.StatusOK, nil},
		{"без проверок", nil, false, http.StatusOK, nil},
		{"зависимость недоступна", []health.Check{{Name: "postgres", Checker: ok}, {Name: "redis", Checker: down}}, false, http.StatusServiceUnavailable, map[string]any{"redis": "fail"}},
		{"дренаж", []health.Check{{Name: "postgres", Checker: ok}}, true, http.StatusServiceUnavailable, map[string]any{"shutdown": "draining"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logs := &bytes.Buffer{}
			p := newProbes(t, logs, c.checks...)
			if c.drain {
				p.StartDraining()
			}

			rec, body := serve(t, p, http.MethodGet, health.ReadyPath)

			if rec.Code != c.status {
				t.Fatalf("статус %d, ждали %d; тело %v", rec.Code, c.status, body)
			}
			if c.failed == nil && body["checks"] != nil {
				t.Fatalf("checks в успешном ответе: %v", body)
			}
			if c.failed != nil {
				got, _ := body["checks"].(map[string]any)
				if len(got) != len(c.failed) {
					t.Fatalf("checks %v, ждали %v", got, c.failed)
				}
				for k, v := range c.failed {
					if got[k] != v {
						t.Fatalf("checks %v, ждали %v", got, c.failed)
					}
				}
			}
			// Причина — в журнал, наружу — только имя проверки.
			if strings.Contains(rec.Body.String(), "секрет") || strings.Contains(rec.Body.String(), "10.0.0.5") {
				t.Fatalf("внутренности в ответе: %s", rec.Body)
			}
			if c.name == "зависимость недоступна" && !strings.Contains(logs.String(), "10.0.0.5") {
				t.Fatalf("причины нет в журнале: %s", logs)
			}
			if rec.Header().Get("Cache-Control") != "no-store" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("заголовки %v", rec.Header())
			}
		})
	}
}

func TestReadyBoundsSlowCheck(t *testing.T) {
	stuck := checkerFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	p := newProbes(t, &bytes.Buffer{}, health.Check{Name: "slow", Checker: stuck})

	start := time.Now()
	rec, _ := serve(t, p, http.MethodGet, health.ReadyPath)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("статус %d, ждали 503", rec.Code)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("проба ждала %v — срок проверок не соблюдён", d)
	}
}

func TestProbesMethods(t *testing.T) {
	p := newProbes(t, &bytes.Buffer{})
	for _, path := range []string{health.LivePath, health.ReadyPath} {
		if rec, _ := serve(t, p, http.MethodHead, path); rec.Code != http.StatusOK {
			t.Errorf("HEAD %s: %d", path, rec.Code)
		}
		rec, _ := serve(t, p, http.MethodPost, path)
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
			t.Errorf("POST %s: %d, Allow=%q", path, rec.Code, rec.Header().Get("Allow"))
		}
	}
}

func TestNewRejectsBadChecks(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	for _, c := range []health.Check{{Name: "", Checker: ok}, {Name: "x", Checker: nil}} {
		if _, err := health.New(log, c); err == nil {
			t.Errorf("New(%+v): ждали ошибку", c)
		}
	}
	if _, err := health.New(nil); err == nil {
		t.Error("New без журнала: ждали ошибку")
	}
	if _, err := health.New(log, health.Check{Name: "a", Checker: ok}, health.Check{Name: "a", Checker: ok}); err == nil {
		t.Error("повтор имени: ждали ошибку")
	}
}
