package problem_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/middleware"
	"messenger/internal/platform/problem"
)

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != problem.ContentType {
		t.Fatalf("Content-Type %q, ждали %q", ct, problem.ContentType)
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func newRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(t.Context(), method, target, nil)
}

func TestWriteFields(t *testing.T) {
	r := newRequest(t, http.MethodPost, "/v1/chats?x=1")
	var got map[string]any
	h := middleware.RequestID(idFunc("req-7"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := problem.Write(w, r, problem.Invalid("text", "от 1 до 4096 символов")); err != nil {
			t.Fatal(err)
		}
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	got = decode(t, rec)
	want := map[string]any{
		"type": problem.TypeBase + "invalid_input", "title": "Некорректные данные", "status": float64(400),
		"detail": "от 1 до 4096 символов", "instance": "/v1/chats", "code": "invalid_input", "request_id": "req-7",
		"invalid_params": []any{map[string]any{"name": "text", "reason": "от 1 до 4096 символов"}},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("тело\n%v\nждали\n%v", got, want)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("статус %d", rec.Code)
	}
}

type idFunc string

func (f idFunc) NewID() string { return string(f) }

func TestWriteUnknownCode(t *testing.T) {
	rec := httptest.NewRecorder()
	err := problem.Write(rec, newRequest(t, http.MethodGet, "/v1/chats"), problem.Problem{Code: "no_such_code", Detail: "секрет"})
	if err == nil {
		t.Fatal("незарегистрированный код должен вернуть ошибку")
	}
	m := decode(t, rec)
	if rec.Code != http.StatusInternalServerError || m["code"] != string(problem.CodeInternal) {
		t.Fatalf("%d %v, ждали 500 internal", rec.Code, m["code"])
	}
	if strings.Contains(rec.Body.String(), "секрет") {
		t.Fatal("detail незарегистрированной проблемы утёк в ответ")
	}
}

func TestEveryCodeHasStatusAndTitle(t *testing.T) {
	for _, c := range problem.Codes() {
		t.Run(string(c), func(t *testing.T) {
			rec := httptest.NewRecorder()
			if err := problem.Write(rec, newRequest(t, http.MethodGet, "/"), problem.Problem{Code: c}); err != nil {
				t.Fatal(err)
			}
			m := decode(t, rec)
			if rec.Code < 400 || rec.Code > 599 || m["status"] != float64(rec.Code) {
				t.Fatalf("статус ответа %d, в теле %v", rec.Code, m["status"])
			}
			if title, _ := m["title"].(string); title == "" {
				t.Fatal("пустой title")
			}
		})
	}
}

func TestRespondPolicy(t *testing.T) {
	never := func(error) (problem.Problem, bool) { return problem.Problem{}, false }
	secret := errors.New("pq: password authentication failed for host db-1")
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"разбор тела", &httpjson.DecodeError{Status: 413, Code: "body_too_large", Msg: "тело больше 1048576 байт"}, 413, "body_too_large"},
		{"дедлайн", fmt.Errorf("x: %w", context.DeadlineExceeded), 503, "timeout"},
		{"чужой Canceled при живом клиенте", fmt.Errorf("x: %w", context.Canceled), 500, "internal"},
		{"всё прочее", secret, 500, "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			problem.Respond(rec, newRequest(t, http.MethodGet, "/v1/chats"), slog.New(slog.DiscardHandler), tt.err, never)
			m := decode(t, rec)
			if rec.Code != tt.status || m["code"] != tt.code {
				t.Fatalf("%d %v, ждали %d %s", rec.Code, m["code"], tt.status, tt.code)
			}
			if strings.Contains(rec.Body.String(), "db-1") {
				t.Fatal("внутренняя ошибка утекла в ответ")
			}
		})
	}
}

func TestRespondClassifierWins(t *testing.T) {
	errKnown := errors.New("известная ошибка")
	classify := func(err error) (problem.Problem, bool) {
		if errors.Is(err, errKnown) {
			return problem.Problem{Code: problem.CodeConflict, Detail: "известная"}, true
		}
		return problem.Problem{}, false
	}
	rec := httptest.NewRecorder()
	// Таблица контекста важнее контекста запроса: доменная ошибка, завёрнутая вместе с Canceled.
	problem.Respond(rec, newRequest(t, http.MethodGet, "/v1/chats"), slog.New(slog.DiscardHandler), fmt.Errorf("%w: %w", errKnown, context.Canceled), classify)
	m := decode(t, rec)
	if rec.Code != http.StatusConflict || m["code"] != "conflict" || m["detail"] != "известная" {
		t.Fatalf("%d %v", rec.Code, m)
	}
}

func TestRespondSilentWhenClientGone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	rec := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/chats", nil)
	problem.Respond(rec, r, slog.New(slog.DiscardHandler), fmt.Errorf("x: %w", context.Canceled), func(error) (problem.Problem, bool) { return problem.Problem{}, false })
	if rec.Body.Len() != 0 || rec.Header().Get("Content-Type") != "" {
		t.Fatalf("клиент ушёл — отвечать некому, а записано: %q", rec.Body.String())
	}
}

// Реестр кодов в Go и перечисление ProblemCode в контракте — одно знание в двух местах;
// тест не даёт им разойтись.
func TestCodesMatchContract(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	schema := doc.Components.Schemas["ProblemCode"]
	if schema == nil {
		t.Fatal("в контракте нет схемы ProblemCode")
	}
	var inContract []string
	for _, v := range schema.Value.Enum {
		inContract = append(inContract, v.(string))
	}
	var inGo []string
	for _, c := range problem.Codes() {
		inGo = append(inGo, string(c))
	}
	slices.Sort(inContract)
	if !slices.Equal(inGo, inContract) {
		t.Fatalf("коды в Go %v, в контракте %v", inGo, inContract)
	}
}
