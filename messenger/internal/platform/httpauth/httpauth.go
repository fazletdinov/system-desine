package httpauth

import (
	"context"
	"errors"
	"fmt"
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

// #region bearer
// ErrInvalidToken — токен предъявлен, но не прошёл проверку.
var ErrInvalidToken = errors.New("токен не прошёл проверку")

// TokenVerifier — порт проверки токена, объявлен здесь, у потребителя. Реализация —
// auth/adapters/jwt; её подставляет composition root.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (userID string, err error)
}

// Bearer — аутентификация по заголовку Authorization: Bearer <токен> (RFC 6750).
type Bearer struct{ verifier TokenVerifier }

func NewBearer(v TokenVerifier) (*Bearer, error) {
	if v == nil {
		return nil, errors.New("httpauth: nil-зависимость")
	}
	return &Bearer{verifier: v}, nil
}

func (b *Bearer) Authenticate(r *http.Request) (string, error) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	// Схема в HTTP нечувствительна к регистру (RFC 9110 §11.1). Другая схема (Basic) —
	// для нас то же, что отсутствие данных: их нечем проверить.
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", ErrNoCredentials
	}
	id, err := b.verifier.Verify(r.Context(), strings.TrimSpace(token))
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	return id, nil
}

// Challenge — значение WWW-Authenticate для ответа 401 (RFC 6750 §3): error="invalid_token",
// если токен был, но не прошёл; без error — если токена не было вовсе. Почему именно
// не прошёл — клиенту не сообщаем: это подсказка атакующему.
func Challenge(err error) string {
	if errors.Is(err, ErrInvalidToken) {
		return `Bearer realm="messenger", error="invalid_token"`
	}
	return `Bearer realm="messenger"`
}

// #endregion bearer
