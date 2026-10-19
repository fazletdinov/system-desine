package idempotency

import (
	"context"
	"crypto/sha256"
	"net/http"
	"time"
)

const (
	Header         = "Idempotency-Key"
	ReplayedHeader = "Idempotent-Replayed"
	// KeyMaxLen — как у Stripe: UUID с запасом, но не произвольно длинная строка.
	KeyMaxLen = 255
	// TTL — сколько помним ключ. Клиент повторяет секунды и минуты, а не дни;
	// сутки — с запасом на клиентов, которые были офлайн.
	TTL = 24 * time.Hour
)

type Clock interface {
	Now() time.Time
}

// #region record
// Record — что сервер помнит о ключе. Пока Done == false, запрос «в работе».
type Record struct {
	Fingerprint [sha256.Size]byte
	Done        bool
	Status      int
	Header      http.Header // только заголовки, которые нужно повторить: Content-Type, Location
	Body        []byte
	ExpiresAt   time.Time
}

// Store — хранилище ключей. scope отделяет ключи разных пользователей: ключ — это
// договорённость клиента с сервером, и совпадение ключей у двух клиентов — не повтор.
type Store interface {
	// Reserve атомарно: если записи нет или она просрочена к now — сохраняет rec
	// («в работе») и возвращает (rec, true); иначе — существующую запись и false.
	Reserve(ctx context.Context, scope, key string, rec Record, now time.Time) (Record, bool, error)
	// Complete сохраняет готовый ответ.
	Complete(ctx context.Context, scope, key string, rec Record) error
	// Release удаляет запись: обработка не удалась, повтор должен пройти заново.
	Release(ctx context.Context, scope, key string) error
}

// #endregion record

// Sweeper — фоновая очистка просроченных записей. Отдельный узкий порт, а не метод
// Store: middleware очистка не нужна, и хранилищу не обязательно уметь её в том же
// интерфейсе — у Postgres (модуль 2a) это может быть вовсе задача по расписанию.
type Sweeper interface {
	// Sweep удаляет записи с ExpiresAt <= now; возвращает число удалённых.
	Sweep(ctx context.Context, now time.Time) (removed int, err error)
}
