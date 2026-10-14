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

	"messenger/internal/chat/adapters/httpapi"
	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/principal"
)

const (
	aliceID = "00000000-0000-7000-b000-000000000001"
	bobID   = "00000000-0000-7000-b000-000000000002"
	carolID = "00000000-0000-7000-b000-000000000003"
	chatID  = "00000000-0000-7000-c000-000000000001"
	msgID   = "00000000-0000-7000-d000-000000000001"
)

// Время не в UTC: маппер обязан привести его к UTC сам.
var t0 = time.Date(2026, 10, 13, 15, 0, 0, 0, time.FixedZone("MSK", 3*60*60))

// Фейки use case'ов — структуры с полями-функциями.
type fakeCreate struct {
	fn func(context.Context, usecase.CreateChatCommand) (usecase.CreateChatResult, error)
}

func (f fakeCreate) Execute(ctx context.Context, cmd usecase.CreateChatCommand) (usecase.CreateChatResult, error) {
	return f.fn(ctx, cmd)
}

type fakeSend struct {
	fn func(context.Context, usecase.SendMessageCommand) (domain.Message, error)
}

func (f fakeSend) Execute(ctx context.Context, cmd usecase.SendMessageCommand) (domain.Message, error) {
	return f.fn(ctx, cmd)
}

type fakeList struct {
	fn func(context.Context, usecase.ListMessagesQuery) ([]domain.Message, error)
}

func (f fakeList) Execute(ctx context.Context, q usecase.ListMessagesQuery) ([]domain.Message, error) {
	return f.fn(ctx, q)
}

type fakes struct {
	create fakeCreate
	send   fakeSend
	list   fakeList
}

// server собирает настоящий ServeMux; не заданный тестом use case валит тест при вызове.
func server(t *testing.T, f fakes) (*http.ServeMux, *bytes.Buffer) {
	t.Helper()
	if f.create.fn == nil {
		f.create.fn = func(context.Context, usecase.CreateChatCommand) (usecase.CreateChatResult, error) {
			t.Fatal("CreateChat не должен вызываться")
			return usecase.CreateChatResult{}, nil
		}
	}
	if f.send.fn == nil {
		f.send.fn = func(context.Context, usecase.SendMessageCommand) (domain.Message, error) {
			t.Fatal("SendMessage не должен вызываться")
			return domain.Message{}, nil
		}
	}
	if f.list.fn == nil {
		f.list.fn = func(context.Context, usecase.ListMessagesQuery) ([]domain.Message, error) {
			t.Fatal("ListMessages не должен вызываться")
			return nil, nil
		}
	}
	var logs bytes.Buffer
	h, err := httpapi.New(f.create, f.send, f.list, slog.New(slog.NewJSONHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	return mux, &logs
}

func must[T any](t *testing.T) func(T, error) T {
	return func(v T, err error) T {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}

func directChat(t *testing.T) *domain.Chat {
	t.Helper()
	id := must[domain.ChatID](t)(domain.ParseChatID(chatID))
	a := must[domain.UserID](t)(domain.ParseUserID(aliceID))
	b := must[domain.UserID](t)(domain.ParseUserID(bobID))
	return must[*domain.Chat](t)(domain.NewDirectChat(id, a, b, t0))
}

func groupChat(t *testing.T) *domain.Chat {
	t.Helper()
	id := must[domain.ChatID](t)(domain.ParseChatID(chatID))
	a := must[domain.UserID](t)(domain.ParseUserID(aliceID))
	b := must[domain.UserID](t)(domain.ParseUserID(bobID))
	c := must[domain.UserID](t)(domain.ParseUserID(carolID))
	title := must[domain.Title](t)(domain.NewTitle("Команда"))
	return must[*domain.Chat](t)(domain.NewGroupChat(id, title, a, []domain.UserID{b, c}, t0))
}

func message(t *testing.T) domain.Message {
	t.Helper()
	id := must[domain.MessageID](t)(domain.ParseMessageID(msgID))
	author := must[domain.UserID](t)(domain.ParseUserID(aliceID))
	text := must[domain.Text](t)(domain.NewText("Привет, Боб!"))
	return must[domain.Message](t)(directChat(t).Post(id, author, text, t0))
}

type request struct {
	method, target, body string
	userID               string // пусто — личности в контексте нет
}

// asUser кладёт личность в контекст так же, как это делает middleware аутентификации:
// адаптер не знает, откуда она взялась, — заголовок, токен или тест.
func asUser(r *http.Request, userID string) *http.Request {
	if userID == "" {
		return r
	}
	return r.WithContext(principal.WithUserID(r.Context(), userID))
}

func do(t *testing.T, mux http.Handler, req request) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(t.Context(), req.method, req.target, strings.NewReader(req.body))
	r.Header.Set("Content-Type", "application/json")
	r = asUser(r, req.userID)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field"`
	} `json:"error"`
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var b errorBody
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("тело ошибки %q: %v", w.Body.String(), err)
	}
	return b
}

