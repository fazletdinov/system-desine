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
	"strings"
	"testing"
	"time"

	"messenger/internal/platform/httpjson"
	"messenger/internal/user/adapters/httpapi"
	"messenger/internal/user/domain"
	"messenger/internal/user/usecase"
)

const aliceID = "00000000-0000-7000-b000-000000000001"

// Фейки use case'ов — структуры с полями-функциями: каждый тест задаёт только то
// поведение, которое проверяет, без библиотек моков.
type fakeRegister struct {
	fn func(context.Context, usecase.RegisterUserCommand) (domain.User, error)
}

func (f fakeRegister) Execute(ctx context.Context, cmd usecase.RegisterUserCommand) (domain.User, error) {
	return f.fn(ctx, cmd)
}

type fakeGet struct {
	fn func(context.Context, usecase.GetUserQuery) (domain.User, error)
}

func (f fakeGet) Execute(ctx context.Context, q usecase.GetUserQuery) (domain.User, error) {
	return f.fn(ctx, q)
}

func alice(t *testing.T) domain.User {
	t.Helper()
	id, err := domain.ParseUserID(aliceID)
	if err != nil {
		t.Fatal(err)
	}
	name, err := domain.NewUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	display, err := domain.NewDisplayName("Алиса")
	if err != nil {
		t.Fatal(err)
	}
	// Не UTC: проверяем, что маппер сам приводит время к UTC.
	created := time.Date(2026, 10, 13, 15, 4, 5, 0, time.FixedZone("MSK", 3*60*60))
	return domain.NewUser(id, name, display, created)
}

// server собирает настоящий ServeMux с обработчиком и пишет лог в буфер.
func server(t *testing.T, reg fakeRegister, get fakeGet) (*http.ServeMux, *bytes.Buffer) {
	t.Helper()
	if reg.fn == nil {
		reg.fn = func(context.Context, usecase.RegisterUserCommand) (domain.User, error) {
			t.Fatal("RegisterUser не должен вызываться")
			return domain.User{}, nil
		}
	}
	if get.fn == nil {
		get.fn = func(context.Context, usecase.GetUserQuery) (domain.User, error) {
			t.Fatal("GetUser не должен вызываться")
			return domain.User{}, nil
		}
	}
	var logs bytes.Buffer
	h, err := httpapi.New(reg, get, slog.New(slog.NewJSONHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	return mux, &logs
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field"`
	} `json:"error"`
}

func do(t *testing.T, mux http.Handler, method, target, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var b errorBody
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("тело ошибки %q: %v", w.Body.String(), err)
	}
	return b
}

