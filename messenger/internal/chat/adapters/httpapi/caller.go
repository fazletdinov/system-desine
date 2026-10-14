package httpapi

import (
	"net/http"

	"messenger/internal/chat/domain"
	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/principal"
)

// #region caller
// caller — пользователь, которого аутентифицировал middleware. Хендлер не знает, как
// именно: заголовком-заглушкой или токеном. Нет личности или она не UUID — 401:
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

func (h *Handler) unauthenticated(w http.ResponseWriter, r *http.Request) {
	if err := httpjson.WriteError(w, http.StatusUnauthorized, "unauthenticated", "не удалось определить пользователя", ""); err != nil {
		h.log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
