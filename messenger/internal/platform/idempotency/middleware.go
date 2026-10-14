package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/principal"
	"messenger/internal/platform/problem"
)

// #region middleware
// Middleware делает POST-маршруты из routes идемпотентными по заголовку Idempotency-Key
// (как у Stripe; черновик IETF draft-ietf-httpapi-idempotency-key-header). Без заголовка —
// обычная обработка. Маршрут — шаблон ServeMux ("POST /v1/chats"): middleware подключается
// после маршрутизатора (через сгенерированный обработчик), где r.Pattern уже заполнен.
//
// Ограничение: запись ключа и бизнес-операция — разные операции. Упади процесс между
// ними — повтор выполнится заново. Атомарно станет в 2a, когда запись ключа переедет
// в транзакцию операции (ADR-0005).
func Middleware(store Store, clock Clock, log *slog.Logger, routes ...string) func(http.Handler) http.Handler {
	guarded := make(map[string]bool, len(routes))
	for _, r := range routes {
		guarded[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(Header)
			if key == "" || !guarded[r.Pattern] {
				next.ServeHTTP(w, r)
				return
			}
			g := guard{store: store, clock: clock, log: log}
			g.serve(w, r, next, key)
		})
	}
}

// #endregion middleware

type guard struct {
	store Store
	clock Clock
	log   *slog.Logger
}

// #region serve
func (g guard) serve(w http.ResponseWriter, r *http.Request, next http.Handler, key string) {
	if !validKey(key) {
		g.write(w, r, problem.Invalid(Header, "от 1 до 255 печатных ASCII-символов"))
		return
	}
	scope, ok := principal.UserIDFrom(r.Context())
	if !ok {
		// Ключ без пользователя не к чему привязать. Сюда можно попасть, только если
		// middleware подключили раньше аутентификации, — это ошибка сборки, а не клиента.
		g.log.Error("idempotency: нет личности в context — проверьте порядок middleware", "path", r.URL.Path)
		g.write(w, r, problem.Problem{Code: problem.CodeInternal})
		return
	}
	// Тело нужно целиком: оно входит в отпечаток. Читаем его здесь с тем же лимитом,
	// что и декодер обработчика, и возвращаем обработчику копией.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpjson.MaxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			g.write(w, r, problem.Problem{Code: problem.CodeBodyTooLarge, Detail: "тело больше 1 МиБ"})
			return
		}
		problem.Respond(w, r, g.log, err, notOurs)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	fp := fingerprint(r, body)
	now := g.clock.Now()
	rec, created, err := g.store.Reserve(r.Context(), scope, key, Record{Fingerprint: fp, ExpiresAt: now.Add(TTL)}, now)
	if err != nil {
		problem.Respond(w, r, g.log, err, notOurs)
		return
	}
	if !created {
		g.replay(w, r, rec, fp)
		return
	}
	g.run(w, r, next, scope, key, rec)
}

// #endregion serve

// notOurs — у middleware нет своих ошибок для таблицы: всё, что не разбор тела и не
// контекст, — 500 по общей политике problem.Respond.
func notOurs(error) (problem.Problem, bool) { return problem.Problem{}, false }

// #region run
// run выполняет запрос впервые и запоминает ответ. После обработчика клиент мог уйти —
// с хранилищем говорим вне его отмены, иначе ключ навсегда остался бы «в работе».
func (g guard) run(w http.ResponseWriter, r *http.Request, next http.Handler, scope, key string, rec Record) {
	ctx := context.WithoutCancel(r.Context())
	cw := &captureWriter{ResponseWriter: w}
	defer func() {
		if v := recover(); v != nil {
			g.release(ctx, scope, key)
			panic(v) // ответит Recover снаружи
		}
	}()
	next.ServeHTTP(cw, r)
	// 5xx и «ничего не записано» (клиент ушёл) — запрос не удался: освобождаем ключ,
	// чтобы повтор прошёл заново. 2xx и 4xx — окончательный ответ, его и повторяем.
	if cw.status == 0 || cw.status >= http.StatusInternalServerError {
		g.release(ctx, scope, key)
		return
	}
	rec.Done, rec.Status, rec.Body = true, cw.status, cw.body.Bytes()
	rec.Header = http.Header{}
	for _, h := range []string{"Content-Type", "Location"} {
		if v := cw.Header().Get(h); v != "" {
			rec.Header.Set(h, v)
		}
	}
	if err := g.store.Complete(ctx, scope, key, rec); err != nil {
		g.log.Error("idempotency: не удалось сохранить ответ", "key", key, "err", err)
	}
}

// #endregion run

// #region replay
// replay отвечает на запрос, ключ которого уже занят: тот же ответ, «ещё выполняется»
// или «ключ от другого запроса». Отпечаток сравниваем первым: чужой запрос не должен
// узнать даже то, что под этим ключом что-то выполняется.
func (g guard) replay(w http.ResponseWriter, r *http.Request, rec Record, fp [sha256.Size]byte) {
	switch {
	case rec.Fingerprint != fp:
		g.write(w, r, problem.Problem{Code: problem.CodeIdempotencyKeyReused, Detail: "ключ уже использован для другого запроса"})
	case !rec.Done:
		g.write(w, r, problem.Problem{Code: problem.CodeIdempotencyInProgress, Detail: "запрос с этим ключом ещё выполняется, повторите позже"})
	default:
		for k, v := range rec.Header {
			w.Header()[k] = v
		}
		w.Header().Set(ReplayedHeader, "true")
		w.WriteHeader(rec.Status)
		if _, err := w.Write(rec.Body); err != nil {
			g.log.Error("запись ответа", "path", r.URL.Path, "err", err)
		}
	}
}

// #endregion replay

func (g guard) release(ctx context.Context, scope, key string) {
	if err := g.store.Release(ctx, scope, key); err != nil {
		g.log.Error("idempotency: не удалось освободить ключ", "key", key, "err", err)
	}
}

func (g guard) write(w http.ResponseWriter, r *http.Request, p problem.Problem) {
	if err := problem.Write(w, r, p); err != nil {
		g.log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}

// fingerprint отличает повтор от другого запроса с тем же ключом: тот же метод, путь
// с запросом и то же тело. Разделитель — байт 0: он не встречается ни в методе, ни в пути.
func fingerprint(r *http.Request, body []byte) [sha256.Size]byte {
	h := sha256.New()
	h.Write([]byte(r.Method))
	h.Write([]byte{0})
	h.Write([]byte(r.URL.RequestURI()))
	h.Write([]byte{0})
	h.Write(body)
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}

// validKey — непустой, не длиннее KeyMaxLen, только печатные ASCII (0x20–0x7e): ключ
// уходит в журнал и в хранилище, и управляющие символы там не нужны.
func validKey(k string) bool {
	if k == "" || len(k) > KeyMaxLen {
		return false
	}
	for i := range len(k) {
		if k[i] < 0x20 || k[i] > 0x7e {
			return false
		}
	}
	return true
}

// captureWriter пропускает ответ клиенту и копит его для повтора.
type captureWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (c *captureWriter) WriteHeader(code int) {
	if c.status == 0 {
		c.status = code
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureWriter) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}

// Unwrap даёт http.ResponseController добраться до настоящего ResponseWriter (Flush и др.).
func (c *captureWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }
