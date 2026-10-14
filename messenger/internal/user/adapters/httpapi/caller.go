package httpapi

import (
	"net/http"

	"messenger/internal/platform/httpauth"
	"messenger/internal/platform/principal"
	"messenger/internal/platform/problem"
	"messenger/internal/user/domain"
)

// caller — пользователь, которого аутентифицировал middleware (тот же приём, что в
// адаптере chat). Нет личности или она не UUID — 401: «не знаю, кто вы» — не ошибка
// данных запроса, поэтому до use case дело не доходит.
func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw, ok := principal.UserIDFrom(r.Context())
	if !ok {
		h.unauthenticated(w, r)
		return "", false
	}
	if _, err := domain.ParseUserID(raw); err != nil {
		h.unauthenticated(w, r)
		return "", false
	}
	return raw, true
}

// unauthenticated — 401 с WWW-Authenticate, как требует RFC 9110 §15.5.2. Сюда попадают,
// только если middleware аутентификации не подключён или пропустил запрос без личности,
// так что токена, который можно было бы назвать неверным, здесь нет.
func (h *Handler) unauthenticated(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", httpauth.Challenge(httpauth.ErrNoCredentials))
	writeProblem(w, r, h.log, problem.Problem{Code: problem.CodeUnauthenticated, Detail: "не удалось определить пользователя"})
}

// subjectGone — 401 для токена, чей субъект больше не существует. Токен предъявлен и
// подпись верна, но назвать по нему некого — для клиента это тот же «токен негоден»
// (RFC 6750 §3.1, error="invalid_token"): войти заново и получить новый.
func (h *Handler) subjectGone(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", httpauth.Challenge(httpauth.ErrInvalidToken))
	writeProblem(w, r, h.log, problem.Problem{Code: problem.CodeUnauthenticated, Detail: "пользователь токена не найден, войдите заново"})
}