func TestCreateChatStatus(t *testing.T) {
	tests := []struct {
		name       string
		chat       func(*testing.T) *domain.Chat
		created    bool
		body       string
		wantStatus int
		wantBody   string
	}{
		{
			name: "личный создан", chat: directChat, created: true,
			body:       `{"kind":"direct","member_ids":["` + bobID + `"]}`,
			wantStatus: http.StatusCreated,
			wantBody:   `{"id":"` + chatID + `","kind":"direct","member_ids":["` + aliceID + `","` + bobID + `"],"created_at":"2026-10-13T12:00:00Z"}`,
		},
		{
			name: "личный уже был", chat: directChat, created: false,
			body:       `{"kind":"direct","member_ids":["` + bobID + `"]}`,
			wantStatus: http.StatusOK,
			wantBody:   `{"id":"` + chatID + `","kind":"direct","member_ids":["` + aliceID + `","` + bobID + `"],"created_at":"2026-10-13T12:00:00Z"}`,
		},
		{
			name: "группа с названием", chat: groupChat, created: true,
			body:       `{"kind":"group","title":"Команда","member_ids":["` + bobID + `","` + carolID + `"]}`,
			wantStatus: http.StatusCreated,
			wantBody:   `{"id":"` + chatID + `","kind":"group","title":"Команда","member_ids":["` + aliceID + `","` + bobID + `","` + carolID + `"],"created_at":"2026-10-13T12:00:00Z"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got usecase.CreateChatCommand
			mux, _ := server(t, fakes{create: fakeCreate{fn: func(_ context.Context, cmd usecase.CreateChatCommand) (usecase.CreateChatResult, error) {
				got = cmd
				return usecase.CreateChatResult{Chat: tt.chat(t), Created: tt.created}, nil
			}}})

			w := do(t, mux, request{method: http.MethodPost, target: "/v1/chats", body: tt.body, userID: aliceID})

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			if w.Body.String() != tt.wantBody+"\n" {
				t.Errorf("body\n%s\nwant\n%s", w.Body, tt.wantBody)
			}
			if got.CreatorID != aliceID {
				t.Errorf("CreatorID %q, want из principal %q", got.CreatorID, aliceID)
			}
			// И при 201, и при 200 (личный чат уже был) — адрес чата.
			if loc := w.Header().Get("Location"); loc != "/v1/chats/"+chatID {
				t.Errorf("Location %q, want /v1/chats/%s", loc, chatID)
			}
		})
	}
}

func TestSendMessageCreated(t *testing.T) {
	var got usecase.SendMessageCommand
	mux, _ := server(t, fakes{send: fakeSend{fn: func(_ context.Context, cmd usecase.SendMessageCommand) (domain.Message, error) {
		got = cmd
		return message(t), nil
	}}})

	w := do(t, mux, request{method: http.MethodPost, target: "/v1/chats/" + chatID + "/messages", body: `{"text":"Привет, Боб!"}`, userID: aliceID})

	if w.Code != http.StatusCreated {
		t.Fatalf("status %d, want 201; body %s", w.Code, w.Body)
	}
	// У сообщения нет своего адреса с GET — значит, и Location ему не на что указывать.
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("Location %q у сообщения, want пусто", loc)
	}
	want := usecase.SendMessageCommand{ChatID: chatID, AuthorID: aliceID, Text: "Привет, Боб!"}
	if got != want {
		t.Errorf("команда %+v, want %+v", got, want)
	}
	wantBody := `{"id":"` + msgID + `","chat_id":"` + chatID + `","author_id":"` + aliceID + `","text":"Привет, Боб!","seq":1,"sent_at":"2026-10-13T12:00:00Z"}` + "\n"
	if w.Body.String() != wantBody {
		t.Errorf("body\n%s\nwant\n%s", w.Body, wantBody)
	}
}

func TestListMessages(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		msgs      func(*testing.T) []domain.Message
		wantQuery usecase.ListMessagesQuery
		wantBody  string
	}{
		{
			name: "пустая история — [], а не null", query: "",
			msgs:      func(*testing.T) []domain.Message { return nil },
			wantQuery: usecase.ListMessagesQuery{ChatID: chatID, ReaderID: bobID},
			wantBody:  `{"messages":[]}`,
		},
		{
			name: "after и limit из строки запроса", query: "?after=7&limit=10",
			msgs:      func(t *testing.T) []domain.Message { return []domain.Message{message(t)} },
			wantQuery: usecase.ListMessagesQuery{ChatID: chatID, ReaderID: bobID, AfterSeq: 7, Limit: 10},
			wantBody:  `{"messages":[{"id":"` + msgID + `","chat_id":"` + chatID + `","author_id":"` + aliceID + `","text":"Привет, Боб!","seq":1,"sent_at":"2026-10-13T12:00:00Z"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got usecase.ListMessagesQuery
			mux, _ := server(t, fakes{list: fakeList{fn: func(_ context.Context, q usecase.ListMessagesQuery) ([]domain.Message, error) {
				got = q
				return tt.msgs(t), nil
			}}})

			w := do(t, mux, request{method: http.MethodGet, target: "/v1/chats/" + chatID + "/messages" + tt.query, userID: bobID})

			if w.Code != http.StatusOK {
				t.Fatalf("status %d, want 200; body %s", w.Code, w.Body)
			}
			if got != tt.wantQuery {
				t.Errorf("запрос %+v, want %+v", got, tt.wantQuery)
			}
			if w.Body.String() != tt.wantBody+"\n" {
				t.Errorf("body\n%s\nwant\n%s", w.Body, tt.wantBody)
			}
		})
	}
}

func TestListMessagesBadQuery(t *testing.T) {
	tests := []struct {
		query     string
		wantField string
	}{
		{query: "?limit=abc", wantField: "limit"},
		{query: "?after=1.5", wantField: "after"},
		{query: "?after=99999999999999999999", wantField: "after"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			mux, _ := server(t, fakes{})

			w := do(t, mux, request{method: http.MethodGet, target: "/v1/chats/" + chatID + "/messages" + tt.query, userID: bobID})

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400; body %s", w.Code, w.Body)
			}
			if b := decodeError(t, w); b.Error.Code != "invalid_input" || b.Error.Field != tt.wantField {
				t.Errorf("code %q field %q, want invalid_input %q", b.Error.Code, b.Error.Field, tt.wantField)
			}
		})
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
		{name: "валидация", err: &domain.ValidationError{Field: "text", Reason: "пустое сообщение"}, wantStatus: 400, wantCode: "invalid_input", wantField: "text"},
		{name: "неизвестный создатель", err: &domain.ValidationError{Field: "creator_id", Reason: "нет такого пользователя"}, wantStatus: 400, wantCode: "invalid_input", wantField: "creator_id"},
		{name: "чат не найден", err: fmt.Errorf("send message: %w", domain.ErrChatNotFound), wantStatus: 404, wantCode: "not_found"},
		{name: "не участник", err: domain.ErrNotMember, wantStatus: 403, wantCode: "not_member"},
		{name: "конфликт версий", err: fmt.Errorf("send message (попытка 3): %w", domain.ErrConcurrentUpdate), wantStatus: 409, wantCode: "conflict"},
		{name: "дедлайн", err: fmt.Errorf("send message: %w", context.DeadlineExceeded), wantStatus: 503, wantCode: "timeout"},
		{name: "сбой генератора id — не ошибка клиента", err: errors.New(`send message: генератор выдал не UUID: "x"`), wantStatus: 500, wantCode: "internal"},
		{name: "прочее", err: errors.New("секрет БД"), wantStatus: 500, wantCode: "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, _ := server(t, fakes{send: fakeSend{fn: func(context.Context, usecase.SendMessageCommand) (domain.Message, error) {
				return domain.Message{}, tt.err
			}}})

			w := do(t, mux, request{method: http.MethodPost, target: "/v1/chats/" + chatID + "/messages", body: `{"text":"a"}`, userID: aliceID})

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			b := decodeError(t, w)
			if b.Error.Code != tt.wantCode || b.Error.Field != tt.wantField {
				t.Errorf("code %q field %q, want %q %q", b.Error.Code, b.Error.Field, tt.wantCode, tt.wantField)
			}
		})
	}
}

