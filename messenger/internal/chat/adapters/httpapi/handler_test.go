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
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"messenger/internal/chat/adapters/httpapi"
	"messenger/internal/chat/adapters/httpapi/gen"
	"messenger/internal/chat/domain"
	"messenger/internal/chat/usecase"
	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/idempotency"
	idemmemory "messenger/internal/platform/idempotency/memory"
	"messenger/internal/platform/openapitest"
	"messenger/internal/platform/principal"
	"messenger/internal/platform/problem"
)

const (
	aliceID = "00000000-0000-7000-b000-000000000001"
	bobID   = "00000000-0000-7000-b000-000000000002"
	carolID = "00000000-0000-7000-b000-000000000003"
	chatID  = "00000000-0000-7000-c000-000000000001"
	msgID   = "00000000-0000-7000-d000-000000000001"
	cmsgID  = "00000000-0000-7000-e000-000000000001" // client_message_id: его придумал клиент
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
	fn func(context.Context, usecase.SendMessageCommand) (usecase.SendMessageResult, error)
}

func (f fakeSend) Execute(ctx context.Context, cmd usecase.SendMessageCommand) (usecase.SendMessageResult, error) {
	return f.fn(ctx, cmd)
}

type fakeList struct {
	fn func(context.Context, usecase.ListMessagesQuery) (usecase.MessagePage, error)
}

func (f fakeList) Execute(ctx context.Context, q usecase.ListMessagesQuery) (usecase.MessagePage, error) {
	return f.fn(ctx, q)
}

type fakeListChats struct {
	fn func(context.Context, usecase.ListChatsQuery) (usecase.ChatPage, error)
}

func (f fakeListChats) Execute(ctx context.Context, q usecase.ListChatsQuery) (usecase.ChatPage, error) {
	return f.fn(ctx, q)
}

type fakeGet struct {
	fn func(context.Context, usecase.GetChatQuery) (*domain.Chat, error)
}

func (f fakeGet) Execute(ctx context.Context, q usecase.GetChatQuery) (*domain.Chat, error) {
	return f.fn(ctx, q)
}

type fakes struct {
	create fakeCreate
	send   fakeSend
	list   fakeList
	chats  fakeListChats
	get    fakeGet
}

// #region contract-server
// server собирает настоящий ServeMux и оборачивает его проверкой по контракту: каждый
// ответ сверяется с api/openapi.yaml. Не заданный тестом use case валит тест при вызове.
func server(t *testing.T, f fakes) (http.Handler, *bytes.Buffer) {
	t.Helper()
	mux, logs := rawServer(t, f)
	return openapitest.New(t).Wrap(mux), logs
}

// #endregion contract-server

// rawServer — без проверки по контракту: только для случая, когда ответа нет вовсе
// (клиент ушёл), — проверять по схеме нечего.
func rawServer(t *testing.T, f fakes) (*http.ServeMux, *bytes.Buffer) {
	t.Helper()
	return rawServerWith(t, f)
}

// rawServerWith — то же с middleware операций, как их подключает composition root.
func rawServerWith(t *testing.T, f fakes, mws ...gen.MiddlewareFunc) (*http.ServeMux, *bytes.Buffer) {
	t.Helper()
	if f.create.fn == nil {
		f.create.fn = func(context.Context, usecase.CreateChatCommand) (usecase.CreateChatResult, error) {
			t.Fatal("CreateChat не должен вызываться")
			return usecase.CreateChatResult{}, nil
		}
	}
	if f.send.fn == nil {
		f.send.fn = func(context.Context, usecase.SendMessageCommand) (usecase.SendMessageResult, error) {
			t.Fatal("SendMessage не должен вызываться")
			return usecase.SendMessageResult{}, nil
		}
	}
	if f.list.fn == nil {
		f.list.fn = func(context.Context, usecase.ListMessagesQuery) (usecase.MessagePage, error) {
			t.Fatal("ListMessages не должен вызываться")
			return usecase.MessagePage{}, nil
		}
	}
	if f.chats.fn == nil {
		f.chats.fn = func(context.Context, usecase.ListChatsQuery) (usecase.ChatPage, error) {
			t.Fatal("ListChats не должен вызываться")
			return usecase.ChatPage{}, nil
		}
	}
	if f.get.fn == nil {
		f.get.fn = func(context.Context, usecase.GetChatQuery) (*domain.Chat, error) {
			t.Fatal("GetChat не должен вызываться")
			return nil, nil
		}
	}
	var logs bytes.Buffer
	h, err := httpapi.New(useCases(f), slog.New(slog.NewJSONHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	h.Register(mux, mws...)
	return mux, &logs
}

func useCases(f fakes) httpapi.UseCases {
	return httpapi.UseCases{CreateChat: f.create, SendMessage: f.send, ListMessages: f.list, ListChats: f.chats, GetChat: f.get}
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
	clientID := must[domain.ClientMessageID](t)(domain.ParseClientMessageID(cmsgID))
	return must[domain.Message](t)(directChat(t).Post(id, clientID, author, text, t0))
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
			wantBody:   `{"id":"` + chatID + `","kind":"direct","member_ids":["` + aliceID + `","` + bobID + `"],"created_at":"2026-10-13T12:00:00Z","last_activity_at":"2026-10-13T12:00:00Z"}`,
		},
		{
			name: "личный уже был", chat: directChat, created: false,
			body:       `{"kind":"direct","member_ids":["` + bobID + `"]}`,
			wantStatus: http.StatusOK,
			wantBody:   `{"id":"` + chatID + `","kind":"direct","member_ids":["` + aliceID + `","` + bobID + `"],"created_at":"2026-10-13T12:00:00Z","last_activity_at":"2026-10-13T12:00:00Z"}`,
		},
		{
			name: "группа с названием", chat: groupChat, created: true,
			body:       `{"kind":"group","title":"Команда","member_ids":["` + bobID + `","` + carolID + `"]}`,
			wantStatus: http.StatusCreated,
			wantBody:   `{"id":"` + chatID + `","kind":"group","title":"Команда","member_ids":["` + aliceID + `","` + bobID + `","` + carolID + `"],"created_at":"2026-10-13T12:00:00Z","last_activity_at":"2026-10-13T12:00:00Z"}`,
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
			assertJSON(t, w.Body, tt.wantBody)
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

// messageJSON — ответ с сообщением из message(t).
const messageJSON = `{"id":"` + msgID + `","client_message_id":"` + cmsgID + `","chat_id":"` + chatID + `","author_id":"` + aliceID + `","text":"Привет, Боб!","seq":1,"sent_at":"2026-10-13T12:00:00Z"}`

// Первая попытка — 201, повтор с тем же client_message_id — 200 и то же сообщение.
func TestSendMessageStatus(t *testing.T) {
	tests := []struct {
		name       string
		created    bool
		wantStatus int
	}{
		{name: "новое сообщение", created: true, wantStatus: http.StatusCreated},
		{name: "повтор client_message_id", created: false, wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got usecase.SendMessageCommand
			mux, _ := server(t, fakes{send: fakeSend{fn: func(_ context.Context, cmd usecase.SendMessageCommand) (usecase.SendMessageResult, error) {
				got = cmd
				return usecase.SendMessageResult{Message: message(t), Created: tt.created}, nil
			}}})

			w := do(t, mux, request{method: http.MethodPost, target: "/v1/chats/" + chatID + "/messages", body: `{"client_message_id":"` + cmsgID + `","text":"Привет, Боб!"}`, userID: aliceID})

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			// У сообщения нет своего адреса с GET — значит, и Location ему не на что указывать.
			if loc := w.Header().Get("Location"); loc != "" {
				t.Errorf("Location %q у сообщения, want пусто", loc)
			}
			want := usecase.SendMessageCommand{ChatID: chatID, AuthorID: aliceID, ClientMessageID: cmsgID, Text: "Привет, Боб!"}
			if got != want {
				t.Errorf("команда %+v, want %+v", got, want)
			}
			assertJSON(t, w.Body, messageJSON)
		})
	}
}

func TestListMessages(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		page      func(*testing.T) usecase.MessagePage
		wantQuery usecase.ListMessagesQuery
		wantBody  string
	}{
		{
			name: "пустая история — [] и next_cursor null", query: "",
			page:      func(*testing.T) usecase.MessagePage { return usecase.MessagePage{} },
			wantQuery: usecase.ListMessagesQuery{ChatID: chatID, ReaderID: bobID},
			wantBody:  `{"items":[],"next_cursor":null}`,
		},
		{
			name: "последняя страница — next_cursor null", query: "?limit=10",
			page: func(t *testing.T) usecase.MessagePage {
				return usecase.MessagePage{Messages: []domain.Message{message(t)}}
			},
			wantQuery: usecase.ListMessagesQuery{ChatID: chatID, ReaderID: bobID, Limit: 10},
			wantBody:  `{"items":[` + messageJSON + `],"next_cursor":null}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got usecase.ListMessagesQuery
			mux, _ := server(t, fakes{list: fakeList{fn: func(_ context.Context, q usecase.ListMessagesQuery) (usecase.MessagePage, error) {
				got = q
				return tt.page(t), nil
			}}})

			w := do(t, mux, request{method: http.MethodGet, target: "/v1/chats/" + chatID + "/messages" + tt.query, userID: bobID})

			if w.Code != http.StatusOK {
				t.Fatalf("status %d, want 200; body %s", w.Code, w.Body)
			}
			if got != tt.wantQuery {
				t.Errorf("запрос %+v, want %+v", got, tt.wantQuery)
			}
			assertJSON(t, w.Body, tt.wantBody)
		})
	}
}

// nextCursor достаёт next_cursor из ответа-страницы.
func nextCursor(t *testing.T, w *httptest.ResponseRecorder) *string {
	t.Helper()
	var page struct {
		NextCursor *string `json:"next_cursor"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("тело %q: %v", w.Body, err)
	}
	return page.NextCursor
}

// Курсор с первой страницы открывает вторую: адаптер заворачивает ключ use case'а
// в курсор и разворачивает обратно — клиент только переносит строку.
func TestListMessagesCursorOpensNextPage(t *testing.T) {
	var queries []usecase.ListMessagesQuery
	mux, _ := server(t, fakes{list: fakeList{fn: func(_ context.Context, q usecase.ListMessagesQuery) (usecase.MessagePage, error) {
		queries = append(queries, q)
		if q.BeforeSeq == 0 {
			return usecase.MessagePage{Messages: []domain.Message{message(t)}, NextBeforeSeq: 41}, nil
		}
		return usecase.MessagePage{}, nil
	}}})
	target := "/v1/chats/" + chatID + "/messages?limit=1"

	first := do(t, mux, request{method: http.MethodGet, target: target, userID: bobID})
	cur := nextCursor(t, first)
	if first.Code != http.StatusOK || cur == nil {
		t.Fatalf("первая страница: status %d, next_cursor %v", first.Code, cur)
	}
	second := do(t, mux, request{method: http.MethodGet, target: target + "&cursor=" + url.QueryEscape(*cur), userID: bobID})

	if second.Code != http.StatusOK {
		t.Fatalf("вторая страница: status %d; body %s", second.Code, second.Body)
	}
	if len(queries) != 2 || queries[1].BeforeSeq != 41 || queries[1].Limit != 1 {
		t.Fatalf("запросы к use case %+v, ждали BeforeSeq 41 во втором", queries)
	}
	if nextCursor(t, second) != nil {
		t.Error("на последней странице next_cursor должен быть null")
	}
}

func TestListMessagesBadCursor(t *testing.T) {
	const otherChat = "00000000-0000-7000-c000-000000000002"
	// Настоящий курсор чата otherChat: снимаем его с ответа сервера, а не собираем руками.
	foreign := func(t *testing.T) string {
		t.Helper()
		mux, _ := server(t, fakes{list: fakeList{fn: func(context.Context, usecase.ListMessagesQuery) (usecase.MessagePage, error) {
			return usecase.MessagePage{Messages: []domain.Message{message(t)}, NextBeforeSeq: 5}, nil
		}}})
		w := do(t, mux, request{method: http.MethodGet, target: "/v1/chats/" + otherChat + "/messages", userID: bobID})
		cur := nextCursor(t, w)
		if cur == nil {
			t.Fatal("нет next_cursor")
		}
		return *cur
	}
	tests := []struct {
		name   string
		cursor func(*testing.T) string
	}{
		{name: "курсор другого чата", cursor: foreign},
		{name: "мусор", cursor: func(*testing.T) string { return "abc" }},
		{name: "номер страницы", cursor: func(*testing.T) string { return "2" }},
		{name: "длиннее 512", cursor: func(*testing.T) string { return strings.Repeat("A", 513) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, logs := server(t, fakes{}) // до use case'а дело не доходит
			cur := tt.cursor(t)

			w := do(t, mux, request{method: http.MethodGet, target: "/v1/chats/" + chatID + "/messages?cursor=" + url.QueryEscape(cur), userID: bobID})

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400; body %s", w.Code, w.Body)
			}
			if b := decodeError(t, w); b.Code != "invalid_input" || b.field() != "cursor" {
				t.Errorf("code %q field %q, want invalid_input cursor", b.Code, b.field())
			}
			if !strings.Contains(logs.String(), "негодный курсор") {
				t.Errorf("причина отказа не попала в журнал: %s", logs)
			}
		})
	}
}

func TestListsBadQuery(t *testing.T) {
	tests := []struct {
		target    string
		wantField string
	}{
		{target: "/v1/chats/" + chatID + "/messages?limit=abc", wantField: "limit"},
		{target: "/v1/chats/" + chatID + "/messages?limit=1.5", wantField: "limit"},
		// Явный limit вне 1…100 — ошибка клиента, даже 0: «не передан» и «передан 0» —
		// разные запросы, и молча подставить значение по умолчанию значит скрыть ошибку.
		{target: "/v1/chats/" + chatID + "/messages?limit=0", wantField: "limit"},
		{target: "/v1/chats/" + chatID + "/messages?limit=-1", wantField: "limit"},
		{target: "/v1/chats/" + chatID + "/messages?limit=101", wantField: "limit"},
		{target: "/v1/chats?limit=0", wantField: "limit"},
		{target: "/v1/chats?limit=-5", wantField: "limit"},
		{target: "/v1/chats?limit=101", wantField: "limit"},
		// Параметр пути разбирает сгенерированная обёртка, и поле в ошибке — как в контракте.
		{target: "/v1/chats/not-a-uuid/messages", wantField: "id"},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			mux, _ := server(t, fakes{})

			w := do(t, mux, request{method: http.MethodGet, target: tt.target, userID: bobID})

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want 400; body %s", w.Code, w.Body)
			}
			if b := decodeError(t, w); b.Code != "invalid_input" || b.field() != tt.wantField {
				t.Errorf("code %q field %q, want invalid_input %q", b.Code, b.field(), tt.wantField)
			}
		})
	}
}

