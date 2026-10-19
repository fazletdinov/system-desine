package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"

	"messenger/internal/auth/adapters/argon2id"
	"messenger/internal/auth/adapters/jwt"
	"messenger/internal/platform/idempotency"
	"messenger/internal/platform/lifecycle"
	"messenger/internal/platform/openapitest"
)

// fastHasher — единственная разница с боевой сборкой: OWASP-минимум (19 МиБ на хеш)
// сделал бы сквозной тест медленным, а проверяем мы здесь сборку, а не стойкость.
var fastHasher = argon2id.Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}

// testConfig — рабочий конфиг сборки для тестов: новый ключ подписи, быстрый хешер,
// сроки и параллелизм по умолчанию.
func testConfig(t *testing.T) config {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return config{
		SigningKey:            key,
		HasherParams:          fastHasher,
		RequestTimeout:        defaultRequestTimeout,
		IdempotencySweepEvery: defaultIdempotencySweepEvery,
		HashConcurrency:       2,
		HashQueue:             8,
	}
}

// Пароли из тел запросов: их не должно быть ни в ответах, ни в журнале.
const (
	alicePassword = "алиса любит длинные пароли"
	bobPassword   = "боб тоже любит длинные пароли"
	evePassword   = "ева подслушивает всех подряд"
)

// syncBuffer — журнал сервера: обработчики пишут в него из своих горутин.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// client — настоящий HTTP-клиент против настоящего сервера; каждый ответ сверяется
// с контрактом по дороге.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

type response struct {
	status int
	header http.Header
	body   []byte
}

func (r response) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.body, &m); err != nil {
		t.Fatalf("тело %q: %v", r.body, err)
	}
	return m
}

// do шлёт запрос; token == "" — без Authorization; headers — пары «имя, значение».
func (c client) do(method, path, token, body string, headers ...string) response {
	c.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(c.t.Context(), method, c.base+path, rd)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	return response{status: resp.StatusCode, header: resp.Header, body: b}
}

func (c client) expect(r response, status int, what string) {
	c.t.Helper()
	if r.status != status {
		c.t.Fatalf("%s: статус %d, ждали %d; тело %s", what, r.status, status, r.body)
	}
}

func (c client) signUp(username, password string) string {
	c.t.Helper()
	r := c.do(http.MethodPost, "/v1/users", "", `{"username":"`+username+`","display_name":"`+username+`","password":"`+password+`"}`)
	c.expect(r, http.StatusCreated, "регистрация "+username)
	return r.json(c.t)["id"].(string)
}

func (c client) login(username, password string) response {
	c.t.Helper()
	return c.do(http.MethodPost, "/v1/tokens", "", `{"username":"`+username+`","password":"`+password+`"}`)
}

func (c client) token(username, password string) string {
	c.t.Helper()
	r := c.login(username, password)
	c.expect(r, http.StatusOK, "вход "+username)
	return r.json(c.t)["access_token"].(string)
}