func TestInternalErrorHidesDetails(t *testing.T) {
	mux, logs := server(t, fakes{list: fakeList{fn: func(context.Context, usecase.ListMessagesQuery) ([]domain.Message, error) {
		return nil, errors.New("секрет БД: pq: password authentication failed for user messenger")
	}}})

	w := do(t, mux, request{method: http.MethodGet, target: "/v1/chats/" + chatID + "/messages", userID: bobID})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "секрет") || strings.Contains(w.Body.String(), "pq") {
		t.Errorf("ответ раскрывает внутреннюю ошибку: %s", w.Body)
	}
	if !strings.Contains(logs.String(), "секрет БД") || !strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Errorf("в логе нет деталей на уровне ERROR: %s", logs)
	}
}

func TestClientGoneWritesNothing(t *testing.T) {
	// Use case, как и настоящий, возвращает ошибку своего ctx — контекста запроса.
	mux, logs := server(t, fakes{send: fakeSend{fn: func(ctx context.Context, _ usecase.SendMessageCommand) (domain.Message, error) {
		return domain.Message{}, fmt.Errorf("send message: %w", ctx.Err())
	}}})
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // клиент закрыл соединение
	r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/chats/"+chatID+"/messages", strings.NewReader(`{"text":"a"}`))
	r.Header.Set("Content-Type", "application/json")
	r = asUser(r, aliceID)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, r)

	if w.Body.Len() != 0 || w.Header().Get("Content-Type") != "" {
		t.Errorf("ответ записан, хотя клиент ушёл: %d %s", w.Code, w.Body)
	}
	if !strings.Contains(logs.String(), `"level":"INFO"`) || strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Errorf("уход клиента должен логироваться на Info, не Error: %s", logs)
	}
}