// chatAt — личный чат Алисы с собеседником n, созданный в момент at.
func chatAt(t *testing.T, n int, at time.Time) *domain.Chat {
	t.Helper()
	id := must[domain.ChatID](t)(domain.ParseChatID(fmt.Sprintf("00000000-0000-7000-c000-%012d", n)))
	a := must[domain.UserID](t)(domain.ParseUserID(aliceID))
	b := must[domain.UserID](t)(domain.ParseUserID(fmt.Sprintf("00000000-0000-7000-e000-%012d", n)))
	return must[*domain.Chat](t)(domain.NewDirectChat(id, a, b, at))
}

func TestListChats(t *testing.T) {
	newer, older := chatAt(t, 1, t0.Add(time.Minute)), chatAt(t, 2, t0)
	var queries []usecase.ListChatsQuery
	mux, _ := server(t, fakes{chats: fakeListChats{fn: func(_ context.Context, q usecase.ListChatsQuery) (usecase.ChatPage, error) {
		queries = append(queries, q)
		if q.After == nil {
			return usecase.ChatPage{Chats: []*domain.Chat{newer}, Next: &usecase.ChatPosition{LastActivityAt: newer.LastActivityAt(), ID: newer.ID()}}, nil
		}
		return usecase.ChatPage{Chats: []*domain.Chat{older}}, nil
	}}})

	first := do(t, mux, request{method: http.MethodGet, target: "/v1/chats?limit=1", userID: aliceID})
	if first.Code != http.StatusOK {
		t.Fatalf("status %d, want 200; body %s", first.Code, first.Body)
	}
	assertJSON(t, first.Body, `{"items":[{"id":"`+newer.ID().String()+`","kind":"direct","member_ids":["`+aliceID+`","00000000-0000-7000-e000-000000000001"],"created_at":"2026-10-13T12:01:00Z","last_activity_at":"2026-10-13T12:01:00Z"}],"next_cursor":`+jsonString(t, nextCursor(t, first))+`}`)
	cur := nextCursor(t, first)
	if cur == nil {
		t.Fatal("нет next_cursor на первой странице")
	}

	second := do(t, mux, request{method: http.MethodGet, target: "/v1/chats?limit=1&cursor=" + url.QueryEscape(*cur), userID: aliceID})

	if second.Code != http.StatusOK || nextCursor(t, second) != nil {
		t.Fatalf("вторая страница: status %d, body %s", second.Code, second.Body)
	}
	want := usecase.ListChatsQuery{MemberID: aliceID, Limit: 1, After: &usecase.ChatPosition{LastActivityAt: newer.LastActivityAt(), ID: newer.ID()}}
	if len(queries) != 2 || queries[0].MemberID != aliceID || queries[0].After != nil ||
		queries[1].MemberID != want.MemberID || queries[1].Limit != 1 || queries[1].After == nil ||
		!queries[1].After.LastActivityAt.Equal(want.After.LastActivityAt) || queries[1].After.ID != want.After.ID {
		t.Fatalf("запросы к use case %+v, ждали второй %+v", queries, want)
	}
}

