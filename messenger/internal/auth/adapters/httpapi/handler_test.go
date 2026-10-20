package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"messenger/internal/auth/adapters/httpapi"
	"messenger/internal/auth/domain"
	"messenger/internal/auth/usecase"
	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/middleware"
	"messenger/internal/platform/openapitest"
	"messenger/internal/platform/problem"
)

const (
	aliceID = "00000000-0000-7000-b000-000000000001"
	// secret — пароль из тел запросов: его не должно быть ни в одном журнале.
	secret     = "очень-секретный-пароль"
	signUpBody = `{"username":"alice","display_name":"Алиса","password":"` + secret + `"}`
)

type fakeSignUp struct {
	fn func(context.Context, usecase.SignUpCommand) (usecase.Profile, error)
}

func (f fakeSignUp) Execute(ctx context.Context, cmd usecase.SignUpCommand) (usecase.Profile, error) {
	return f.fn(ctx, cmd)
}

func alice(t *testing.T) usecase.Profile {
	t.Helper()
	id, err := domain.ParseUserID(aliceID)
	if err != nil {
		t.Fatal(err)
	}
	// Не UTC: проверяем, что маппер сам приводит время к UTC.
	created := time.Date(2026, 10, 14, 15, 4, 5, 0, time.FixedZone("MSK", 3*60*60))
	return usecase.Profile{ID: id, Username: "alice", DisplayName: "Алиса", CreatedAt: created}
}

type fakeLogin struct {
	fn func(context.Context, usecase.LoginCommand) (usecase.Token, error)
}

func (f fakeLogin) Execute(ctx context.Context, cmd usecase.LoginCommand) (usecase.Token, error) {
	return f.fn(ctx, cmd)
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

var now = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// server собирает настоящий ServeMux, пишет лог в буфер и сверяет каждый ответ с контрактом.
func server(t *testing.T, signUp fakeSignUp) (http.Handler, *bytes.Buffer) {
	t.Helper()
	return newServer(t, signUp, fakeLogin{})
}

func newServer(t *testing.T, signUp fakeSignUp, login fakeLogin) (http.Handler, *bytes.Buffer) {
	t.Helper()
	if signUp.fn == nil {
		signUp.fn = func(context.Context, usecase.SignUpCommand) (usecase.Profile, error) {
			t.Fatal("SignUp не должен вызываться")
			return usecase.Profile{}, nil
		}
	}
	if login.fn == nil {
		login.fn = func(context.Context, usecase.LoginCommand) (usecase.Token, error) {
			t.Fatal("Login не должен вызываться")
			return usecase.Token{}, nil
		}
	}
	var logs bytes.Buffer
	h, err := httpapi.New(signUp, login, fixedClock{now}, slog.New(slog.NewJSONHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	return openapitest.New(t).Wrap(mux), &logs
}

func post(t *testing.T, mux http.Handler, contentType, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/users", strings.NewReader(body))
	r.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

type problemBody struct {
	Code          string `json:"code"`
	InvalidParams []struct {
		Name string `json:"name"`
	} `json:"invalid_params"`
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) problemBody {
	t.Helper()
	if ct := w.Header().Get("Content-Type"); ct != problem.ContentType {
		t.Fatalf("Content-Type %q, want %q", ct, problem.ContentType)
	}
	var b problemBody
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("тело ошибки %q: %v", w.Body.String(), err)
	}
	return b
}

func (b problemBody) field() string {
	if len(b.InvalidParams) == 0 {
		return ""
	}
	return b.InvalidParams[0].Name
}

func TestSignUpCreated(t *testing.T) {
	var got usecase.SignUpCommand
	mux, _ := server(t, fakeSignUp{fn: func(_ context.Context, cmd usecase.SignUpCommand) (usecase.Profile, error) {
		got = cmd
		return alice(t), nil
	}})

	w := post(t, mux, "application/json", signUpBody)

	if w.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201; body %s", w.Code, w.Body)
	}
	if loc := w.Header().Get("Location"); loc != "/v1/users/"+aliceID {
		t.Errorf("Location %q, want /v1/users/%s", loc, aliceID)
	}
	if got != (usecase.SignUpCommand{Username: "alice", DisplayName: "Алиса", Password: secret}) {
		t.Errorf("команда %+v", got)
	}
	var body, want any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"id":"`+aliceID+`","username":"alice","display_name":"Алиса","created_at":"2026-10-14T12:04:05Z"}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("тело %s", w.Body)
	}
}

func TestSignUpErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantField  string
	}{
		{name: "политика пароля", err: &domain.ValidationError{Field: "password", Reason: "от 15 до 128 символов"}, wantStatus: 400, wantCode: "invalid_input", wantField: "password"},
		{name: "имя от контекста user, обёрнутое", err: fmt.Errorf("sign up: %w", &domain.ValidationError{Field: "username", Reason: "только a–z, 0–9 и _"}), wantStatus: 400, wantCode: "invalid_input", wantField: "username"},
		{name: "имя занято", err: fmt.Errorf("sign up: %w", domain.ErrUsernameTaken), wantStatus: 409, wantCode: "username_taken"},
		{name: "дедлайн", err: fmt.Errorf("sign up: %w", context.DeadlineExceeded), wantStatus: 503, wantCode: "timeout"},
		{name: "профиль без пароля", err: errors.New("sign up: профиль создан без учётных данных: бум"), wantStatus: 500, wantCode: "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, _ := server(t, fakeSignUp{fn: func(context.Context, usecase.SignUpCommand) (usecase.Profile, error) {
				return usecase.Profile{}, tt.err
			}})

			w := post(t, mux, "application/json", signUpBody)

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			if b := decodeError(t, w); b.Code != tt.wantCode || b.field() != tt.wantField {
				t.Errorf("code %q field %q, want %q %q", b.Code, b.field(), tt.wantCode, tt.wantField)
			}
		})
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
		wantCode    string
	}{
		{name: "битый JSON", contentType: "application/json", body: `{"username":`, wantStatus: 400, wantCode: "bad_json"},
		{name: "неизвестное поле", contentType: "application/json", body: `{"username":"a","is_admin":true}`, wantStatus: 400, wantCode: "bad_json"},
		{name: "больше 1 МиБ", contentType: "application/json", body: `{"password":"` + strings.Repeat("a", httpjson.MaxBodyBytes) + `"}`, wantStatus: 413, wantCode: "body_too_large"},
		{name: "не JSON", contentType: "application/x-www-form-urlencoded", body: `username=alice`, wantStatus: 415, wantCode: "unsupported_media_type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, _ := server(t, fakeSignUp{})

			w := post(t, mux, tt.contentType, tt.body)

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			if b := decodeError(t, w); b.Code != tt.wantCode {
				t.Errorf("code %q, want %q", b.Code, tt.wantCode)
			}
		})
	}
}