func TestUnauthenticated(t *testing.T) {
	routes := []request{
		{method: http.MethodPost, target: "/v1/chats", body: `{"kind":"direct","member_ids":["` + bobID + `"]}`},
		{method: http.MethodPost, target: "/v1/chats/" + chatID + "/messages", body: `{"text":"a"}`},
		{method: http.MethodGet, target: "/v1/chats/" + chatID + "/messages"},
	}
	// Личность в контексте, какой её положил бы middleware. Он сам отсекает пустой
	// заголовок, но адаптер не должен на это полагаться: формат id проверяет он.
	identities := map[string]string{
		"нет личности":            "",
		"не UUID":                 "alice",
		"UUID в верхнем регистре": strings.ToUpper(aliceID),
		"одни пробелы":            "   ",
	}
	for _, route := range routes {
		for name, identity := range identities {
			t.Run(route.method+" "+route.target+"/"+name, func(t *testing.T) {
				mux, _ := server(t, fakes{}) // use case'ы не должны вызываться вовсе
				route.userID = identity

				w := do(t, mux, route)

				if w.Code != http.StatusUnauthorized {
					t.Fatalf("status %d, want 401; body %s", w.Code, w.Body)
				}
				if b := decodeError(t, w); b.Error.Code != "unauthenticated" {
					t.Errorf("code %q, want unauthenticated", b.Error.Code)
				}
			})
		}
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{name: "два объекта", body: `{"text":"a"}{"text":"b"}`, wantStatus: 400, wantCode: "bad_json"},
		{name: "неизвестное поле", body: `{"text":"a","author_id":"` + bobID + `"}`, wantStatus: 400, wantCode: "bad_json"},
		{name: "больше 1 МиБ", body: `{"text":"` + strings.Repeat("я", httpjson.MaxBodyBytes/2+1) + `"}`, wantStatus: 413, wantCode: "body_too_large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, _ := server(t, fakes{})

			w := do(t, mux, request{method: http.MethodPost, target: "/v1/chats/" + chatID + "/messages", body: tt.body, userID: aliceID})

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			if b := decodeError(t, w); b.Error.Code != tt.wantCode {
				t.Errorf("code %q, want %q", b.Error.Code, tt.wantCode)
			}
		})
	}
}