func TestRegisterUserCreated(t *testing.T) {
	var got usecase.RegisterUserCommand
	mux, _ := server(t, fakeRegister{fn: func(_ context.Context, cmd usecase.RegisterUserCommand) (domain.User, error) {
		got = cmd
		return alice(t), nil
	}}, fakeGet{})

	w := do(t, mux, http.MethodPost, "/users", `{"username":"alice","display_name":"Алиса"}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201; body %s", w.Code, w.Body)
	}
	if got != (usecase.RegisterUserCommand{Username: "alice", DisplayName: "Алиса"}) {
		t.Errorf("команда %+v", got)
	}
	want := `{"id":"` + aliceID + `","username":"alice","display_name":"Алиса","created_at":"2026-10-13T12:04:05Z"}` + "\n"
	if w.Body.String() != want {
		t.Errorf("body\n%s\nwant\n%s", w.Body, want)
	}
}

func TestGetUserOK(t *testing.T) {
	var got usecase.GetUserQuery
	mux, _ := server(t, fakeRegister{}, fakeGet{fn: func(_ context.Context, q usecase.GetUserQuery) (domain.User, error) {
		got = q
		return alice(t), nil
	}})

	w := do(t, mux, http.MethodGet, "/users/"+aliceID, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200; body %s", w.Code, w.Body)
	}
	if got.ID != aliceID {
		t.Errorf("id из пути %q, want %q", got.ID, aliceID)
	}
}

// TestErrorMapping — по строке на каждую строку таблицы «ошибка → статус».
func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantField  string
	}{
		{name: "валидация", err: &domain.ValidationError{Field: "username", Reason: "только a–z, 0–9 и _"}, wantStatus: 400, wantCode: "invalid_input", wantField: "username"},
		{name: "валидация, обёрнутая", err: fmt.Errorf("register user: %w", &domain.ValidationError{Field: "display_name", Reason: "пусто"}), wantStatus: 400, wantCode: "invalid_input", wantField: "display_name"},
		{name: "не найден", err: fmt.Errorf("get user: %w", domain.ErrUserNotFound), wantStatus: 404, wantCode: "not_found"},
		{name: "имя занято", err: fmt.Errorf("register user: %w", domain.ErrUsernameTaken), wantStatus: 409, wantCode: "username_taken"},
		{name: "дедлайн", err: fmt.Errorf("register user: %w", context.DeadlineExceeded), wantStatus: 503, wantCode: "timeout"},
		{name: "прочее", err: errors.New("секрет БД: pq: connection refused"), wantStatus: 500, wantCode: "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, _ := server(t, fakeRegister{fn: func(context.Context, usecase.RegisterUserCommand) (domain.User, error) {
				return domain.User{}, tt.err
			}}, fakeGet{})

			w := do(t, mux, http.MethodPost, "/users", `{"username":"alice","display_name":"Алиса"}`)

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			b := decodeError(t, w)
			if b.Error.Code != tt.wantCode || b.Error.Field != tt.wantField {
				t.Errorf("code %q field %q, want %q %q", b.Error.Code, b.Error.Field, tt.wantCode, tt.wantField)
			}
			if b.Error.Message == "" {
				t.Error("пустое message")
			}
		})
	}
}

func TestInternalErrorHidesDetails(t *testing.T) {
	mux, logs := server(t, fakeRegister{}, fakeGet{fn: func(context.Context, usecase.GetUserQuery) (domain.User, error) {
		return domain.User{}, errors.New("секрет БД: pq: password authentication failed")
	}})

	w := do(t, mux, http.MethodGet, "/users/"+aliceID, "")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "секрет") {
		t.Errorf("ответ раскрывает внутреннюю ошибку: %s", w.Body)
	}
	if !strings.Contains(logs.String(), "секрет БД") || !strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Errorf("в логе нет деталей на уровне ERROR: %s", logs)
	}
	if !strings.Contains(logs.String(), `"path":"/users/`+aliceID+`"`) {
		t.Errorf("в логе нет пути запроса: %s", logs)
	}
}

func TestClientGoneWritesNothing(t *testing.T) {
	// Use case, как и настоящий, возвращает ошибку своего ctx — контекста запроса.
	mux, logs := server(t, fakeRegister{fn: func(ctx context.Context, _ usecase.RegisterUserCommand) (domain.User, error) {
		return domain.User{}, fmt.Errorf("register user: %w", ctx.Err())
	}}, fakeGet{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // клиент закрыл соединение
	r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users", strings.NewReader(`{"username":"alice","display_name":"Алиса"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, r)

	if w.Body.Len() != 0 || w.Header().Get("Content-Type") != "" {
		t.Errorf("ответ записан, хотя клиент ушёл: %d %s", w.Code, w.Body)
	}
	if !strings.Contains(logs.String(), `"level":"INFO"`) {
		t.Errorf("уход клиента не залогирован на Info: %s", logs)
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
		{name: "больше 1 МиБ", contentType: "application/json", body: `{"username":"` + strings.Repeat("a", httpjson.MaxBodyBytes) + `"}`, wantStatus: 413, wantCode: "body_too_large"},
		{name: "не JSON", contentType: "application/x-www-form-urlencoded", body: `username=alice`, wantStatus: 415, wantCode: "unsupported_media_type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, _ := server(t, fakeRegister{}, fakeGet{})
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/users", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			if b := decodeError(t, w); b.Error.Code != tt.wantCode {
				t.Errorf("code %q, want %q", b.Error.Code, tt.wantCode)
			}
		})
	}
}

func TestNewNilDependencies(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	reg := fakeRegister{}
	get := fakeGet{}
	if _, err := httpapi.New(nil, get, log); err == nil {
		t.Error("New(nil register): want error")
	}
	if _, err := httpapi.New(reg, nil, log); err == nil {
		t.Error("New(nil get): want error")
	}
	if _, err := httpapi.New(reg, get, nil); err == nil {
		t.Error("New(nil log): want error")
	}
}