func newClient(t *testing.T) (client, *openapitest.Validator, *syncBuffer) {
	t.Helper()
	logs := &syncBuffer{}
	a, err := newApp(testConfig(t), slog.New(slog.NewJSONHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	v := openapitest.New(t)
	srv := httptest.NewServer(v.Wrap(a.handler))
	t.Cleanup(srv.Close)
	return client{t: t, base: srv.URL, http: srv.Client()}, v, logs
}

// withoutRequestID — тело ошибки без единственного поля, которое обязано различаться.
func withoutRequestID(m map[string]any) map[string]any {
	out := maps.Clone(m)
	delete(out, "request_id")
	return out
}

// #region e2e
// TestMessengerEndToEnd — сценарий урока 1.14 целиком через HTTP: регистрация, вход,
// токен, чаты, идемпотентность, сообщения, пагинация, доступ. В конце — проверка, что
// каждая операция контракта хоть раз ответила и ответ сверен с контрактом.
func TestMessengerEndToEnd(t *testing.T) {
	c, v, logs := newClient(t)

	// 1. Регистрация.
	alice := c.signUp("alice", alicePassword)
	bob := c.signUp("bob", bobPassword)
	c.signUp("eve", evePassword)

	// 2. Вход: верный пароль — токен; неверный и несуществующее имя — один и тот же ответ.
	issued := c.login("alice", alicePassword)
	c.expect(issued, http.StatusOK, "вход Алисы")
	if cc := issued.header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control %q", cc)
	}
	aliceToken := issued.json(t)["access_token"].(string)
	wrong, nobody := c.login("alice", "совсем не тот пароль"), c.login("mallory", alicePassword)
	c.expect(wrong, http.StatusUnauthorized, "неверный пароль")
	c.expect(nobody, http.StatusUnauthorized, "нет такого пользователя")
	if wrong.json(t)["code"] != "invalid_credentials" {
		t.Errorf("код %v", wrong.json(t)["code"])
	}
	if a, b := withoutRequestID(wrong.json(t)), withoutRequestID(nobody.json(t)); !reflect.DeepEqual(a, b) {
		t.Errorf("ответы различаются:\n%v\n%v", a, b)
	}
	bobToken, eveToken := c.token("bob", bobPassword), c.token("eve", evePassword)

	// 3. Токен открывает /v1/users/me; без токена и с испорченным — 401 с разным WWW-Authenticate.
	me := c.do(http.MethodGet, "/v1/users/me", aliceToken, "")
	c.expect(me, http.StatusOK, "мой профиль")
	if me.json(t)["id"] != alice {
		t.Fatalf("/v1/users/me вернул %v, ждали Алису %s", me.json(t)["id"], alice)
	}
	c.expect(c.do(http.MethodGet, "/v1/users/"+bob, aliceToken, ""), http.StatusOK, "профиль Боба")
	anon := c.do(http.MethodGet, "/v1/users/me", "", "")
	c.expect(anon, http.StatusUnauthorized, "без токена")
	if got := anon.header.Get("WWW-Authenticate"); got != `Bearer realm="messenger"` {
		t.Errorf("без токена: WWW-Authenticate %q", got)
	}
	// Подделка: payload с sub Боба, подпись — от токена Алисы.
	parts := strings.Split(aliceToken, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	forged := parts[0] + "." + base64.RawURLEncoding.EncodeToString(bytes.Replace(payload, []byte(alice), []byte(bob), 1)) + "." + parts[2]
	bad := c.do(http.MethodGet, "/v1/users/me", forged, "")
	c.expect(bad, http.StatusUnauthorized, "подделанный sub")
	if got := bad.header.Get("WWW-Authenticate"); got != `Bearer realm="messenger", error="invalid_token"` {
		t.Errorf("подделка: WWW-Authenticate %q", got)
	}
	// Старая заглушка больше ничего не значит.
	c.expect(c.do(http.MethodGet, "/v1/users/me", "", "", "X-User-ID", alice), http.StatusUnauthorized, "X-User-ID")

	// 4. Личный чат и группа с ключом идемпотентности.
	direct := c.do(http.MethodPost, "/v1/chats", aliceToken, `{"kind":"direct","member_ids":["`+bob+`"]}`)
	c.expect(direct, http.StatusCreated, "личный чат")
	directID := direct.json(t)["id"].(string)
	groupBody := `{"kind":"group","title":"Команда","member_ids":["` + bob + `"]}`
	key := uuid.NewString()
	group := c.do(http.MethodPost, "/v1/chats", aliceToken, groupBody, "Idempotency-Key", key)
	c.expect(group, http.StatusCreated, "группа")
	again := c.do(http.MethodPost, "/v1/chats", aliceToken, groupBody, "Idempotency-Key", key)
	c.expect(again, http.StatusCreated, "повтор группы")
	if again.header.Get("Idempotent-Replayed") != "true" || !bytes.Equal(again.body, group.body) {
		t.Errorf("повтор не из сохранённого ответа: %v %s", again.header, again.body)
	}
	other := `{"kind":"group","title":"Другая","member_ids":["` + bob + `"]}`
	c.expect(c.do(http.MethodPost, "/v1/chats", aliceToken, other, "Idempotency-Key", key), http.StatusUnprocessableEntity, "ключ с другим телом")
	c.expect(c.do(http.MethodGet, "/v1/chats/"+directID, bobToken, ""), http.StatusOK, "чат глазами Боба")

	// 5. Семь сообщений; повтор с тем же client_message_id — 200, с другим текстом — 409.
	msgs := "/v1/chats/" + directID + "/messages"
	var first string
	for i := range 7 {
		cmid := uuid.NewString()
		if i == 0 {
			first = cmid
		}
		c.expect(c.do(http.MethodPost, msgs, aliceToken, `{"client_message_id":"`+cmid+`","text":"сообщение `+string(rune('1'+i))+`"}`), http.StatusCreated, "сообщение")
	}
	c.expect(c.do(http.MethodPost, msgs, aliceToken, `{"client_message_id":"`+first+`","text":"сообщение 1"}`), http.StatusOK, "повтор сообщения")
	c.expect(c.do(http.MethodPost, msgs, aliceToken, `{"client_message_id":"`+first+`","text":"другой текст"}`), http.StatusConflict, "тот же id, другой текст")

	// 6. Боб листает историю по 3 до конца: семь сообщений без повторов. Ева — посторонняя.
	seen := map[string]bool{}
	for path, pages := msgs+"?limit=3", 0; ; pages++ {
		if pages > 3 {
			t.Fatal("пагинация не заканчивается")
		}
		page := c.do(http.MethodGet, path, bobToken, "")
		c.expect(page, http.StatusOK, "страница истории")
		body := page.json(t)
		for _, m := range body["items"].([]any) {
			id := m.(map[string]any)["id"].(string)
			if seen[id] {
				t.Fatalf("сообщение %s повторилось", id)
			}
			seen[id] = true
		}
		next, ok := body["next_cursor"].(string)
		if !ok {
			break
		}
		path = msgs + "?limit=3&cursor=" + next
	}
	if len(seen) != 7 {
		t.Fatalf("в истории %d сообщений, ждали 7", len(seen))
	}
	c.expect(c.do(http.MethodGet, msgs, eveToken, ""), http.StatusForbidden, "Ева читает чужой чат")

	// 7. Список чатов Алисы: оба, первым — личный (в нём последнее сообщение).
	list := c.do(http.MethodGet, "/v1/chats", aliceToken, "")
	c.expect(list, http.StatusOK, "список чатов")
	items := list.json(t)["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["id"] != directID {
		t.Fatalf("список чатов: %s", list.body)
	}

	// 8. Каждая операция контракта ответила хотя бы раз — и ответ сверен с контрактом.
	if u := v.Uncovered(); len(u) > 0 {
		t.Fatalf("операции контракта без проверенного ответа: %v", u)
	}
	for _, secret := range []string{alicePassword, bobPassword, evePassword, aliceToken} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("секрет %q в журнале", secret)
		}
	}
}

