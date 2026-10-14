package httpauth_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"messenger/internal/platform/httpauth"
	"messenger/internal/platform/principal"
)

var public = []string{"POST /v1/users"}

// headerAuth — тестовый Authenticator: личность — значение заголовка X-Test-User.
// Middleware не знает, как устроена аутентификация, и проверяется отдельно от Bearer.
type headerAuth struct{}

func (headerAuth) Authenticate(r *http.Request) (string, error) {
	id := strings.TrimSpace(r.Header.Get("X-Test-User"))
	if id == "" {
		return "", httpauth.ErrNoCredentials
	}
	return id, nil
}

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
	httpauth.Middleware(headerAuth{}, public, unauthorized)(next).ServeHTTP(httptest.NewRecorder(), r)
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
				r.Header.Set("X-Test-User", tt.header)
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

// fakeVerifier запоминает предъявленный токен и отвечает заданным результатом.
type fakeVerifier struct {
	got *string
	id  string
	err error
}

func (f fakeVerifier) Verify(_ context.Context, token string) (string, error) {
	*f.got = token
	return f.id, f.err
}

func TestBearer(t *testing.T) {
	errBadSig := errors.New("подпись не сошлась")
	tests := []struct {
		name      string
		header    string
		verifyErr error
		wantToken string // что дошло до верификатора; "" — не должен вызываться
		wantID    string
		wantErr   []error
	}{
		{name: "нет заголовка", wantErr: []error{httpauth.ErrNoCredentials}},
		{name: "чужая схема Basic", header: "Basic YWxpY2U6c2VjcmV0", wantErr: []error{httpauth.ErrNoCredentials}},
		{name: "Bearer без токена", header: "Bearer ", wantErr: []error{httpauth.ErrNoCredentials}},
		{name: "одно слово Bearer", header: "Bearer", wantErr: []error{httpauth.ErrNoCredentials}},
		{name: "схема в другом регистре", header: "bearer x", wantToken: "x", wantID: "u-1"},
		{name: "пробелы вокруг токена срезаны", header: "Bearer   abc  ", wantToken: "abc", wantID: "u-1"},
		{name: "верификатор отказал", header: "Bearer bad", verifyErr: errBadSig, wantToken: "bad", wantErr: []error{httpauth.ErrInvalidToken, errBadSig}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			b, err := httpauth.NewBearer(fakeVerifier{got: &got, id: "u-1", err: tt.verifyErr})
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/users/me", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}

			id, err := b.Authenticate(r)

			if got != tt.wantToken {
				t.Errorf("верификатор получил %q, ждали %q", got, tt.wantToken)
			}
			for _, want := range tt.wantErr {
				if !errors.Is(err, want) {
					t.Errorf("ошибка %v, ждали внутри %v", err, want)
				}
			}
			if len(tt.wantErr) == 0 && (err != nil || id != tt.wantID) {
				t.Errorf("id %q, ошибка %v; ждали %q", id, err, tt.wantID)
			}
		})
	}
}

func TestNewBearerNil(t *testing.T) {
	if _, err := httpauth.NewBearer(nil); err == nil {
		t.Fatal("NewBearer(nil): ждали ошибку")
	}
}

func TestChallenge(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "токена не было", err: httpauth.ErrNoCredentials, want: `Bearer realm="messenger"`},
		{name: "токен не прошёл", err: fmt.Errorf("%w: %w", httpauth.ErrInvalidToken, errors.New("просрочен")), want: `Bearer realm="messenger", error="invalid_token"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := httpauth.Challenge(tt.err); got != tt.want {
				t.Fatalf("Challenge %q, ждали %q", got, tt.want)
			}
		})
	}
}
