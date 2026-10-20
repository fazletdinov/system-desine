// Package memory — хранилище ключей идемпотентности в памяти процесса. Просроченные
// записи удаляет фоновая очистка (Sweep, урок 1.5.6); до неё просроченную запись
// только перезаписывал Reserve, а ключ, к которому не возвращались, жил вечно.
// В модуле 2a хранилищем станет Postgres.
package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"messenger/internal/platform/idempotency"
)

type entryKey struct{ scope, key string }

// #region store
type Store struct {
	mu      sync.Mutex // защищает records
	records map[entryKey]idempotency.Record
}

func NewStore() *Store { return &Store{records: make(map[entryKey]idempotency.Record)} }

// Reserve — проверка «есть ли запись» и запись «в работе» под одним замком: между ними
// никто не вклинится, и из двух одновременных запросов ключ займёт ровно один.
func (s *Store) Reserve(ctx context.Context, scope, key string, rec idempotency.Record, now time.Time) (idempotency.Record, bool, error) {
	if err := ctx.Err(); err != nil {
		return idempotency.Record{}, false, fmt.Errorf("idempotency memory: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := entryKey{scope, key}
	if old, ok := s.records[k]; ok && now.Before(old.ExpiresAt) {
		return clone(old), false, nil
	}
	s.records[k] = clone(rec)
	return clone(rec), true, nil
}

// #endregion store

func (s *Store) Complete(ctx context.Context, scope, key string, rec idempotency.Record) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("idempotency memory: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[entryKey{scope, key}] = clone(rec)
	return nil
}

func (s *Store) Release(ctx context.Context, scope, key string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("idempotency memory: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, entryKey{scope, key})
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
	for k, rec := range s.records {
		if now.Before(rec.ExpiresAt) {
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
