package idempotency_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/idempotency"
	"messenger/internal/platform/idempotency/memory"
	"messenger/internal/platform/principal"
	"messenger/internal/platform/problem"
)

const (
	route     = "POST /v1/chats"
	userHdr   = "X-Test-User" // кем представиться в тесте; пусто — alice
	noUser    = "-"           // значение userHdr: личности в контексте нет
	otherPath = "/v1/other"   // маршрут, которого нет в routes
)

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// clock — часы, которые тест двигает сам.
type clock struct{ now atomic.Int64 }

func (c *clock) Now() time.Time { return t0.Add(time.Duration(c.now.Load())) }

func (c *clock) Advance(d time.Duration) { c.now.Add(int64(d)) }

type server struct {
	h     http.Handler
	clock *clock
	logs  *bytes.Buffer
}

// serve собирает настоящий ServeMux: middleware подключается к маршруту, как в бою через
// сгенерированный обработчик, и r.Pattern заполнен. Личность кладётся в контекст так же,
// как это делает аутентификация, — раньше маршрутизатора.
func serve(t *testing.T, store idempotency.Store, h http.Handler) server {
	t.Helper()
	var logs bytes.Buffer
	clk := &clock{}
	mw := idempotency.Middleware(store, clk, slog.New(slog.NewJSONHandler(&logs, nil)), route)
	mux := http.NewServeMux()
	mux.Handle(route, mw(h))
	mux.Handle("POST "+otherPath, mw(h))
	withUser := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch user := r.Header.Get(userHdr); user {
		case noUser:
		case "":
			r = r.WithContext(principal.WithUserID(r.Context(), "alice"))
		default:
			r = r.WithContext(principal.WithUserID(r.Context(), user))
		}
		mux.ServeHTTP(w, r)
	})
	return server{h: withUser, clock: clk, logs: &logs}
}

// reservationTTL — срок резервации хранилищ теста: как в сборке при сроке запроса 10 с.
const reservationTTL = 10*time.Second + idempotency.ReservationGrace

func newMemStore(t *testing.T) *memory.Store {
	t.Helper()
	s, err := memory.NewStore(reservationTTL)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func request(t *testing.T, key, body string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/chats", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set(idempotency.Header, key)
	}
	return r
}

func (s server) do(r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.h.ServeHTTP(rec, r)
	return rec
}