// #endregion e2e

func TestNewAppRejectsBadConfig(t *testing.T) {
	cases := []struct {
		name  string
		spoil func(*config)
	}{
		{"без ключа подписи", func(c *config) { c.SigningKey = nil }},
		{"с нулевыми параметрами хешера", func(c *config) { c.HasherParams = argon2id.Params{} }},
		{"с нулевым таймаутом запроса", func(c *config) { c.RequestTimeout = 0 }},
		{"с нулевым периодом очистки", func(c *config) { c.IdempotencySweepEvery = 0 }},
		{"без мест для хеширования", func(c *config) { c.HashConcurrency = 0 }},
		{"с отрицательной очередью хешей", func(c *config) { c.HashQueue = -1 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := testConfig(t)
			c.spoil(&cfg)
			if _, err := newApp(cfg, slog.New(slog.DiscardHandler)); err == nil {
				t.Fatal("ждали ошибку")
			}
		})
	}
}

// TestAppBackgroundStops — сборка отдаёт фоновую очистку идемпотентности: по периоду
// она удаляет просроченный ключ, а остановка через lifecycle проходит без ошибки.
// Всё в виртуальном времени synctest: сутки TTL проходят мгновенно.
func TestAppBackgroundStops(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logs := &syncBuffer{}
		log := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		a, err := newApp(testConfig(t), log)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.back) != 1 || a.back[0].Name() != "idempotency-sweeper" {
			t.Fatalf("фоновые компоненты: %v, ждали idempotency-sweeper", a.back)
		}
		// Ключ идемпотентности — через API, как его создаёт клиент. Сервер в сети в
		// памяти: в пузыре synctest настоящая сеть не годится.
		srv := httptest.NewTestServer(t, a.handler)
		c := client{t: t, base: "http://messenger", http: srv.Client()}
		bob := c.signUp("bob", bobPassword)
		c.signUp("alice", alicePassword)
		token := c.token("alice", alicePassword)
		body := `{"kind":"group","title":"Команда","member_ids":["` + bob + `"]}`
		c.expect(c.do(http.MethodPost, "/v1/chats", token, body, "Idempotency-Key", uuid.NewString()), http.StatusCreated, "группа с ключом")

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- lifecycle.Run(ctx, log, lifecycle.Stages{Back: a.back}) }()

		// Через TTL ключ просрочен; ещё два периода (с джиттером ≤ 1.1 × Every) — и
		// очистка его точно прошла.
		synctest.Sleep(idempotency.TTL + 2*defaultIdempotencySweepEvery)
		cancel()

		if err := <-done; err != nil {
			t.Fatalf("lifecycle.Run: %v", err)
		}
		out := logs.String()
		if strings.Count(out, `"removed":1`) != 1 || !strings.Contains(out, "фоновые компоненты остановлены") {
			t.Fatalf("журнал: ждали ровно одну очистку с removed=1 и остановку фона:\n%s", out)
		}
	})
}

