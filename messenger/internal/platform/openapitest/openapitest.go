package openapitest

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// Validator — контракт, загруженный для одного теста, и операции, ответы которых он видел.
type Validator struct {
	t      testing.TB
	doc    *openapi3.T
	router routers.Router

	mu   sync.Mutex // защищает seen: обработчики в тестах бывают параллельными
	seen map[string]bool
}

// New загружает контракт. Путь — от этого файла, а не от рабочего каталога теста:
// тесты разных пакетов запускаются из разных каталогов.
func New(t testing.TB) *Validator {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	spec := filepath.Join(filepath.Dir(file), "..", "..", "..", "api", "openapi.yaml")
	doc, err := openapi3.NewLoader().LoadFromFile(spec)
	if err != nil {
		t.Fatalf("openapitest: загрузка контракта: %v", err)
	}
	if err := doc.Validate(t.Context()); err != nil {
		t.Fatalf("openapitest: контракт некорректен: %v", err)
	}
	// Роутер kin-openapi сверяет и хост с servers; в тестах хост другой.
	doc.Servers = nil
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("openapitest: роутер: %v", err)
	}
	return &Validator{t: t, doc: doc, router: router, seen: map[string]bool{}}
}

// Wrap пропускает запрос через h, проверяет ответ по контракту и отдаёт его дальше как есть.
func (v *Validator) Wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r)
		v.check(r, rec)
		for k, vals := range rec.Header() {
			w.Header()[k] = vals
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	})
}

// #region check
func (v *Validator) check(r *http.Request, rec *httptest.ResponseRecorder) {
	v.t.Helper()
	route, params, err := v.router.FindRoute(r)
	if err != nil {
		// Пути нет в контракте: так и должно быть только у ответов маршрутизатора
		// и у отказа в аутентификации, который приходит раньше маршрутизации.
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusUnauthorized {
			v.t.Errorf("openapitest: %s %s нет в контракте, а ответ %d", r.Method, r.URL.Path, rec.Code)
		}
		return
	}
	opts := &openapi3filter.Options{IncludeResponseStatus: true}
	in := &openapi3filter.RequestValidationInput{Request: r, PathParams: params, Route: route, Options: opts}
	out := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: in,
		Status:                 rec.Code,
		Header:                 rec.Header(),
		Body:                   io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
		Options:                opts,
	}
	if err := openapi3filter.ValidateResponse(r.Context(), out); err != nil {
		v.t.Errorf("openapitest: %s %s → %d не по контракту: %v\nтело: %s", r.Method, r.URL.Path, rec.Code, err, rec.Body.String())
	}
	v.mu.Lock()
	v.seen[route.Method+" "+route.Path] = true
	v.mu.Unlock()
}

// #endregion check

// Uncovered — операции контракта, ни один ответ которых не прошёл через Wrap.
func (v *Validator) Uncovered() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out []string
	for path, item := range v.doc.Paths.Map() {
		for method := range item.Operations() {
			if key := method + " " + path; !v.seen[key] {
				out = append(out, key)
			}
		}
	}
	slices.Sort(out)
	return out
}
