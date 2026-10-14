package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"messenger/internal/platform/middleware"
)

type fixedIDs string

func (f fixedIDs) NewID() string { return string(f) }

func logger(buf *bytes.Buffer) *slog.Logger { return slog.New(slog.NewJSONHandler(buf, nil)) }

func respond500(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "внутренняя ошибка", http.StatusInternalServerError)
}

// accessLine — последняя строка журнала доступа (msg == "запрос") или nil.
func accessLine(t *testing.T, logs string) map[string]any {
	t.Helper()
	var access map[string]any
	for line := range strings.Lines(logs) {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err == nil && m["msg"] == "запрос" {
			access = m
		}
	}
	return access
}

func TestChainOrder(t *testing.T) {
	var trace []string
	mw := func(name string) middleware.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				trace = append(trace, name+"-in")
				next.ServeHTTP(w, r)
				trace = append(trace, name+"-out")
			})
		}
	}
	h := middleware.Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		trace = append(trace, "h")
	}), mw("a"), mw("b"), mw("c"))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	want := "a-in b-in c-in h c-out b-out a-out"
	if got := strings.Join(trace, " "); got != want {
		t.Fatalf("порядок %q, ждали %q", got, want)
	}
}

// Порядок цепочки из урока: паника должна попасть в access-log как 500 — значит,
// Recover стоит ВНУТРИ AccessLog.
func TestPanicIsLoggedAs500(t *testing.T) {
	var logs bytes.Buffer
	log := logger(&logs)
	boom := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("бум") })
	h := middleware.Chain(boom, middleware.RequestID(fixedIDs("req-1")), middleware.AccessLog(log), middleware.Recover(log, respond500))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/chats", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("статус %d, ждали 500", rec.Code)
	}
	access := accessLine(t, logs.String())
	if access == nil || access["status"] != float64(500) || access["request_id"] != "req-1" {
		t.Fatalf("строка access-log: %v", access)
	}
	if !strings.Contains(logs.String(), `"msg":"паника в обработчике"`) || !strings.Contains(logs.String(), "бум") {
		t.Fatalf("в журнале нет записи о панике: %s", logs.String())
	}
}

func TestRecoverRepanicsAbortHandler(t *testing.T) {
	defer func() {
		if v := recover(); v != http.ErrAbortHandler { //nolint:errorlint // сравниваем значение паники, а не ошибку из цепочки
			t.Fatalf("ждали повторную панику ErrAbortHandler, получили %v", v)
		}
	}()
	abort := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })
	middleware.Recover(slog.New(slog.DiscardHandler), respond500)(abort).ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
}

func TestRequestID(t *testing.T) {
	tests := []struct {
		name, incoming, want string
	}{
		{"нет заголовка — свой", "", "gen-1"},
		{"допустимый — принимаем", "abc-123_X.y", "abc-123_X.y"},
		{"перевод строки — свой", "a\nb", "gen-1"},
		{"длиннее 64 — свой", strings.Repeat("a", 65), "gen-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen string
			h := middleware.RequestID(fixedIDs("gen-1"))(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				seen = middleware.RequestIDFrom(r.Context())
			}))
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if tt.incoming != "" {
				r.Header.Set(middleware.RequestIDHeader, tt.incoming)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, r)
			if seen != tt.want || rec.Header().Get(middleware.RequestIDHeader) != tt.want {
				t.Fatalf("в context %q, в ответе %q, ждали %q", seen, rec.Header().Get(middleware.RequestIDHeader), tt.want)
			}
		})
	}
}

func TestAccessLogFields(t *testing.T) {
	var logs bytes.Buffer
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("12345"))
	})
	h := middleware.Chain(ok, middleware.RequestID(fixedIDs("req-7")), middleware.AccessLog(logger(&logs)))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/chats", nil))

	access := accessLine(t, logs.String())
	if access == nil {
		t.Fatalf("нет строки access-log: %s", logs.String())
	}
	want := map[string]any{"method": "POST", "path": "/v1/chats", "status": float64(201), "bytes": float64(5), "request_id": "req-7"}
	for k, v := range want {
		if access[k] != v {
			t.Errorf("%s = %v, ждали %v", k, access[k], v)
		}
	}
	if _, ok := access["duration"]; !ok {
		t.Error("нет поля duration")
	}
}

func TestAccessLogImplicit200(t *testing.T) {
	var logs bytes.Buffer
	h := middleware.AccessLog(logger(&logs))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/x", nil))

	if access := accessLine(t, logs.String()); access == nil || access["status"] != float64(200) {
		t.Fatalf("Write без WriteHeader — это 200: %v", access)
	}
}

func TestAccessLogNoQuery(t *testing.T) {
	var logs bytes.Buffer
	h := middleware.AccessLog(logger(&logs))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/chats?cursor=секрет", nil))

	if strings.Contains(logs.String(), "секрет") || strings.Contains(logs.String(), "cursor") {
		t.Fatalf("строка запроса попала в журнал: %s", logs.String())
	}
}

func TestRouteErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/chats", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	mux.HandleFunc("POST /v1/chats", func(http.ResponseWriter, *http.Request) {})
	var got []int
	h := middleware.RouteErrors(mux, func(w http.ResponseWriter, _ *http.Request, status int) {
		got = append(got, status)
		w.WriteHeader(status)
	})
	tests := []struct {
		method, path string
		want         int
		allow        string
	}{
		{http.MethodGet, "/v1/chats", http.StatusTeapot, ""}, // найденный маршрут — как есть
		{http.MethodGet, "/v1/nope", http.StatusNotFound, ""},
		{http.MethodDelete, "/v1/chats", http.StatusMethodNotAllowed, "GET, HEAD, POST"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil))
		if rec.Code != tt.want || rec.Header().Get("Allow") != tt.allow {
			t.Fatalf("%s %s: %d Allow=%q, ждали %d %q", tt.method, tt.path, rec.Code, rec.Header().Get("Allow"), tt.want, tt.allow)
		}
	}
	if len(got) != 2 {
		t.Fatalf("respond вызван %d раз, ждали 2 (404 и 405)", len(got))
	}
}

// Найденный маршрут обслуживает mux.ServeHTTP — только тогда хендлер видит r.Pattern.
func TestRouteErrorsKeepsPattern(t *testing.T) {
	mux := http.NewServeMux()
	var pattern string
	mux.HandleFunc("GET /v1/chats/{id}", func(_ http.ResponseWriter, r *http.Request) { pattern = r.Pattern })
	h := middleware.RouteErrors(mux, func(http.ResponseWriter, *http.Request, int) {
		t.Fatal("respond не должен вызываться")
	})

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/chats/42", nil))

	if pattern != "GET /v1/chats/{id}" {
		t.Fatalf("r.Pattern = %q", pattern)
	}
}
