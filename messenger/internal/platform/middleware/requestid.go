package middleware

import (
	"context"
	"net/http"
)

const (
	RequestIDHeader = "X-Request-ID"
	// requestIDMaxLen и набор символов ниже защищают журнал: чужое значение попадает
	// в каждую строку лога, и перевод строки в нём позволил бы подделать запись.
	requestIDMaxLen = 64
)

type IDGenerator interface {
	NewID() string
}

type requestIDKey struct{}

// RequestID принимает входящий X-Request-ID (его ставит балансировщик или клиент — так
// один запрос виден насквозь в логах нескольких сервисов), если он безопасен, иначе
// выдаёт свой. Значение возвращается в ответе: клиент сообщит его в поддержку.
func RequestID(ids IDGenerator) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(RequestIDHeader)
			if !validRequestID(id) {
				id = ids.NewID()
			}
			w.Header().Set(RequestIDHeader, id)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
		})
	}
}

// RequestIDFrom — id текущего запроса; пустая строка, если RequestID не подключён.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func validRequestID(s string) bool {
	if s == "" || len(s) > requestIDMaxLen {
		return false
	}
	for i := range len(s) {
		c := s[i]
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.'
		if !ok {
			return false
		}
	}
	return true
}