// created — обработчик «создания»: считает вызовы, отвечает 201 с телом, Location и
// лишним заголовком, который повторять не нужно. Тело запроса он читает сам: middleware
// обязан вернуть его на место.
func created(calls *atomic.Int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/v1/chats/1")
		w.Header().Set("X-Call", string(rune('0'+n)))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"1","echo":` + string(body) + `}`))
	})
}

func problemCode(t *testing.T, rec *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	var p struct {
		Code          string `json:"code"`
		InvalidParams []struct {
			Name string `json:"name"`
		} `json:"invalid_params"`
	}
	if ct := rec.Header().Get("Content-Type"); ct != problem.ContentType {
		t.Fatalf("Content-Type %q, ждали %q; тело %s", ct, problem.ContentType, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("тело %q: %v", rec.Body, err)
	}
	if len(p.InvalidParams) == 0 {
		return p.Code, ""
	}
	return p.Code, p.InvalidParams[0].Name
}

func TestIdempotencyReplaysResponse(t *testing.T) {
	var calls atomic.Int32
	s := serve(t, newMemStore(t), created(&calls))

	first := s.do(request(t, "key-1", `{"kind":"group"}`))
	again := s.do(request(t, "key-1", `{"kind":"group"}`))

	if calls.Load() != 1 {
		t.Fatalf("обработчик вызван %d раз, ждали 1", calls.Load())
	}
	if first.Header().Get(idempotency.ReplayedHeader) != "" {
		t.Error("первый ответ помечен как повтор")
	}
	if again.Code != first.Code || !bytes.Equal(again.Body.Bytes(), first.Body.Bytes()) {
		t.Fatalf("повтор: %d %s, ждали %d %s", again.Code, again.Body, first.Code, first.Body)
	}
	if again.Header().Get(idempotency.ReplayedHeader) != "true" || again.Header().Get("Location") != "/v1/chats/1" ||
		again.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("заголовки повтора: %v", again.Header())
	}
	if again.Header().Get("X-Call") != "" {
		t.Error("повторён заголовок, которого нет в списке сохраняемых")
	}
	if !strings.Contains(first.Body.String(), `"echo":{"kind":"group"}`) {
		t.Fatalf("обработчик не получил тело запроса: %s", first.Body)
	}
}

func TestIdempotencyPassThrough(t *testing.T) {
	tests := []struct {
		name   string
		first  func(t *testing.T) *http.Request
		second func(t *testing.T) *http.Request
	}{
		{
			name:   "без ключа — обычная обработка",
			first:  func(t *testing.T) *http.Request { return request(t, "", `{}`) },
			second: func(t *testing.T) *http.Request { return request(t, "", `{}`) },
		},
		{
			name:   "другой ключ — другой запрос",
			first:  func(t *testing.T) *http.Request { return request(t, "key-1", `{}`) },
			second: func(t *testing.T) *http.Request { return request(t, "key-2", `{}`) },
		},
		{
			name:  "тот же ключ у другого пользователя — не повтор",
			first: func(t *testing.T) *http.Request { return request(t, "key-1", `{}`) },
			second: func(t *testing.T) *http.Request {
				r := request(t, "key-1", `{}`)
				r.Header.Set(userHdr, "bob")
				return r
			},
		},
		{
			name: "маршрут не из routes — middleware не вмешивается",
			first: func(t *testing.T) *http.Request {
				r := request(t, "key-1", `{}`)
				r.URL.Path = otherPath
				return r
			},
			second: func(t *testing.T) *http.Request {
				r := request(t, "key-1", `{}`)
				r.URL.Path = otherPath
				return r
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			s := serve(t, newMemStore(t), created(&calls))

			s.do(tt.first(t))
			rec := s.do(tt.second(t))

			if calls.Load() != 2 || rec.Code != http.StatusCreated || rec.Header().Get(idempotency.ReplayedHeader) != "" {
				t.Fatalf("обработчик вызван %d раз, второй ответ %d %v", calls.Load(), rec.Code, rec.Header())
			}
		})
	}
}

func TestIdempotencyKeyReusedWithOtherBody(t *testing.T) {
	var calls atomic.Int32
	s := serve(t, newMemStore(t), created(&calls))

	s.do(request(t, "key-1", `{"title":"A"}`))
	rec := s.do(request(t, "key-1", `{"title":"B"}`))

	if code, _ := problemCode(t, rec); rec.Code != http.StatusUnprocessableEntity || code != string(problem.CodeIdempotencyKeyReused) {
		t.Fatalf("ответ %d %s, ждали 422 idempotency_key_reused", rec.Code, code)
	}
	if calls.Load() != 1 {
		t.Fatalf("обработчик вызван %d раз, ждали 1", calls.Load())
	}
}

// statusSeq — обработчик, который на i-й вызов отвечает статусом codes[i] (0 — ничего
// не пишет), а после конца списка — 201.
func statusSeq(calls *atomic.Int32, codes ...int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := int(calls.Add(1))
		code := http.StatusCreated
		if n <= len(codes) {
			code = codes[n-1]
		}
		if code == 0 {
			return
		}
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"call":` + string(rune('0'+n)) + `}`))
	})
}

func TestIdempotencyReleasesAfter5xx(t *testing.T) {
	tests := []struct {
		name  string
		first int
	}{
		{name: "500", first: http.StatusInternalServerError},
		{name: "503", first: http.StatusServiceUnavailable},
		{name: "ничего не записано — клиент ушёл", first: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			s := serve(t, newMemStore(t), statusSeq(&calls, tt.first))

			s.do(request(t, "key-1", `{}`))
			rec := s.do(request(t, "key-1", `{}`))

			if rec.Code != http.StatusCreated || calls.Load() != 2 || rec.Header().Get(idempotency.ReplayedHeader) != "" {
				t.Fatalf("после неудачи повтор: %d, вызовов %d — ждали 201 и 2", rec.Code, calls.Load())
			}
		})
	}
}

