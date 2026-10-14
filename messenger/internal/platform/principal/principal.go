// Package principal переносит «кто вызывает» от middleware аутентификации к хендлерам.
// Это маленькое общее ядро (shared kernel): его знают все контексты, поэтому в нём —
// только строка id, без типов какого-либо контекста.
package principal

import "context"

// ctxKey — неэкспортируемый тип ключа: никакой другой пакет не сможет ни прочитать,
// ни подменить значение по совпавшему ключу (в отличие от ключа-строки).
type ctxKey struct{}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, userID)
}

// UserIDFrom — id того, кого аутентифицировал middleware. ok == false — личности нет:
// маршрут публичный или middleware не подключили.
func UserIDFrom(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKey{}).(string)
	return id, ok && id != ""
}
