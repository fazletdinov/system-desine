package problem

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"messenger/internal/platform/httpjson"
	"messenger/internal/platform/middleware"
)

// Classifier — таблица контекста: узнаёт его ошибки и говорит, какой проблемой они
// становятся. ok == false — «это не моя ошибка».
type Classifier func(err error) (p Problem, ok bool)

// #region respond
// Respond — единая политика: разбор тела, таблица контекста, контекст запроса,
// и всё неузнанное — 500 без деталей наружу (детали — только в журнал).
func Respond(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error, classify Classifier) {
	var decodeErr *httpjson.DecodeError
	if errors.As(err, &decodeErr) {
		p := Problem{Code: Code(decodeErr.Code), Detail: decodeErr.Msg}
		if decodeErr.Field != "" {
			p.InvalidParams = []InvalidParam{{Name: decodeErr.Field, Reason: decodeErr.Msg}}
		}
		write(w, r, log, p)
		return
	}
	if p, ok := classify(err); ok {
		write(w, r, log, p)
		return
	}
	switch {
	case errors.Is(err, context.Canceled) && r.Context().Err() != nil:
		// Клиент ушёл: отвечать некому. Проверяем именно контекст запроса — Canceled
		// от внутреннего контекста при живом клиенте это сбой сервера (иначе пустой 200).
		log.Info("клиент ушёл, не дождавшись ответа", "method", r.Method, "path", r.URL.Path, "err", err)
	case errors.Is(err, context.DeadlineExceeded):
		write(w, r, log, Problem{Code: CodeTimeout, Detail: "повторите запрос позже"})
	default:
		log.Error("внутренняя ошибка", "method", r.Method, "path", r.URL.Path, "err", err, "request_id", middleware.RequestIDFrom(r.Context()))
		write(w, r, log, Problem{Code: CodeInternal})
	}
}

// #endregion respond

func write(w http.ResponseWriter, r *http.Request, log *slog.Logger, p Problem) {
	if err := Write(w, r, p); err != nil {
		log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