// TestProbesOutsideAPI — пробы отвечают мимо аутентификации и контракта; дренаж
// переключает только готовность, API продолжает работать.
func TestProbesOutsideAPI(t *testing.T) {
	logs := &syncBuffer{}
	a, err := newApp(testConfig(t), slog.New(slog.NewJSONHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	// Без валидатора контракта: проб в контракте нет — и не должно быть.
	srv := httptest.NewServer(a.handler)
	t.Cleanup(srv.Close)
	c := client{t: t, base: srv.URL, http: srv.Client()}

	c.expect(c.do(http.MethodGet, "/livez", "", ""), http.StatusOK, "livez без токена")
	c.expect(c.do(http.MethodGet, "/readyz", "", ""), http.StatusOK, "readyz без токена")
	r := c.do(http.MethodPost, "/readyz", "", "")
	c.expect(r, http.StatusMethodNotAllowed, "POST readyz")
	if r.header.Get("Allow") != "GET, HEAD" || !strings.HasPrefix(r.header.Get("Content-Type"), "application/problem+json") {
		t.Fatalf("405: заголовки %v", r.header)
	}

	a.probes.StartDraining()

	c.expect(c.do(http.MethodGet, "/readyz", "", ""), http.StatusServiceUnavailable, "readyz во время дренажа")
	c.expect(c.do(http.MethodGet, "/livez", "", ""), http.StatusOK, "livez во время дренажа")
	c.signUp("dora", "дренаж не мешает регистрироваться")
	if strings.Contains(logs.String(), `"path":"/readyz"`) {
		t.Fatal("пробы попали в журнал доступа")
	}
}

// writeKeyFile кладёт новый ключ подписи во временный файл и возвращает его ключ и путь.
func writeKeyFile(t *testing.T) (ed25519.PrivateKey, string) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes, err := jwt.EncodePrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "jwt.pem")
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	return key, path
}

