package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/chat/domain"
	"messenger/internal/platform/httpjson"
)

// #region error-map
// writeError — единственное место, где ошибка контекста chat становится HTTP-ответом.
// Порядок важен: сначала ошибки разбора и валидации (виноват клиент), потом сигнальные
// ошибки домена, потом контекст, и всё, что не узнали, — 500.
func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var (
		decodeErr *httpjson.DecodeError
		invalid   *domain.ValidationError
	)
	status, code, msg, field := 0, "", "", ""
	switch {
	case errors.As(err, &decodeErr):
		status, code, msg = decodeErr.Status, decodeErr.Code, decodeErr.Msg
	case errors.As(err, &invalid):
		status, code, msg, field = http.StatusBadRequest, "invalid_input", invalid.Reason, invalid.Field
	case errors.Is(err, domain.ErrChatNotFound):
		status, code, msg = http.StatusNotFound, "not_found", domain.ErrChatNotFound.Error()
	case errors.Is(err, domain.ErrNotMember):
		status, code, msg = http.StatusForbidden, "not_member", domain.ErrNotMember.Error()
	case errors.Is(err, domain.ErrConcurrentUpdate):
		status, code, msg = http.StatusConflict, "conflict", domain.ErrConcurrentUpdate.Error()
	case errors.Is(err, context.Canceled):
		// Клиент закрыл соединение: отвечать некому. Это не сбой сервера — уровень Info.
		log.Info("клиент ушёл, не дождавшись ответа", "method", r.Method, "path", r.URL.Path, "err", err)
		return
	case errors.Is(err, context.DeadlineExceeded):
		status, code, msg = http.StatusServiceUnavailable, "timeout", "сервер не успел ответить, повторите позже"
	default:
		// Текст внутренней ошибки может содержать адрес БД, SQL или имя таблицы —
		// клиенту он не нужен и опасен. Детали — только в лог.
		log.Error("внутренняя ошибка", "method", r.Method, "path", r.URL.Path, "err", err)
		status, code, msg = http.StatusInternalServerError, "internal", "внутренняя ошибка сервера"
	}
	if werr := httpjson.WriteError(w, status, code, msg, field); werr != nil {
		log.Error("запись ответа", "path", r.URL.Path, "err", werr)
	}
}

// #endregion error-map
