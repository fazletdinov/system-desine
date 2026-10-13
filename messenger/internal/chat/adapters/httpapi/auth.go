package httpapi

import (
	"net/http"
	"strings"

	"messenger/internal/chat/domain"
	"messenger/internal/platform/httpjson"
)

// UserIDHeader — заголовок, которым клиент сообщает, кто он.
//
// ВНИМАНИЕ: заголовок присылает клиент — подделать его может любой. Это заглушка
// до JWT (часть 1b), годная только для локального стенда; см. docs/adr/0002.
// В 1b меняется этот файл (и появится middleware), обработчики — нет.
const UserIDHeader = "X-User-ID"

// authorID — значение заголовка без пробелов по краям. Формат UUID не проверяет.
func authorID(r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.Header.Get(UserIDHeader))
	return id, id != ""
}

// authenticate разбирает X-User-ID до вызова use case и сам отвечает 401. Если отдать
// сырую строку в use case, кривой id вернулся бы как ValidationError{Field: "user_id"}
// и превратился бы в 400 — а «не знаю, кто вы» это 401, не «неверные данные».
func (h *Handler) authenticate(w http.ResponseWriter, r *http.Request) (domain.UserID, bool) {
	raw, ok := authorID(r)
	if !ok {
		h.unauthenticated(w, r, "нужен заголовок "+UserIDHeader)
		return domain.UserID{}, false
	}
	id, err := domain.ParseUserID(raw)
	if err != nil {
		h.unauthenticated(w, r, UserIDHeader+" должен быть UUID в нижнем регистре")
		return domain.UserID{}, false
	}
	return id, true
}

func (h *Handler) unauthenticated(w http.ResponseWriter, r *http.Request, msg string) {
	if err := httpjson.WriteError(w, http.StatusUnauthorized, "unauthenticated", msg, ""); err != nil {
		h.log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