func jsonString(t *testing.T, s *string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Курсор истории не годится для списка чатов: вид курсора зашит в него самого.
func TestListChatsRejectsMessagesCursor(t *testing.T) {
	msgs, _ := server(t, fakes{list: fakeList{fn: func(context.Context, usecase.ListMessagesQuery) (usecase.MessagePage, error) {
		return usecase.MessagePage{Messages: []domain.Message{message(t)}, NextBeforeSeq: 5}, nil
	}}})
	cur := nextCursor(t, do(t, msgs, request{method: http.MethodGet, target: "/v1/chats/" + chatID + "/messages", userID: aliceID}))
	if cur == nil {
		t.Fatal("нет next_cursor")
	}
	mux, _ := server(t, fakes{})

	w := do(t, mux, request{method: http.MethodGet, target: "/v1/chats?cursor=" + url.QueryEscape(*cur), userID: aliceID})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400; body %s", w.Code, w.Body)
	}
	if b := decodeError(t, w); b.field() != "cursor" {
		t.Errorf("field %q, want cursor", b.field())
	}
}

func TestGetChat(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "участник", wantStatus: http.StatusOK},
		{name: "не участник", err: domain.ErrNotMember, wantStatus: http.StatusForbidden, wantCode: "not_member"},
		{name: "нет чата", err: fmt.Errorf("get chat: %w", domain.ErrChatNotFound), wantStatus: http.StatusNotFound, wantCode: "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got usecase.GetChatQuery
			mux, _ := server(t, fakes{get: fakeGet{fn: func(_ context.Context, q usecase.GetChatQuery) (*domain.Chat, error) {
				got = q
				if tt.err != nil {
					return nil, tt.err
				}
				return directChat(t), nil
			}}})

			w := do(t, mux, request{method: http.MethodGet, target: "/v1/chats/" + chatID, userID: bobID})

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			if want := (usecase.GetChatQuery{ChatID: chatID, ReaderID: bobID}); got != want {
				t.Errorf("запрос %+v, want %+v", got, want)
			}
			if tt.wantCode != "" {
				if b := decodeError(t, w); b.Code != tt.wantCode {
					t.Errorf("code %q, want %q", b.Code, tt.wantCode)
				}
				return
			}
			assertJSON(t, w.Body, `{"id":"`+chatID+`","kind":"direct","member_ids":["`+aliceID+`","`+bobID+`"],"created_at":"2026-10-13T12:00:00Z","last_activity_at":"2026-10-13T12:00:00Z"}`)
		})
	}
}