// 4xx — окончательный ответ: повтор того же запроса получит ту же ошибку, а не второй шанс.
func TestIdempotencyKeeps4xx(t *testing.T) {
	var calls atomic.Int32
	s := serve(t, newMemStore(t), statusSeq(&calls, http.StatusBadRequest))

	first := s.do(request(t, "key-1", `{}`))
	again := s.do(request(t, "key-1", `{}`))

	if again.Code != http.StatusBadRequest || again.Body.String() != first.Body.String() || calls.Load() != 1 {
		t.Fatalf("повтор после 400: %d %s, вызовов %d — ждали тот же 400 и 1 вызов", again.Code, again.Body, calls.Load())
	}
	if again.Header().Get(idempotency.ReplayedHeader) != "true" {
		t.Error("нет Idempotent-Replayed у повторённого 400")
	}
}

func TestIdempotencyExpiredKeyRunsAgain(t *testing.T) {
	var calls atomic.Int32
	s := serve(t, newMemStore(t), created(&calls))

	s.do(request(t, "key-1", `{}`))
	s.clock.Advance(idempotency.TTL)
	rec := s.do(request(t, "key-1", `{}`))

	if calls.Load() != 2 || rec.Header().Get(idempotency.ReplayedHeader) != "" {
		t.Fatalf("через TTL ключ забыт: вызовов %d, ждали 2", calls.Load())
	}
}

func TestIdempotencyRejectsBadKey(t *testing.T) {
	tests := map[string]string{
		"256 символов":    strings.Repeat("k", idempotency.KeyMaxLen+1),
		"перевод строки":  "key\n1",
		"не ASCII":        "ключ",
		"управляющий DEL": "key\x7f",
	}
	for name, key := range tests {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			s := serve(t, newMemStore(t), created(&calls))

			rec := s.do(request(t, key, `{}`))

			code, field := problemCode(t, rec)
			if rec.Code != http.StatusBadRequest || code != string(problem.CodeInvalidInput) || field != idempotency.Header {
				t.Fatalf("ответ %d %s %q, ждали 400 invalid_input Idempotency-Key", rec.Code, code, field)
			}
			if calls.Load() != 0 {
				t.Fatal("обработчик вызван с негодным ключом")
			}
		})
	}
	t.Run("ровно 255 символов — годится", func(t *testing.T) {
		var calls atomic.Int32
		s := serve(t, newMemStore(t), created(&calls))
		if rec := s.do(request(t, strings.Repeat("k", idempotency.KeyMaxLen), `{}`)); rec.Code != http.StatusCreated {
			t.Fatalf("ответ %d, ждали 201", rec.Code)
		}
	})
}

func TestIdempotencyBodyTooLarge(t *testing.T) {
	var calls atomic.Int32
	s := serve(t, newMemStore(t), created(&calls))

	rec := s.do(request(t, "key-1", `{"t":"`+strings.Repeat("a", httpjson.MaxBodyBytes)+`"}`))

	if code, _ := problemCode(t, rec); rec.Code != http.StatusRequestEntityTooLarge || code != string(problem.CodeBodyTooLarge) || calls.Load() != 0 {
		t.Fatalf("ответ %d %s, вызовов %d — ждали 413 body_too_large без вызова", rec.Code, code, calls.Load())
	}
	// Текст тот же, что у декодера обработчика: клиенту не важно, кто из двоих прочитал тело.
	if !strings.Contains(rec.Body.String(), `"detail":"`+httpjson.TooLargeMsg()+`"`) {
		t.Fatalf("detail не из httpjson.TooLargeMsg: %s", rec.Body)
	}
}

