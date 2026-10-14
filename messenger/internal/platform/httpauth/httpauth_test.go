package httpauth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"messenger/internal/platform/httpauth"
	"messenger/internal/platform/principal"
)

var public = []string{"POST /v1/users"}

type result struct {
	called    bool   // дошёл ли запрос до хендлера
	userID    string // что хендлер увидел в principal
	hasUser   bool
	authErr   error // что получил unauthorized
	authCalls int
}

func run(t *testing.T, r *http.Request) result {
	t.Helper()
	var res result
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		res.called = true
		res.userID, res.hasUser = principal.UserIDFrom(r.Context())
	})
	unauthorized := func(w http.ResponseWriter, _ *http.Request, err error) {
		res.authCalls++
		res.authErr = err
		w.WriteHeader(http.StatusUnauthorized)
	}
	httpauth.Middleware(httpauth.HeaderStub{}, public, unauthorized)(next).ServeHTTP(httptest.NewRecorder(), r)
	return res
}

func TestPublicRoutePassesWithoutIdentity(t *testing.T) {
	res := run(t, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/users", nil))

	if !res.called || res.hasUser || res.authCalls != 0 {
		t.Fatalf("публичный маршрут: %+v", res)
	}
}

func TestClosedRoute(t *testing.T) {
	tests := []struct {
		name, method, path, header string
		wantCalled                 bool
		wantUser                   string
	}{
		{name: "без заголовка — 401", method: http.MethodPost, path: "/v1/chats"},
		{name: "одни пробелы — 401", method: http.MethodPost, path: "/v1/chats", header: "   "},
		{name: "метод входит в ключ: GET /v1/users закрыт", method: http.MethodGet, path: "/v1/users"},
		{name: "неизвестный путь без личности — 401", method: http.MethodGet, path: "/v1/nope"},
		{name: "с заголовком — пробелы по краям срезаны", method: http.MethodPost, path: "/v1/chats", header: "  u-1 ", wantCalled: true, wantUser: "u-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
			if tt.header != "" {
				r.Header.Set(httpauth.UserIDHeader, tt.header)
			}

			res := run(t, r)

			if res.called != tt.wantCalled {
				t.Fatalf("хендлер вызван: %v, ждали %v", res.called, tt.wantCalled)
			}
			if tt.wantCalled {
				if !res.hasUser || res.userID != tt.wantUser {
					t.Fatalf("principal %q %v, ждали %q", res.userID, res.hasUser, tt.wantUser)
				}
				return
			}
			if res.authCalls != 1 || !errors.Is(res.authErr, httpauth.ErrNoCredentials) {
				t.Fatalf("unauthorized: вызовов %d, err %v", res.authCalls, res.authErr)
			}
		})
	}
}