func TestLoadConfig(t *testing.T) {
	t.Run("без файла ключа — ошибка с подсказкой", func(t *testing.T) {
		t.Setenv(envJWTKeyFile, "")
		if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "keygen") {
			t.Fatalf("ошибка %v, ждали подсказку про keygen", err)
		}
	})
	t.Run("ключ из файла, адрес по умолчанию", func(t *testing.T) {
		key, path := writeKeyFile(t)
		t.Setenv(envJWTKeyFile, path)
		t.Setenv(envAddr, "")

		cfg, err := loadConfig()

		if err != nil || !cfg.SigningKey.Equal(key) || cfg.Addr != defaultAddr || cfg.HasherParams != argon2id.OWASPMinimum {
			t.Fatalf("config %+v, ошибка %v", cfg.Addr, err)
		}
	})
	t.Run("сроки", func(t *testing.T) {
		_, path := writeKeyFile(t)
		const (
			req   = envRequestTimeout
			stop  = envShutdownTimeout
			drain = envDrainDelay
		)
		cases := []struct {
			name    string
			env     map[string]string
			want    [3]time.Duration // RequestTimeout, ShutdownTimeout, DrainDelay
			wantErr string           // непусто — ждём ошибку с этим текстом
		}{
			{"ничего не задано — значения по умолчанию", nil, [3]time.Duration{10 * time.Second, 10 * time.Second, 5 * time.Second}, ""},
			{"таймаут запроса из окружения", map[string]string{req: "2s"}, [3]time.Duration{2 * time.Second, 10 * time.Second, 5 * time.Second}, ""},
			{"дренаж без задержки разрешён", map[string]string{drain: "0s"}, [3]time.Duration{10 * time.Second, 10 * time.Second, 0}, ""},
			{"нулевой таймаут запроса", map[string]string{req: "0s"}, [3]time.Duration{}, req},
			{"отрицательный срок остановки", map[string]string{stop: "-1s"}, [3]time.Duration{}, stop},
			{"не срок", map[string]string{drain: "пять"}, [3]time.Duration{}, drain},
			{"дренаж и остановка не укладываются в grace period", map[string]string{drain: "20s", stop: "10s"}, [3]time.Duration{}, "меньше"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				t.Setenv(envJWTKeyFile, path)
				for _, name := range []string{req, stop, drain} {
					t.Setenv(name, c.env[name])
				}

				cfg, err := loadConfig()

				if c.wantErr != "" {
					if err == nil || !strings.Contains(err.Error(), c.wantErr) {
						t.Fatalf("ошибка %v, ждали с %q", err, c.wantErr)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if got := [3]time.Duration{cfg.RequestTimeout, cfg.ShutdownTimeout, cfg.DrainDelay}; got != c.want {
					t.Fatalf("сроки %v, ждали %v", got, c.want)
				}
			})
		}
	})
	t.Run("не PEM — ошибка", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "jwt.pem")
		if err := os.WriteFile(path, []byte("не ключ"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(envJWTKeyFile, path)
		if _, err := loadConfig(); err == nil {
			t.Fatal("ждали ошибку")
		}
	})
}

func TestLoadTimeoutsIncludesBackStop(t *testing.T) {
	// DRAIN_DELAY + SHUTDOWN_TIMEOUT = 28 с укладывались в 30 с до урока 1.5.6; теперь
	// после них ещё до lifecycle.BackStopTimeout (2 с) ждём фоновые компоненты.
	t.Setenv(envDrainDelay, "18s")
	t.Setenv(envShutdownTimeout, "10s")
	t.Setenv(envRequestTimeout, "")
	var cfg config
	if err := loadTimeouts(&cfg); err == nil || !strings.Contains(err.Error(), "2s") {
		t.Fatalf("ошибка %v, ждали отказ с учётом BackStopTimeout 2s", err)
	}
	t.Setenv(envDrainDelay, "17s")
	if err := loadTimeouts(&cfg); err != nil {
		t.Fatalf("17s + 10s + 2s < 30s: %v", err)
	}
}