// Без личности ключ не к чему привязать: это ошибка сборки конвейера, а не клиента.
func TestIdempotencyWithoutPrincipal(t *testing.T) {
	var calls atomic.Int32
	s := serve(t, newMemStore(t), created(&calls))
	r := request(t, "key-1", `{}`)
	r.Header.Set(userHdr, noUser)

	rec := s.do(r)

	if code, _ := problemCode(t, rec); rec.Code != http.StatusInternalServerError || code != string(problem.CodeInternal) || calls.Load() != 0 {
		t.Fatalf("ответ %d %s, вызовов %d — ждали 500 internal без вызова", rec.Code, code, calls.Load())
	}
	if !strings.Contains(s.logs.String(), "нет личности") || !strings.Contains(s.logs.String(), `"level":"ERROR"`) {
		t.Fatalf("в журнале нет причины на уровне ERROR: %s", s.logs)
	}
}

// waitFor ждёт условия: горутины тестов идут своим ходом, и часов у них нет.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("условие не выполнилось за 2 с")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestIdempotencyConcurrent(t *testing.T) {
	store := newMemStore(t)
	var calls atomic.Int32
	release := make(chan struct{})
	s := serve(t, store, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		<-release // держим первый запрос «в работе», пока не придут остальные
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	const n = 10
	// Запросы готовим заранее: request зовёт t.Fatal, а его нельзя вызывать из чужой горутины.
	reqs := make([]*http.Request, n)
	for i := range n {
		reqs[i] = request(t, "key-1", `{"kind":"group"}`)
	}
	codes := make(chan int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			codes <- s.do(reqs[i]).Code
		})
	}
	// Все, кроме первого, должны ответить 409 и не ждать: ждём их, потом отпускаем первого.
	waitFor(t, func() bool { return len(codes) == n-1 })
	close(release)
	wg.Wait()
	close(codes)
	count := map[int]int{}
	for c := range codes {
		count[c]++
	}
	if calls.Load() != 1 || count[http.StatusCreated] != 1 || count[http.StatusConflict] != n-1 {
		t.Fatalf("обработчик вызван %d раз, ответы %v", calls.Load(), count)
	}
	// Первый закончил — теперь повтор получает его ответ, а не 409.
	if rec := s.do(request(t, "key-1", `{"kind":"group"}`)); rec.Code != http.StatusCreated || rec.Header().Get(idempotency.ReplayedHeader) != "true" {
		t.Fatalf("повтор после завершения: %d %v", rec.Code, rec.Header())
	}
}

func TestIdempotencyInProgressCode(t *testing.T) {
	store := newMemStore(t)
	entered, release := make(chan struct{}), make(chan struct{})
	s := serve(t, store, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusCreated)
	}))
	first := request(t, "key-1", `{}`)
	var wg sync.WaitGroup
	wg.Go(func() { s.do(first) })
	<-entered

	rec := s.do(request(t, "key-1", `{}`))
	close(release)
	wg.Wait()

	if code, _ := problemCode(t, rec); rec.Code != http.StatusConflict || code != string(problem.CodeIdempotencyInProgress) {
		t.Fatalf("ответ %d %s, ждали 409 idempotency_in_progress", rec.Code, code)
	}
}

func TestIdempotencyReleasesAfterPanic(t *testing.T) {
	store := newMemStore(t)
	var calls atomic.Int32
	s := serve(t, store, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			panic("бум")
		}
		w.WriteHeader(http.StatusCreated)
	}))
	first := request(t, "key-2", `{}`)
	func() {
		defer func() { _ = recover() }() // в бою панику ловит Recover снаружи
		s.do(first)
	}()
	rec := s.do(request(t, "key-2", `{}`))
	if rec.Code != http.StatusCreated || calls.Load() != 2 {
		t.Fatalf("после паники повтор: %d, вызовов %d — ждали 201 и 2", rec.Code, calls.Load())
	}
}

