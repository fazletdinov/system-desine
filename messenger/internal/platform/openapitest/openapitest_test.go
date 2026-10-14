package openapitest_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"messenger/internal/platform/openapitest"
)

// spyTB — testing.TB, который запоминает Errorf вместо того, чтобы валить тест:
// так проверяется сама проверка — что она ругается там, где должна.
type spyTB struct {
	testing.TB
	errs []string
}

func (s *spyTB) Helper() {}

func (s *spyTB) Errorf(format string, args ...any) {
	s.errs = append(s.errs, fmt.Sprintf(format, args...))
}

const msgPath = "/v1/chats/00000000-0000-7000-c000-000000000001/messages"

func respond(status int, body string) http.Handler {
	return respondAs("application/json; charset=utf-8", status, body)
}

func respondAs(contentType string, status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

func TestWrapChecksResponse(t *testing.T) {
	const okMessage = `{"id":"m","client_message_id":"k","chat_id":"c","author_id":"a","text":"hi","seq":1,"sent_at":"2026-10-13T12:00:00Z"}`
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string // пусто — ответ по контракту
	}{
		{name: "по контракту", status: http.StatusCreated, body: okMessage},
		{name: "seq строкой", status: http.StatusCreated, body: strings.Replace(okMessage, `"seq":1`, `"seq":"1"`, 1), wantErr: "seq"},
		{name: "нет обязательного поля", status: http.StatusCreated, body: `{"id":"m"}`, wantErr: "chat_id"},
		{name: "лишнее поле", status: http.StatusCreated, body: strings.Replace(okMessage, `{`, `{"debug":true,`, 1), wantErr: "debug"},
		{name: "статуса нет в контракте", status: http.StatusTeapot, body: `{}`, wantErr: "status is not supported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := &spyTB{TB: t}
			h := openapitest.New(spy).Wrap(respond(tt.status, tt.body))
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, msgPath, nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			if w.Code != tt.status || w.Body.String() != tt.body {
				t.Errorf("Wrap изменил ответ: %d %s", w.Code, w.Body)
			}
			switch {
			case tt.wantErr == "" && len(spy.errs) > 0:
				t.Errorf("ответ по контракту отвергнут: %v", spy.errs)
			case tt.wantErr != "" && (len(spy.errs) != 1 || !strings.Contains(spy.errs[0], tt.wantErr)):
				t.Errorf("ошибки %q, want одну с %q", spy.errs, tt.wantErr)
			}
		})
	}
}

func TestWrapUnknownPath(t *testing.T) {
	tests := []struct {
		status  int
		wantErr bool
	}{
		// Ответы маршрутизатора и аутентификации приходят на любые пути.
		{status: http.StatusNotFound},
		{status: http.StatusUnauthorized},
		{status: http.StatusOK, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			spy := &spyTB{TB: t}
			h := openapitest.New(spy).Wrap(respond(tt.status, `{}`))
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/nowhere", nil)

			h.ServeHTTP(httptest.NewRecorder(), r)

			if got := len(spy.errs) > 0; got != tt.wantErr {
				t.Errorf("ошибка %v, want %v: %v", got, tt.wantErr, spy.errs)
			}
		})
	}
}

const (
	problemType = "application/problem+json"
	notFound    = `{"type":"https://messenger.example/problems/not_found","title":"Не найдено","status":404,"instance":"/v1/chats","code":"not_found"}`
)

// Тело ошибки — application/problem+json; kin-openapi разбирает его как JSON и сверяет
// со схемой Problem так же строго, как успешные ответы.
func TestWrapChecksProblem(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantErr     string
	}{
		{name: "по контракту", contentType: problemType, body: notFound},
		{name: "старый формат", contentType: problemType, body: `{"error":{"code":"not_found","message":"нет"}}`, wantErr: "error"},
		{name: "код не из реестра", contentType: problemType, body: strings.Replace(notFound, `"code":"not_found"`, `"code":"oops"`, 1), wantErr: "code"},
		{name: "обычный JSON вместо problem", contentType: "application/json", body: notFound, wantErr: "Content-Type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := &spyTB{TB: t}
			h := openapitest.New(spy).Wrap(respondAs(tt.contentType, http.StatusNotFound, tt.body))
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, msgPath, nil)

			h.ServeHTTP(httptest.NewRecorder(), r)

			switch {
			case tt.wantErr == "" && len(spy.errs) > 0:
				t.Errorf("ответ по контракту отвергнут: %v", spy.errs)
			case tt.wantErr != "" && (len(spy.errs) != 1 || !strings.Contains(spy.errs[0], tt.wantErr)):
				t.Errorf("ошибки %q, want одну с %q", spy.errs, tt.wantErr)
			}
		})
	}
}

func TestUncovered(t *testing.T) {
	v := openapitest.New(t)
	before := v.Uncovered()
	if !slices.Contains(before, "POST /v1/chats/{id}/messages") {
		t.Fatalf("до запросов операция должна быть непокрытой: %v", before)
	}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, msgPath, nil)

	v.Wrap(respondAs(problemType, http.StatusNotFound, notFound)).ServeHTTP(httptest.NewRecorder(), r)

	after := v.Uncovered()
	if slices.Contains(after, "POST /v1/chats/{id}/messages") || len(after) != len(before)-1 {
		t.Errorf("после ответа операция всё ещё непокрыта: %v", after)
	}
}
