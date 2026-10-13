package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/platform/httpjson"
	"messenger/internal/user/domain"
)

// writeError — единственное место, где ошибка контекста user становится HTTP-ответом.
// Таблица та же, что в chat/adapters/httpapi; различаются только сигнальные ошибки.
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
	case errors.Is(err, domain.ErrUserNotFound):
		status, code, msg = http.StatusNotFound, "not_found", domain.ErrUserNotFound.Error()
	case errors.Is(err, domain.ErrUsernameTaken):
		status, code, msg = http.StatusConflict, "username_taken", domain.ErrUsernameTaken.Error()
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
