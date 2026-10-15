package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// #region timeout
// Timeout ставит каждому запросу срок d через context. Обработчики и use case'ы
// уважают ctx (урок 1.5): с истечением срока они бросают работу, а адаптер отвечает
// 503 timeout сам. onTimeout — страховка на случай, когда обработчик вернулся по
// сроку, ничего не записав.
//
// Timeout не прерывает обработчик, который ctx не смотрит, — в Go горутину снаружи не
// остановить. Такой обработчик ответит поздно, и его ответ дойдёт как есть; от совсем
// зависших спасает WriteTimeout сервера. http.TimeoutHandler прерывал бы ответ, но для
// этого буферизует его целиком и ломает потоковую передачу (модуль 3), поэтому не он.
func Timeout(d time.Duration, onTimeout func(w http.ResponseWriter, r *http.Request)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r.WithContext(ctx))
			// Отвечаем, только если ответа ещё нет, истёк именно наш срок и клиент на месте:
			// ушедшему клиенту (r.Context() отменён) отвечать некому.
			if rec.status == 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) && r.Context().Err() == nil {
				onTimeout(rec, r)
			}
		})
	}
}

// #endregion timeout
