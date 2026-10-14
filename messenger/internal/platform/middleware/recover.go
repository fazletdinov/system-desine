package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// #region recover
// Recover превращает панику обработчика в 500 и запись в журнале со стеком: без него
// net/http закрыл бы соединение, а клиент не получил бы ответа. Тело ответа пишет respond —
// формат ошибок задаёт composition root. Если обработчик успел отправить заголовки,
// поменять статус уже нельзя: net/http только предупредит о повторном WriteHeader.
func Recover(log *slog.Logger, respond func(w http.ResponseWriter, r *http.Request)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				// ErrAbortHandler — договорённость net/http «прервать ответ молча»; глотать её нельзя.
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				log.Error("паника в обработчике", "panic", v, "stack", string(debug.Stack()), "request_id", RequestIDFrom(r.Context()))
				respond(w, r)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// #endregion recover