// Пароль не должен попасть в журнал ни при каком исходе: ни при сбое use case'а,
// ни при битом теле, ни в ответе.
func TestPasswordNeverLogged(t *testing.T) {
	tests := []struct {
		name string
		body string
		err  error
	}{
		{name: "сбой use case'а", body: signUpBody, err: errors.New("секрет БД")},
		{name: "битый JSON с паролем", body: `{"password":"` + secret + `",`},
		{name: "неизвестное поле рядом с паролем", body: `{"password":"` + secret + `","x":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, logs := server(t, fakeSignUp{fn: func(context.Context, usecase.SignUpCommand) (usecase.Profile, error) {
				return usecase.Profile{}, tt.err
			}})

			w := post(t, mux, "application/json", tt.body)

			if strings.Contains(logs.String(), secret) || strings.Contains(w.Body.String(), secret) {
				t.Fatalf("пароль утёк\nжурнал: %s\nответ: %s", logs, w.Body)
			}
		})
	}
}

func TestNewNilDependencies(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	if _, err := httpapi.New(nil, fakeLogin{}, fixedClock{}, log); err == nil {
		t.Error("New(nil signUp): want error")
	}
	if _, err := httpapi.New(fakeSignUp{}, nil, fixedClock{}, log); err == nil {
		t.Error("New(nil login): want error")
	}
	if _, err := httpapi.New(fakeSignUp{}, fakeLogin{}, nil, log); err == nil {
		t.Error("New(nil clock): want error")
	}
	if _, err := httpapi.New(fakeSignUp{}, fakeLogin{}, fixedClock{}, nil); err == nil {
		t.Error("New(nil log): want error")
	}
}

const loginBody = `{"username":"alice","password":"` + secret + `"}`

func postToken(t *testing.T, h http.Handler, requestID, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/tokens", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if requestID != "" {
		r.Header.Set(middleware.RequestIDHeader, requestID)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestCreateTokenIssued(t *testing.T) {
	var got usecase.LoginCommand
	h, _ := newServer(t, fakeSignUp{}, fakeLogin{fn: func(_ context.Context, cmd usecase.LoginCommand) (usecase.Token, error) {
		got = cmd
		return usecase.Token{Value: "header.payload.signature", ExpiresAt: now.Add(15 * time.Minute)}, nil
	}})

	w := postToken(t, h, "", loginBody)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200; body %s", w.Code, w.Body)
	}
	if got != (usecase.LoginCommand{Username: "alice", Password: secret}) {
		t.Errorf("команда %+v", got)
	}
	// Токен — секрет: ни один кэш по дороге не должен его сохранить (RFC 6749 §5.1).
	if cc, pragma := w.Header().Get("Cache-Control"), w.Header().Get("Pragma"); cc != "no-store" || pragma != "no-cache" {
		t.Errorf("Cache-Control %q, Pragma %q", cc, pragma)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"access_token": "header.payload.signature", "token_type": "Bearer", "expires_in": float64(900)}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("тело %s", w.Body)
	}
}

type logRecord struct {
	Level     string `json:"level"`
	Msg       string `json:"msg"`
	Reason    string `json:"reason"`
	RequestID string `json:"request_id"`
}

func records(t *testing.T, logs *bytes.Buffer) []logRecord {
	t.Helper()
	var out []logRecord
	for line := range strings.SplitSeq(strings.TrimSpace(logs.String()), "\n") {
		if line == "" {
			continue
		}
		var rec logRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("строка журнала %q: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

// #region rejected-test
// Любая причина отказа — один и тот же ответ клиенту; различает их только журнал.
func TestCreateTokenRejected(t *testing.T) {
	reasons := []error{domain.ErrNoSuchUser, domain.ErrNoCredentials, domain.ErrWrongPassword, domain.ErrPasswordTooLong}
	var firstBody map[string]any
	seen := map[string]bool{}
	for i, reason := range reasons {
		t.Run(reason.Error(), func(t *testing.T) {
			h, logs := newServer(t, fakeSignUp{}, fakeLogin{fn: func(context.Context, usecase.LoginCommand) (usecase.Token, error) {
				return usecase.Token{}, fmt.Errorf("login: %w: %w", domain.ErrInvalidCredentials, reason)
			}})
			requestID := fmt.Sprintf("req-%d", i)

			// Свой X-Request-ID корректен, поэтому генератор id middleware не понадобится.
			w := postToken(t, middleware.RequestID(nil)(h), requestID, loginBody)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status %d, want 401; body %s", w.Code, w.Body)
			}
			if got := w.Header().Get("WWW-Authenticate"); got != `Bearer realm="messenger"` {
				t.Errorf("WWW-Authenticate %q", got)
			}
			if b := decodeError(t, w); b.Code != "invalid_credentials" {
				t.Errorf("code %q", b.Code)
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			delete(body, "request_id")
			if firstBody == nil {
				firstBody = body
			} else if !reflect.DeepEqual(body, firstBody) {
				t.Errorf("ответ отличается от первого: %v против %v", body, firstBody)
			}
			if strings.Contains(logs.String(), secret) || strings.Contains(w.Body.String(), reason.Error()) {
				t.Fatalf("утечка\nжурнал: %s\nответ: %s", logs, w.Body)
			}
			recs := records(t, logs)
			if len(recs) != 1 || recs[0].Msg != "вход отклонён" || recs[0].Level != "INFO" || recs[0].RequestID != requestID || recs[0].Reason == "" {
				t.Fatalf("журнал: %+v", recs)
			}
			seen[recs[0].Reason] = true
		})
	}
	if len(seen) != len(reasons) {
		t.Errorf("причины в журнале не различаются: %v", seen)
	}
}

// #endregion rejected-test

func TestCreateTokenErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "дедлайн", body: loginBody, err: fmt.Errorf("login: %w", context.DeadlineExceeded), wantStatus: 503, wantCode: "timeout"},
		{name: "сбой выпуска токена", body: loginBody, err: errors.New("login: выпуск токена: бум"), wantStatus: 500, wantCode: "internal"},
		{name: "неизвестное поле", body: `{"username":"alice","password":"` + secret + `","scope":"admin"}`, wantStatus: 400, wantCode: "bad_json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, logs := newServer(t, fakeSignUp{}, fakeLogin{fn: func(context.Context, usecase.LoginCommand) (usecase.Token, error) {
				return usecase.Token{}, tt.err
			}})

			w := postToken(t, h, "", tt.body)

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			if b := decodeError(t, w); b.Code != tt.wantCode {
				t.Errorf("code %q, want %q", b.Code, tt.wantCode)
			}
			if strings.Contains(logs.String(), secret) {
				t.Fatalf("пароль в журнале: %s", logs)
			}
		})
	}
}

// #region overloaded-test
// Очередь хешера полна — 503 overloaded с Retry-After: клиент (и балансировщик) знает,
// что повторить можно, и когда. Ответ сверяется с контрактом обёрткой openapitest.
func TestOverloaded(t *testing.T) {
	overloaded := func(prefix string) error {
		return fmt.Errorf("%s: хеширование: %w", prefix, usecase.ErrOverloaded)
	}
	signUp := fakeSignUp{fn: func(context.Context, usecase.SignUpCommand) (usecase.Profile, error) {
		return usecase.Profile{}, overloaded("sign up")
	}}
	login := fakeLogin{fn: func(context.Context, usecase.LoginCommand) (usecase.Token, error) {
		return usecase.Token{}, overloaded("login")
	}}
	tests := []struct {
		name string
		call func(t *testing.T, h http.Handler) *httptest.ResponseRecorder
	}{
		{"POST /v1/users", func(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
			return post(t, h, "application/json", signUpBody)
		}},
		{"POST /v1/tokens", func(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
			return postToken(t, h, "", loginBody)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newServer(t, signUp, login)

			w := tt.call(t, h)

			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("status %d, want 503; body %s", w.Code, w.Body)
			}
			if b := decodeError(t, w); b.Code != "overloaded" {
				t.Errorf("code %q, want overloaded", b.Code)
			}
			if got := w.Header().Get("Retry-After"); got != "1" {
				t.Errorf("Retry-After %q, want 1", got)
			}
		})
	}
}

// #endregion overloaded-test
