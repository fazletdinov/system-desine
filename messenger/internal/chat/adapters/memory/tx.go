package memory

import "context"

// Tx — Transactor для хранилищ в памяти: выполняет fn как есть, без отката. Отменить
// записи в картах после ошибки ему нечем, поэтому атомарности он не даёт — только
// подходит под порт, чтобы use case'ы работали с хранилищем в памяти. Используется
// только в тестах: сервер с модуля 2a работает на Postgres. Откат проверяется на
// Postgres (platform/pgtx).
type Tx struct{}

func (Tx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
