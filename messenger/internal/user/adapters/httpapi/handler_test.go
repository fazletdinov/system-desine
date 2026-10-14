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

	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/openapitest"
	"messenger/internal/platform/problem"
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

// server собирает настоящий ServeMux с обработчиком, пишет лог в буфер и сверяет
// каждый ответ с контрактом api/openapi.yaml.
func server(t *testing.T, reg fakeRegister, get fakeGet) (http.Handler, *bytes.Buffer) {
	t.Helper()
	mux, logs := rawServer(t, reg, get)
	return openapitest.New(t).Wrap(mux), logs
}

// rawServer — без проверки по контракту: только для случая, когда ответа нет вовсе.
func rawServer(t *testing.T, reg fakeRegister, get fakeGet) (*http.ServeMux, *bytes.Buffer) {
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

// problemBody — ответ-ошибка RFC 9457 в том объёме, который проверяют тесты.
type problemBody struct {
	Title         string `json:"title"`
	Code          string `json:"code"`
	Detail        string `json:"detail"`
	RequestID     string `json:"request_id"`
	InvalidParams []struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	} `json:"invalid_params"`
}

// field — имя первого неверного параметра; пусто, если ошибка не про конкретное поле.
func (b problemBody) field() string {
	if len(b.InvalidParams) == 0 {
		return ""
	}
	return b.InvalidParams[0].Name
}

