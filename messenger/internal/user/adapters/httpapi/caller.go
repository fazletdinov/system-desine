package httpapi

import (
	"net/http"

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

func (h *Handler) unauthenticated(w http.ResponseWriter, r *http.Request) {
	writeProblem(w, r, h.log, problem.Problem{Code: problem.CodeUnauthenticated, Detail: "не удалось определить пользователя"})
}