// Content-Type входит в отпечаток: исправленный запрос с тем же ключом — уже другой
// запрос (422), а не повтор сохранённого 415. Обработчик второй раз не вызывается.
func TestIdempotencyContentTypeInFingerprint(t *testing.T) {
	var calls atomic.Int32
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})
	s := serve(t, newMemStore(t), h)

	wrong := request(t, "key-1", `{}`)
	wrong.Header.Set("Content-Type", "text/plain")
	if rec := s.do(wrong); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("первый ответ %d, ждали 415", rec.Code)
	}
	rec := s.do(request(t, "key-1", `{}`))

	if code, _ := problemCode(t, rec); rec.Code != http.StatusUnprocessableEntity || code != string(problem.CodeIdempotencyKeyReused) {
		t.Fatalf("ответ %d %s, ждали 422 idempotency_key_reused", rec.Code, code)
	}
	if calls.Load() != 1 {
		t.Fatalf("обработчик вызван %d раз, ждали 1", calls.Load())
	}
}

// Отпечаток сравнивает тип, а не написание заголовка: регистр и параметры не делают
// запрос другим.
func TestIdempotencyContentTypeSpellingIsSameRequest(t *testing.T) {
	var calls atomic.Int32
	s := serve(t, newMemStore(t), created(&calls))

	s.do(request(t, "key-1", `{}`))
	again := request(t, "key-1", `{}`)
	again.Header.Set("Content-Type", "Application/JSON; charset=utf-8")
	rec := s.do(again)

	if calls.Load() != 1 || rec.Header().Get(idempotency.ReplayedHeader) != "true" {
		t.Fatalf("ответ %d, вызовов %d — ждали повтор и 1 вызов", rec.Code, calls.Load())
	}
}

// lostComplete — хранилище, в котором ответ не сохраняется: так выглядит процесс, упавший
// между операцией и Complete, — ключ остаётся «в работе».
type lostComplete struct{ *memory.Store }

func (lostComplete) Complete(context.Context, string, string, idempotency.Record) error {
	return errors.New("соединение с базой потеряно")
}

// Бэклог «409 на 24 ч»: ответ не сохранился, но ключ не заперт до конца суток. До срока
// резервации повтор получает 409 «ещё выполняется», после — выполняется заново.
func TestIdempotencyStaleReservationIsTakenOver(t *testing.T) {
	var calls atomic.Int32
	s := serve(t, lostComplete{newMemStore(t)}, created(&calls))
	if rec := s.do(request(t, "key-1", `{}`)); rec.Code != http.StatusCreated {
		t.Fatalf("первый запрос: %d", rec.Code)
	}

	s.clock.Advance(reservationTTL - time.Second)
	early := s.do(request(t, "key-1", `{}`))
	s.clock.Advance(time.Second)
	late := s.do(request(t, "key-1", `{}`))

	if code, _ := problemCode(t, early); early.Code != http.StatusConflict || code != string(problem.CodeIdempotencyInProgress) {
		t.Fatalf("до срока резервации: %d %s, ждали 409 idempotency_in_progress", early.Code, code)
	}
	if late.Code != http.StatusCreated || late.Header().Get(idempotency.ReplayedHeader) != "" || calls.Load() != 2 {
		t.Fatalf("после срока резервации: %d, вызовов %d — ждали новое выполнение (201, 2 вызова)", late.Code, calls.Load())
	}
}

// Ответ, который не удалось сохранить потому, что резервацию уже перехватили, — не сбой
// хранилища: в журнале предупреждение, а не ошибка.
func TestIdempotencyLostReservationIsWarning(t *testing.T) {
	store := newMemStore(t)
	var s server
	s = serve(t, store, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Запрос идёт дольше резервации, и повтор перехватывает её посреди обработки.
		s.clock.Advance(reservationTTL)
		if _, created, err := store.Reserve(context.Background(), "alice", "key-1", idempotency.Record{ExpiresAt: s.clock.Now().Add(idempotency.TTL)}, s.clock.Now()); err != nil || !created {
			t.Errorf("перехват: created=%v, %v", created, err)
		}
		w.WriteHeader(http.StatusCreated)
	}))

	s.do(request(t, "key-1", `{}`))

	if out := s.logs.String(); !strings.Contains(out, "перехватил повтор") || !strings.Contains(out, `"level":"WARN"`) || strings.Contains(out, `"level":"ERROR"`) {
		t.Fatalf("журнал: ждали WARN о перехвате без ERROR:\n%s", out)
	}
}