func do(t *testing.T, mux http.Handler, method, target, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

// assertJSON сравнивает тела как JSON, а не как строки: порядок полей в объекте
// значения не имеет (RFC 8259, §4), а сгенерированные типы пишут их по алфавиту.
func assertJSON(t *testing.T, got *bytes.Buffer, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got.Bytes(), &g); err != nil {
		t.Fatalf("тело %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("ожидаемое тело %q: %v", want, err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("body\n%s\nwant\n%s", got, want)
	}
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

func TestRegisterUserCreated(t *testing.T) {
	var got usecase.RegisterUserCommand
	mux, _ := server(t, fakeRegister{fn: func(_ context.Context, cmd usecase.RegisterUserCommand) (domain.User, error) {
		got = cmd
		return alice(t), nil
	}}, fakeGet{})

	w := do(t, mux, http.MethodPost, "/v1/users", `{"username":"alice","display_name":"Алиса"}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201; body %s", w.Code, w.Body)
	}
	if loc := w.Header().Get("Location"); loc != "/v1/users/"+aliceID {
		t.Errorf("Location %q, want /v1/users/%s", loc, aliceID)
	}
	if got != (usecase.RegisterUserCommand{Username: "alice", DisplayName: "Алиса"}) {
		t.Errorf("команда %+v", got)
	}
	assertJSON(t, w.Body, `{"id":"`+aliceID+`","username":"alice","display_name":"Алиса","created_at":"2026-10-13T12:04:05Z"}`)
}

func TestGetUserOK(t *testing.T) {
	var got usecase.GetUserQuery
	mux, _ := server(t, fakeRegister{}, fakeGet{fn: func(_ context.Context, q usecase.GetUserQuery) (domain.User, error) {
		got = q
		return alice(t), nil
	}})

	w := do(t, mux, http.MethodGet, "/v1/users/"+aliceID, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200; body %s", w.Code, w.Body)
	}
	if got.ID != aliceID {
		t.Errorf("id из пути %q, want %q", got.ID, aliceID)
	}
}

func TestGetUserBadID(t *testing.T) {
	mux, _ := server(t, fakeRegister{}, fakeGet{}) // до use case дело не доходит

	w := do(t, mux, http.MethodGet, "/v1/users/alice", "")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400; body %s", w.Code, w.Body)
	}
	// Поле — как параметр в контракте ({id}), а не как в домене.
	if b := decodeError(t, w); b.Code != "invalid_input" || b.field() != "id" {
		t.Errorf("code %q field %q, want invalid_input id", b.Code, b.field())
	}
}

// TestErrorMapping — по строке на каждую строку таблицы «ошибка → статус». Каждую
// ошибку возвращает операция, которая и правда может её вернуть: проверка по контракту
// не пропустит, например, 404 у регистрации — в контракте его нет.
func TestErrorMapping(t *testing.T) {
	const (
		register = http.MethodPost + " /v1/users"
		get      = http.MethodGet + " /v1/users/" + aliceID
	)
	tests := []struct {
		name       string
		route      string
		err        error
		wantStatus int
		wantCode   string
		wantField  string
	}{
		{name: "валидация", route: register, err: &domain.ValidationError{Field: "username", Reason: "только a–z, 0–9 и _"}, wantStatus: 400, wantCode: "invalid_input", wantField: "username"},
		{name: "валидация, обёрнутая", route: register, err: fmt.Errorf("register user: %w", &domain.ValidationError{Field: "display_name", Reason: "пусто"}), wantStatus: 400, wantCode: "invalid_input", wantField: "display_name"},
		{name: "не найден", route: get, err: fmt.Errorf("get user: %w", domain.ErrUserNotFound), wantStatus: 404, wantCode: "not_found"},
		{name: "имя занято", route: register, err: fmt.Errorf("register user: %w", domain.ErrUsernameTaken), wantStatus: 409, wantCode: "username_taken"},
		{name: "дедлайн", route: register, err: fmt.Errorf("register user: %w", context.DeadlineExceeded), wantStatus: 503, wantCode: "timeout"},
		{name: "прочее", route: get, err: errors.New("секрет БД: pq: connection refused"), wantStatus: 500, wantCode: "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, _ := server(t, fakeRegister{fn: func(context.Context, usecase.RegisterUserCommand) (domain.User, error) {
				return domain.User{}, tt.err
			}}, fakeGet{fn: func(context.Context, usecase.GetUserQuery) (domain.User, error) {
				return domain.User{}, tt.err
			}})
			method, target, _ := strings.Cut(tt.route, " ")

			w := do(t, mux, method, target, `{"username":"alice","display_name":"Алиса"}`)

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			b := decodeError(t, w)
			if b.Code != tt.wantCode || b.field() != tt.wantField {
				t.Errorf("code %q field %q, want %q %q", b.Code, b.field(), tt.wantCode, tt.wantField)
			}
			// detail необязателен (у 500 его нет нарочно), а title есть у каждой проблемы.
			if b.Title == "" {
				t.Error("пустой title")
			}
		})
	}
}

func TestInternalErrorHidesDetails(t *testing.T) {
	mux, logs := server(t, fakeRegister{}, fakeGet{fn: func(context.Context, usecase.GetUserQuery) (domain.User, error) {
		return domain.User{}, errors.New("секрет БД: pq: password authentication failed")
	}})

	w := do(t, mux, http.MethodGet, "/v1/users/"+aliceID, "")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "секрет") {
		t.Errorf("ответ раскрывает внутреннюю ошибку: %s", w.Body)
	}
	if !strings.Contains(logs.String(), "секрет БД") || !strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Errorf("в логе нет деталей на уровне ERROR: %s", logs)
	}
	if !strings.Contains(logs.String(), `"path":"/v1/users/`+aliceID+`"`) {
		t.Errorf("в логе нет пути запроса: %s", logs)
	}
}

func TestClientGoneWritesNothing(t *testing.T) {
	// Use case, как и настоящий, возвращает ошибку своего ctx — контекста запроса.
	mux, logs := rawServer(t, fakeRegister{fn: func(ctx context.Context, _ usecase.RegisterUserCommand) (domain.User, error) {
		return domain.User{}, fmt.Errorf("register user: %w", ctx.Err())
	}}, fakeGet{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // клиент закрыл соединение
	r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/users", strings.NewReader(`{"username":"alice","display_name":"Алиса"}`))
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
			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/users", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			if b := decodeError(t, w); b.Code != tt.wantCode {
				t.Errorf("code %q, want %q", b.Code, tt.wantCode)
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
