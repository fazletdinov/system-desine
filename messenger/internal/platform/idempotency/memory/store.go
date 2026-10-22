// Package memory — хранилище ключей идемпотентности в памяти процесса. Просроченные
// записи удаляет фоновая очистка (Sweep, урок 1.5.6); до неё просроченную запись
// только перезаписывал Reserve, а ключ, к которому не возвращались, жил вечно.
// Сервер с модуля 2a хранит ключи в Postgres (idempotency/postgres); это хранилище —
// эталон контракта storetest и фейк для тестов middleware.
package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"messenger/internal/platform/idempotency"
)

type entryKey struct{ scope, key string }

// entry — запись и срок её резервации: срок — деталь хранилища, наружу не отдаётся.
type entry struct {
	rec           idempotency.Record
	reservedUntil time.Time
}

// #region store
type Store struct {
	reservationTTL time.Duration
	mu             sync.Mutex // защищает records
	records        map[entryKey]entry
}

// NewStore — reservationTTL: сколько держится резервация «в работе», после неё повтор
// перехватывает ключ (idempotency.Store).
func NewStore(reservationTTL time.Duration) (*Store, error) {
	if reservationTTL <= 0 {
		return nil, errors.New("idempotency memory: срок резервации должен быть больше нуля")
	}
	return &Store{reservationTTL: reservationTTL, records: make(map[entryKey]entry)}, nil
}

// Reserve — проверка «есть ли запись» и запись «в работе» под одним замком: между ними
// никто не вклинится, и из двух одновременных запросов ключ займёт ровно один. Так же,
// под замком, из двух перехватчиков зависшей резервации её получает один.
func (s *Store) Reserve(ctx context.Context, scope, key string, rec idempotency.Record, now time.Time) (idempotency.Record, bool, error) {
	if err := ctx.Err(); err != nil {
		return idempotency.Record{}, false, fmt.Errorf("idempotency memory: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := entryKey{scope, key}
	if old, ok := s.records[k]; ok && !old.free(now) {
		return clone(old.rec), false, nil
	}
	rec.Owner = uuid.New()
	s.records[k] = entry{rec: clone(rec), reservedUntil: now.Add(s.reservationTTL)}
	return clone(rec), true, nil
}

// free — ключ можно занять: запись просрочена или это зависшая резервация. Границы
// включительны, как у Sweep: в момент срока запись уже не держит ключ.
func (e entry) free(now time.Time) bool {
	return !now.Before(e.rec.ExpiresAt) || (!e.rec.Done && !now.Before(e.reservedUntil))
}

// #endregion store

func (s *Store) Complete(ctx context.Context, scope, key string, rec idempotency.Record) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("idempotency memory: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := entryKey{scope, key}
	old, ok := s.records[k]
	if !ok || old.rec.Owner != rec.Owner {
		return idempotency.ErrReservationLost
	}
	s.records[k] = entry{rec: clone(rec), reservedUntil: old.reservedUntil}
	return nil
}

func (s *Store) Release(ctx context.Context, scope, key string, owner uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("idempotency memory: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := entryKey{scope, key}
	if old, ok := s.records[k]; ok && old.rec.Owner == owner {
		delete(s.records, k)
	}
	return nil
}

// #region sweep
// sweepBatch — сколько записей удалять за одно взятие замка. Компромисс: вся карта
// разом держала бы замок на время, пропорциональное числу ключей, и Reserve всех
// запросов ждал бы очистку; маленькая порция — больше проходов. Между порциями замок
// отпускается, и запросы проходят. Цена — каждый проход заново просматривает живые
// записи с начала (итерацию по карте не продолжить после отпускания замка), но при
// тысячах ключей это микросекунды.
const sweepBatch = 1000

// Sweep удаляет записи с ExpiresAt <= now порциями по sweepBatch. Граница — та же,
// что у Reserve: в момент ExpiresAt запись уже просрочена.
func (s *Store) Sweep(ctx context.Context, now time.Time) (int, error) {
	removed := 0
	for {
		// Отмена проверяется между порциями: очистка большой карты не держит остановку.
		if err := ctx.Err(); err != nil {
			return removed, fmt.Errorf("idempotency memory: очистка: %w", err)
		}
		n := s.deleteExpired(now)
		removed += n
		if n < sweepBatch {
			return removed, nil
		}
	}
}

// deleteExpired удаляет до sweepBatch просроченных записей под одним замком.
func (s *Store) deleteExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for k, e := range s.records {
		if now.Before(e.rec.ExpiresAt) {
			continue
		}
		// Удалять во время range можно: спецификация Go это разрешает.
		delete(s.records, k)
		if n++; n == sweepBatch {
			break
		}
	}
	return n
}

// #endregion sweep

// clone — запись хранит срез и карту: отданная наружу без копии, она менялась бы вместе
// с хранилищем. Header.Clone копирует и срезы значений, не только карту.
func clone(r idempotency.Record) idempotency.Record {
	r.Header = r.Header.Clone()
	r.Body = append([]byte(nil), r.Body...)
	return r
}
