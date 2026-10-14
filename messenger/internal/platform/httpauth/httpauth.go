package httpauth

import (
	"errors"
	"net/http"
	"strings"

	"messenger/internal/platform/principal"
)

// ErrNoCredentials — запрос не предъявил ничего, по чему можно узнать вызывающего.
var ErrNoCredentials = errors.New("нет данных для аутентификации")

type Authenticator interface {
	Authenticate(r *http.Request) (userID string, err error)
}

// Middleware закрыт по умолчанию: открыты только маршруты из public ("POST /v1/users" —
// метод и путь без параметров). Забытый в списке маршрут получит 401, а не откроется всем.
// Стоит до маршрутизатора, поэтому неизвестный путь без личности тоже получает 401:
// не говорим анониму, какие пути существуют.
func Middleware(a Authenticator, public []string, unauthorized func(w http.ResponseWriter, r *http.Request, err error)) func(http.Handler) http.Handler {
	open := make(map[string]bool, len(public))
	for _, p := range public {
		open[p] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if open[r.Method+" "+r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			id, err := a.Authenticate(r)
			if err != nil {
				unauthorized(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(principal.WithUserID(r.Context(), id)))
		})
	}
}

// #region header-stub
// HeaderStub — заглушка до JWT (ADR-0002): верит заголовку X-User-ID, который присылает
// клиент, то есть любой может назваться кем угодно. Годится только для локального стенда.
// Урок 1.14 заменяет её проверкой токена; хендлеры при этом не меняются.
type HeaderStub struct{}

const UserIDHeader = "X-User-ID"

func (HeaderStub) Authenticate(r *http.Request) (string, error) {
	id := strings.TrimSpace(r.Header.Get(UserIDHeader))
	if id == "" {
		return "", ErrNoCredentials
	}
	return id, nil
}

// #endregion header-stub
