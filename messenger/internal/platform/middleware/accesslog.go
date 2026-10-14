package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// AccessLog пишет одну строку на запрос после ответа. Путь — без строки запроса:
// в ней бывают курсоры и прочее, чему не место в журнале. Статус 0 — обработчик ничего
// не записал (так бывает, когда клиент ушёл раньше ответа).
func AccessLog(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			log.LogAttrs(r.Context(), slog.LevelInfo, "запрос",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int("bytes", rec.bytes),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", RequestIDFrom(r.Context())),
			)
		})
	}
}
