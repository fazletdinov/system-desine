package main

import (
	"bytes"
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
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"messenger/internal/auth/adapters/argon2id"
	"messenger/internal/auth/adapters/jwt"
	"messenger/internal/platform/openapitest"
)

// fastHasher — единственная разница с боевой сборкой: OWASP-минимум (19 МиБ на хеш)
// сделал бы сквозной тест медленным, а проверяем мы здесь сборку, а не стойкость.
var fastHasher = argon2id.Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}

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
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	handler, err := newApp(config{SigningKey: key, HasherParams: fastHasher}, slog.New(slog.NewJSONHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	v := openapitest.New(t)
	srv := httptest.NewServer(v.Wrap(handler))
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
	log := slog.New(slog.DiscardHandler)
	if _, err := newApp(config{HasherParams: fastHasher}, log); err == nil {
		t.Error("без ключа подписи: ждали ошибку")
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newApp(config{SigningKey: key}, log); err == nil {
		t.Error("с нулевыми параметрами хешера: ждали ошибку")
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("без файла ключа — ошибка с подсказкой", func(t *testing.T) {
		t.Setenv("MESSENGER_JWT_KEY_FILE", "")
		if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "keygen") {
			t.Fatalf("ошибка %v, ждали подсказку про keygen", err)
		}
	})
	t.Run("ключ из файла, адрес по умолчанию", func(t *testing.T) {
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
		t.Setenv("MESSENGER_JWT_KEY_FILE", path)
		t.Setenv("MESSENGER_ADDR", "")

		cfg, err := loadConfig()

		if err != nil || !cfg.SigningKey.Equal(key) || cfg.Addr != defaultAddr || cfg.HasherParams != argon2id.OWASPMinimum {
			t.Fatalf("config %+v, ошибка %v", cfg.Addr, err)
		}
	})
	t.Run("не PEM — ошибка", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "jwt.pem")
		if err := os.WriteFile(path, []byte("не ключ"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("MESSENGER_JWT_KEY_FILE", path)
		if _, err := loadConfig(); err == nil {
			t.Fatal("ждали ошибку")
		}
	})
}