func TestUnsupportedMediaType(t *testing.T) {
	mux, _ := server(t, fakes{})
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/chats/"+chatID+"/messages", strings.NewReader(`text=a`))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = asUser(r, aliceID)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, r)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status %d, want 415", w.Code)
	}
	if b := decodeError(t, w); b.Error.Code != "unsupported_media_type" {
		t.Errorf("code %q, want unsupported_media_type", b.Error.Code)
	}
}

func TestNewNilDependencies(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	c, s, l := fakeCreate{}, fakeSend{}, fakeList{}
	cases := map[string]func() (*httpapi.Handler, error){
		"create": func() (*httpapi.Handler, error) { return httpapi.New(nil, s, l, log) },
		"send":   func() (*httpapi.Handler, error) { return httpapi.New(c, nil, l, log) },
		"list":   func() (*httpapi.Handler, error) { return httpapi.New(c, s, nil, log) },
		"log":    func() (*httpapi.Handler, error) { return httpapi.New(c, s, l, nil) },
	}
	for name, newHandler := range cases {
		if _, err := newHandler(); err == nil {
			t.Errorf("New с nil %s: want error", name)
		}
	}
}

// TestDecodeErrorsHideInternals: ошибка разбора говорит на языке JSON — без имени
// нашего DTO и типов Go, которые encoding/json кладёт в свой текст ошибки.
func TestDecodeErrorsHideInternals(t *testing.T) {
	bodies := []string{`{"text":42}`, `{"text":"a","author_id":"x"}`, `{"text":}`, `{"text":"a"`, `[]`}
	for _, body := range bodies {
		t.Run(body, func(t *testing.T) {
			mux, _ := server(t, fakes{})

			w := do(t, mux, request{method: http.MethodPost, target: "/v1/chats/" + chatID + "/messages", body: body, userID: aliceID})

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400; body %s", w.Code, w.Body)
			}
			for _, leak := range []string{"Go", "struct", "sendMessageRequest", "json:"} {
				if strings.Contains(w.Body.String(), leak) {
					t.Errorf("ответ раскрывает внутреннее (%q): %s", leak, w.Body)
				}
			}
		})
	}
}