func TestLoadConcurrencyConfig(t *testing.T) {
	_, path := writeKeyFile(t)
	procs := min(runtime.GOMAXPROCS(0), maxHashConcurrency)
	type want struct {
		sweep              time.Duration
		concurrency, queue int
	}
	cases := []struct {
		name    string
		env     map[string]string
		want    want
		wantErr string // непусто — ждём ошибку с этим текстом (именем переменной)
	}{
		{"ничего не задано — значения по умолчанию", nil, want{time.Minute, procs, 4 * procs}, ""},
		{"очередь по умолчанию — от заданного числа мест", map[string]string{envHashConcurrency: "3"}, want{time.Minute, 3, 12}, ""},
		{"очередь 0 — отказ сразу, когда места заняты", map[string]string{envHashQueue: "0"}, want{time.Minute, procs, 0}, ""},
		{"период очистки из окружения", map[string]string{envIdempotencySweepEvery: "30s"}, want{30 * time.Second, procs, 4 * procs}, ""},
		{"ноль мест для хеширования", map[string]string{envHashConcurrency: "0"}, want{}, envHashConcurrency},
		{"не число", map[string]string{envHashConcurrency: "abc"}, want{}, envHashConcurrency},
		{"отрицательная очередь", map[string]string{envHashQueue: "-1"}, want{}, envHashQueue},
		{"очередь — мусор", map[string]string{envHashQueue: "4x"}, want{}, envHashQueue},
		{"мест на потолке", map[string]string{envHashConcurrency: "256"}, want{time.Minute, 256, 1024}, ""},
		{"мест больше потолка", map[string]string{envHashConcurrency: "257"}, want{}, envHashConcurrency},
		// Без потолка 4 × это значение переполнило бы int и дало отрицательную очередь.
		{"мест — на грани переполнения", map[string]string{envHashConcurrency: "4611686018427387904"}, want{}, envHashConcurrency},
		{"очередь больше потолка", map[string]string{envHashQueue: "1025"}, want{}, envHashQueue},
		{"отрицательный период очистки", map[string]string{envIdempotencySweepEvery: "-1s"}, want{}, envIdempotencySweepEvery},
		{"нулевой период очистки", map[string]string{envIdempotencySweepEvery: "0s"}, want{}, envIdempotencySweepEvery},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(envJWTKeyFile, path)
			for _, name := range []string{envIdempotencySweepEvery, envHashConcurrency, envHashQueue} {
				t.Setenv(name, c.env[name])
			}

			cfg, err := loadConfig()

			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("ошибка %v, ждали с %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := (want{cfg.IdempotencySweepEvery, cfg.HashConcurrency, cfg.HashQueue}); got != c.want {
				t.Fatalf("конфиг %+v, ждали %+v", got, c.want)
			}
		})
	}
}

// TestUncleanPathIsAPIError — пути с «//», «/./», «/../» отвечает API, а не ServeMux перед
// ним: 404 problem+json с request ID, а не голый 307 без журнала (урок 1.7: /v1//x → 404).
func TestUncleanPathIsAPIError(t *testing.T) {
	logs := &syncBuffer{}
	a, err := newApp(testConfig(t), slog.New(slog.NewJSONHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.handler)
	t.Cleanup(srv.Close)
	// Клиент не идёт по редиректам: иначе 307 на очищенный путь спрятал бы ошибку.
	noRedirect := srv.Client()
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	c := client{t: t, base: srv.URL, http: noRedirect}
	c.signUp("frank", "фрэнк проверяет кривые пути")
	token := c.token("frank", "фрэнк проверяет кривые пути")

	r := c.do(http.MethodGet, "/v1//nope", token, "")

	c.expect(r, http.StatusNotFound, "GET /v1//nope")
	if !strings.HasPrefix(r.header.Get("Content-Type"), "application/problem+json") {
		t.Fatalf("Content-Type %q", r.header.Get("Content-Type"))
	}
	body := r.json(t)
	if body["code"] != "not_found" || body["request_id"] == nil || body["request_id"] == "" {
		t.Fatalf("тело %s", r.body)
	}
	if !strings.Contains(logs.String(), `"path":"/v1//nope"`) {
		t.Fatal("запроса нет в журнале доступа")
	}
}
