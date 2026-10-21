package idempotency

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	Header         = "Idempotency-Key"
	ReplayedHeader = "Idempotent-Replayed"
	// KeyMaxLen — как у Stripe: UUID с запасом, но не произвольно длинная строка.
	KeyMaxLen = 255
	// TTL — сколько помним ключ. Клиент повторяет секунды и минуты, а не дни;
	// сутки — с запасом на клиентов, которые были офлайн.
	TTL = 24 * time.Hour
	// ReservationGrace — запас срока резервации сверх срока запроса: резервация живёт
	// RequestTimeout + ReservationGrace. Обработчик ограничен RequestTimeout, а после него
	// middleware ещё сохраняет ответ (Complete) — на это и запас. Дольше держать ключ
	// «в работе» незачем: такой запрос уже не закончится, его процесс упал или завис.
	ReservationGrace = 5 * time.Second
)

// ErrReservationLost — Complete от того, кто резервацию больше не держит: её перехватил
// повтор после ReservationTTL или ключ освободили. Ответ не сохранён — повтор выполнит
// операцию по своей резервации.
var ErrReservationLost = errors.New("idempotency: резервация ключа перехвачена или снята")

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
	// Owner — токен резервации. Его выдаёт Reserve, новый при каждом занятии ключа;
	// Complete и Release меняют запись, только если токен совпадает. Иначе запоздавший
	// владелец перехваченной резервации стёр бы или перезаписал запись того, кто её перехватил.
	Owner uuid.UUID
}

// Store — хранилище ключей. scope отделяет ключи разных пользователей: ключ — это
// договорённость клиента с сервером, и совпадение ключей у двух клиентов — не повтор.
//
// Резервация («в работе») держится ReservationTTL хранилища (задаётся в конструкторе).
// Если ответ так и не сохранили — процесс упал между операцией и Complete, — повтор после
// этого срока перехватывает ключ, а не получает 409 до конца суток.
type Store interface {
	// Reserve атомарно: если записи нет, она просрочена к now (ExpiresAt <= now) или это
	// зависшая резервация (не Done, срок резервации истёк к now), — сохраняет rec
	// «в работе» с новым Owner и возвращает (её, true); иначе — существующую запись и false.
	Reserve(ctx context.Context, scope, key string, rec Record, now time.Time) (Record, bool, error)
	// Complete сохраняет готовый ответ резервации rec.Owner. Если резервацию держит уже
	// не он — ErrReservationLost.
	Complete(ctx context.Context, scope, key string, rec Record) error
	// Release удаляет резервацию owner: обработка не удалась, повтор должен пройти заново.
	// Чужую резервацию и незанятый ключ не трогает и ошибкой это не считает.
	Release(ctx context.Context, scope, key string, owner uuid.UUID) error
}

// #endregion record

// Sweeper — фоновая очистка просроченных записей. Отдельный узкий порт, а не метод
// Store: middleware очистка не нужна, и хранилищу не обязательно уметь её в том же
// интерфейсе — у Postgres (модуль 2a) это может быть вовсе задача по расписанию.
type Sweeper interface {
	// Sweep удаляет записи с ExpiresAt <= now; возвращает число удалённых.
	Sweep(ctx context.Context, now time.Time) (removed int, err error)
}
