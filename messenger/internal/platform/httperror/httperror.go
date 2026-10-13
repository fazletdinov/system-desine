package httperror

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/platform/httpjson"
)

// Problem — ответ на ошибку: статус и тело {"error":{"code","message","field"}}.
type Problem struct {
	Status  int
	Code    string
	Message string
	Field   string // только для ошибок валидации
}

// Classifier — таблица контекста: узнаёт его ошибки (валидацию и сигнальные) и
// говорит, каким ответом они становятся. ok == false — «это не моя ошибка».
type Classifier func(err error) (p Problem, ok bool)

// #region policy
// Write — единственное место, где ошибка становится HTTP-ответом. Порядок важен:
// ошибки разбора тела, потом таблица контекста (виноват клиент или правило домена),
// потом контекст запроса, и всё неузнанное — 500.
func Write(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error, classify Classifier) {
	var decodeErr *httpjson.DecodeError
	if errors.As(err, &decodeErr) {
		respond(w, r, log, Problem{Status: decodeErr.Status, Code: decodeErr.Code, Message: decodeErr.Msg, Field: decodeErr.Field})
		return
	}
	if p, ok := classify(err); ok {
		respond(w, r, log, p)
		return
	}
	switch {
	case errors.Is(err, context.Canceled):
		// Клиент закрыл соединение: отвечать некому. Это не сбой сервера — уровень Info.
		log.Info("клиент ушёл, не дождавшись ответа", "method", r.Method, "path", r.URL.Path, "err", err)
	case errors.Is(err, context.DeadlineExceeded):
		respond(w, r, log, Problem{Status: http.StatusServiceUnavailable, Code: "timeout", Message: "сервер не успел ответить, повторите позже"})
	default:
		// Текст внутренней ошибки может содержать адрес БД, SQL или имя таблицы —
		// клиенту он не нужен и опасен. Детали — только в лог.
		log.Error("внутренняя ошибка", "method", r.Method, "path", r.URL.Path, "err", err)
		respond(w, r, log, Problem{Status: http.StatusInternalServerError, Code: "internal", Message: "внутренняя ошибка сервера"})
	}
}

// #endregion policy

// respond пишет ответ; ошибку записи — только в лог: заголовки уже ушли.
func respond(w http.ResponseWriter, r *http.Request, log *slog.Logger, p Problem) {
	if err := httpjson.WriteError(w, p.Status, p.Code, p.Message, p.Field); err != nil {
		log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
