package httpapi

import (
	"net/http"

	"messenger/internal/chat/domain"
	"messenger/internal/platform/httpauth"
	"messenger/internal/platform/principal"
	"messenger/internal/platform/problem"
)

// #region caller
// caller — пользователь, которого аутентифицировал middleware. Хендлер не знает, как
// именно: проверкой токена или чем-то ещё. Нет личности или она не UUID — 401:
// «не знаю, кто вы» — не ошибка данных запроса, поэтому до use case дело не доходит.
func (h *Handler) caller(w http.ResponseWriter, r *http.Request) (domain.UserID, bool) {
	raw, ok := principal.UserIDFrom(r.Context())
	if !ok {
		h.unauthenticated(w, r)
		return domain.UserID{}, false
	}
	id, err := domain.ParseUserID(raw)
	if err != nil {
		h.unauthenticated(w, r)
		return domain.UserID{}, false
	}
	return id, true
}

// #endregion caller

// unauthenticated — 401 с WWW-Authenticate, как требует RFC 9110 §15.5.2. Сюда попадают,
// только если middleware аутентификации не подключён или пропустил запрос без личности,
// так что токена, который можно было бы назвать неверным, здесь нет.
func (h *Handler) unauthenticated(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", httpauth.Challenge(httpauth.ErrNoCredentials))
	writeProblem(w, r, h.log, problem.Problem{Code: problem.CodeUnauthenticated, Detail: "не удалось определить пользователя"})
}