func TestGetChatBadID(t *testing.T) {
	mux, _ := server(t, fakes{})

	w := do(t, mux, request{method: http.MethodGet, target: "/v1/chats/not-a-uuid", userID: bobID})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400; body %s", w.Code, w.Body)
	}
	if b := decodeError(t, w); b.field() != "id" {
		t.Errorf("field %q, want id", b.field())
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
		{name: "client_message_id с другим текстом", err: domain.ErrClientMessageIDReused, wantStatus: 409, wantCode: "client_message_id_reused"},
		{name: "дедлайн", err: fmt.Errorf("send message: %w", context.DeadlineExceeded), wantStatus: 503, wantCode: "timeout"},
		{name: "сбой генератора id — не ошибка клиента", err: errors.New(`send message: генератор выдал не UUID: "x"`), wantStatus: 500, wantCode: "internal"},
		{name: "прочее", err: errors.New("секрет БД"), wantStatus: 500, wantCode: "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, _ := server(t, fakes{send: fakeSend{fn: func(context.Context, usecase.SendMessageCommand) (usecase.SendMessageResult, error) {
				return usecase.SendMessageResult{}, tt.err
			}}})

			w := do(t, mux, request{method: http.MethodPost, target: "/v1/chats/" + chatID + "/messages", body: `{"text":"a"}`, userID: aliceID})

			if w.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d; body %s", w.Code, tt.wantStatus, w.Body)
			}
			b := decodeError(t, w)
			if b.Code != tt.wantCode || b.field() != tt.wantField {
				t.Errorf("code %q field %q, want %q %q", b.Code, b.field(), tt.wantCode, tt.wantField)
			}
		})
	}
}

