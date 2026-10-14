// Package memory — хранилище ключей идемпотентности в памяти процесса. Просроченные
// записи удаляются, когда к ним обращаются; фоновая очистка — забота урока о воркерах
// (модуль 1.5), а в модуле 2a хранилищем станет Postgres.
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

// clone — запись хранит срез и карту: отданная наружу без копии, она менялась бы вместе
// с хранилищем. Header.Clone копирует и срезы значений, не только карту.
func clone(r idempotency.Record) idempotency.Record {
	r.Header = r.Header.Clone()
	r.Body = append([]byte(nil), r.Body...)
	return r
}
