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

// server собирает настоящий ServeMux, пишет лог в буфер и сверяет каждый ответ с контрактом.
func server(t *testing.T, signUp fakeSignUp) (http.Handler, *bytes.Buffer) {
	t.Helper()
	if signUp.fn == nil {
		signUp.fn = func(context.Context, usecase.SignUpCommand) (usecase.Profile, error) {
			t.Fatal("SignUp не должен вызываться")
			return usecase.Profile{}, nil
		}
	}
	var logs bytes.Buffer
	h, err := httpapi.New(signUp, slog.New(slog.NewJSONHandler(&logs, nil)))
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
	if _, err := httpapi.New(nil, slog.New(slog.DiscardHandler)); err == nil {
		t.Error("New(nil signUp): want error")
	}
	if _, err := httpapi.New(fakeSignUp{}, nil); err == nil {
		t.Error("New(nil log): want error")
	}
}