func TestInternalErrorHidesDetails(t *testing.T) {
	mux, logs := server(t, fakes{list: fakeList{fn: func(context.Context, usecase.ListMessagesQuery) (usecase.MessagePage, error) {
		return usecase.MessagePage{}, errors.New("секрет БД: FATAL: password authentication failed for user \"messenger\" (SQLSTATE 28P01)")
	}}})

	w := do(t, mux, request{method: http.MethodGet, target: "/v1/chats/" + chatID + "/messages", userID: bobID})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "секрет") || strings.Contains(w.Body.String(), "SQLSTATE") {
		t.Errorf("ответ раскрывает внутреннюю ошибку: %s", w.Body)
	}
	if !strings.Contains(logs.String(), "секрет БД") || !strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Errorf("в логе нет деталей на уровне ERROR: %s", logs)
	}
}

func TestClientGoneWritesNothing(t *testing.T) {
	// Use case, как и настоящий, возвращает ошибку своего ctx — контекста запроса.
	mux, logs := rawServer(t, fakes{send: fakeSend{fn: func(ctx context.Context, _ usecase.SendMessageCommand) (usecase.SendMessageResult, error) {
		return usecase.SendMessageResult{}, fmt.Errorf("send message: %w", ctx.Err())
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

// TestInternalCanceledIsServerError: Canceled пришёл не от клиента (тот ждёт ответа), а
// от внутреннего контекста сервера — это сбой, и молчать нельзя: иначе клиент получит
// пустой 200.
func TestInternalCanceledIsServerError(t *testing.T) {
	mux, logs := server(t, fakes{send: fakeSend{fn: func(context.Context, usecase.SendMessageCommand) (usecase.SendMessageResult, error) {
		inner, cancel := context.WithCancel(context.Background())
		cancel()
		return usecase.SendMessageResult{}, fmt.Errorf("send message: %w", inner.Err())
	}}})

	w := do(t, mux, request{method: http.MethodPost, target: "/v1/chats/" + chatID + "/messages", body: `{"text":"a"}`, userID: aliceID})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500; body %s", w.Code, w.Body)
	}
	if b := decodeError(t, w); b.Code != "internal" {
		t.Errorf("code %q, want internal", b.Code)
	}
	if !strings.Contains(logs.String(), `"level":"ERROR"`) {
		t.Errorf("сбой сервера должен логироваться на Error: %s", logs)
	}
}

func TestUnauthenticated(t *testing.T) {
	routes := []request{
		{method: http.MethodPost, target: "/v1/chats", body: `{"kind":"direct","member_ids":["` + bobID + `"]}`},
		{method: http.MethodPost, target: "/v1/chats/" + chatID + "/messages", body: `{"text":"a"}`},
		{method: http.MethodGet, target: "/v1/chats/" + chatID + "/messages"},
		{method: http.MethodGet, target: "/v1/chats"},
		{method: http.MethodGet, target: "/v1/chats/" + chatID},
	}
	// Личность в контексте, какой её положил бы middleware. Он сам отсекает пустой
	// заголовок, но адаптер не должен на это полагаться: формат id проверяет он.
	identities := map[string]string{
		"нет личности": "",
		"не UUID":      "alice",
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
				if b := decodeError(t, w); b.Code != "unauthenticated" {
					t.Errorf("code %q, want unauthenticated", b.Code)
				}
				if got := w.Header().Get("WWW-Authenticate"); got != `Bearer realm="messenger"` {
					t.Errorf("WWW-Authenticate %q", got)
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
			if b := decodeError(t, w); b.Code != tt.wantCode {
				t.Errorf("code %q, want %q", b.Code, tt.wantCode)
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
	if b := decodeError(t, w); b.Code != "unsupported_media_type" {
		t.Errorf("code %q, want unsupported_media_type", b.Code)
	}
}

func TestNewNilDependencies(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	full := useCases(fakes{})
	cases := map[string]func(*httpapi.UseCases){
		"create": func(u *httpapi.UseCases) { u.CreateChat = nil },
		"send":   func(u *httpapi.UseCases) { u.SendMessage = nil },
		"list":   func(u *httpapi.UseCases) { u.ListMessages = nil },
		"chats":  func(u *httpapi.UseCases) { u.ListChats = nil },
		"get":    func(u *httpapi.UseCases) { u.GetChat = nil },
	}
	for name, drop := range cases {
		uc := full
		drop(&uc)
		if _, err := httpapi.New(uc, log); err == nil {
			t.Errorf("New без %s: want error", name)
		}
	}
	if _, err := httpapi.New(full, nil); err == nil {
		t.Error("New без журнала: want error")
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
			for _, leak := range []string{"Go", "struct", "SendMessageRequest", "gen.", "json:"} {
				if strings.Contains(w.Body.String(), leak) {
					t.Errorf("ответ раскрывает внутреннее (%q): %s", leak, w.Body)
				}
			}
		})
	}
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return t0 }

// Idempotency-Key через настоящий middleware, подключённый так же, как в main: повтор
// POST /v1/chats не создаёт второй чат и получает тот же ответ; другое тело с тем же
// ключом — 422. Все ответы сверяются с контрактом.
func TestCreateChatIdempotencyKey(t *testing.T) {
	var calls int
	mw := idempotency.Middleware(idemmemory.NewStore(), fixedClock{}, slog.New(slog.DiscardHandler), "POST /v1/chats")
	raw, _ := rawServerWith(t, fakes{create: fakeCreate{fn: func(context.Context, usecase.CreateChatCommand) (usecase.CreateChatResult, error) {
		calls++
		return usecase.CreateChatResult{Chat: groupChat(t), Created: true}, nil
	}}}, gen.MiddlewareFunc(mw))
	mux := openapitest.New(t).Wrap(raw)
	post := func(key, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/chats", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set(idempotency.Header, key)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, asUser(r, aliceID))
		return w
	}
	const body = `{"kind":"group","title":"Команда","member_ids":["` + bobID + `","` + carolID + `"]}`

	first := post("key-1", body)
	again := post("key-1", body)

	if first.Code != http.StatusCreated || again.Code != http.StatusCreated || calls != 1 {
		t.Fatalf("ответы %d и %d, вызовов use case %d — ждали 201, 201 и 1", first.Code, again.Code, calls)
	}
	if again.Body.String() != first.Body.String() || again.Header().Get("Location") != first.Header().Get("Location") {
		t.Fatalf("повтор отличается от первого ответа:\n%s\n%s", first.Body, again.Body)
	}
	if again.Header().Get(idempotency.ReplayedHeader) != "true" || first.Header().Get(idempotency.ReplayedHeader) != "" {
		t.Fatalf("Idempotent-Replayed: первый %q, повтор %q", first.Header().Get(idempotency.ReplayedHeader), again.Header().Get(idempotency.ReplayedHeader))
	}
	other := post("key-1", strings.Replace(body, "Команда", "Другая", 1))
	if other.Code != http.StatusUnprocessableEntity || calls != 1 {
		t.Fatalf("тот же ключ, другое тело: %d, вызовов %d — ждали 422 и 1", other.Code, calls)
	}
	if b := decodeError(t, other); b.Code != "idempotency_key_reused" {
		t.Errorf("code %q, want idempotency_key_reused", b.Code)
	}
	bad := post(strings.Repeat("k", idempotency.KeyMaxLen+1), body)
	if b := decodeError(t, bad); bad.Code != http.StatusBadRequest || b.field() != idempotency.Header {
		t.Errorf("длинный ключ: %d, field %q — want 400 Idempotency-Key", bad.Code, b.field())
	}
}
