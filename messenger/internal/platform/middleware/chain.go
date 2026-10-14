package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

// Chain оборачивает h так, что первый middleware в списке — самый внешний:
// Chain(h, a, b) == a(b(h)). Запрос проходит a → b → h, ответ — в обратном порядке.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
