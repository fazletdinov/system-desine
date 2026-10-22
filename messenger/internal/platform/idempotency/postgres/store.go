package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"messenger/internal/platform/idempotency"
	"messenger/internal/platform/pgtx"
)

// Состояния записи — значения столбца state (CHECK idempotency_keys_state_check).
const (
	stateInProgress = "in_progress"
	stateDone       = "done"
)

// reserveAttempts — сколько раз Reserve повторяет «занять — перехватить — прочитать».
// Повтор нужен, только если запись исчезла между шагами (её освободил Release или удалила
// очистка): тогда ключ свободен, и следующая попытка его займёт. Третий подряд такой
// исход — уже не гонка, а сбой, о котором лучше узнать ошибкой.
const reserveAttempts = 3

// #region sweep-batch
// sweepBatch — сколько записей удалять одним DELETE. Порция, а не всё разом: DELETE
// миллиона строк — одна долгая транзакция, которая держит блокировки строк (Reserve того
// же ключа ждал бы её конца), раздувает WAL одним куском и не даёт VACUUM убрать мёртвые
// версии, пока не закончится. Тысяча строк по индексу expires_at — миллисекунды.
const sweepBatch = 1000

// #endregion sweep-batch

type Store struct {
	pool           *pgxpool.Pool
	reservationTTL time.Duration
}

// New — reservationTTL: сколько держится резервация «в работе», после неё повтор
// перехватывает ключ (idempotency.Store).
func New(pool *pgxpool.Pool, reservationTTL time.Duration) (*Store, error) {
	if pool == nil {
		return nil, errors.New("idempotency postgres: nil-пул")
	}
	if reservationTTL <= 0 {
		return nil, errors.New("idempotency postgres: срок резервации должен быть больше нуля")
	}
	return &Store{pool: pool, reservationTTL: reservationTTL}, nil
}

// #region reserve-sql
const (
	// Свободный ключ: строки нет — вставляем. Конфликт — не ошибка, а «ключ занят»:
	// RETURNING тогда пуст. Владелец — новый случайный токен, его выдаёт база.
	insertKey = `INSERT INTO platform.idempotency_keys
    (scope, key, fingerprint, state, owner, reserved_until, expires_at)
VALUES ($1, $2, $3, 'in_progress', gen_random_uuid(), $4, $5)
ON CONFLICT (scope, key) DO NOTHING
RETURNING owner`
	// Занятый ключ, который можно перехватить: запись просрочена или это резервация,
	// срок которой истёк, — ответ на неё так и не сохранили. Условие проверяется под
	// блокировкой строки: второй перехватчик ждёт первого, перечитывает строку (READ
	// COMMITTED) и видит свежий reserved_until — условие уже ложно.
	takeOverKey = `UPDATE platform.idempotency_keys
SET fingerprint = $3, state = 'in_progress', owner = gen_random_uuid(),
    status = NULL, header = NULL, body = NULL, reserved_until = $4, expires_at = $5
WHERE scope = $1 AND key = $2
  AND (expires_at <= $6 OR (state = 'in_progress' AND reserved_until <= $6))
RETURNING owner`
	selectKey = `SELECT fingerprint, state, owner, status, header, body, expires_at
FROM platform.idempotency_keys WHERE scope = $1 AND key = $2`
)

// #endregion reserve-sql

// #region reserve
// Reserve — три шага, и каждый атомарен сам: занять свободный ключ, иначе перехватить
// просроченный или зависший, иначе прочитать, чей он. Одним оператором (INSERT … ON
// CONFLICT DO UPDATE … WHERE) не выйти: при конфликте с невыполненным WHERE он ничего не
// возвращает, и чужую запись всё равно пришлось бы читать отдельно.
func (s *Store) Reserve(ctx context.Context, scope, key string, rec idempotency.Record, now time.Time) (idempotency.Record, bool, error) {
	q := pgtx.Q(ctx, s.pool)
	reservedUntil := now.Add(s.reservationTTL)
	for range reserveAttempts {
		owner, ok, err := s.claim(ctx, q, insertKey, scope, key, rec, reservedUntil)
		if err != nil {
			return idempotency.Record{}, false, fmt.Errorf("idempotency postgres: резервация: %w", err)
		}
		if !ok {
			owner, ok, err = s.claim(ctx, q, takeOverKey, scope, key, rec, reservedUntil, now)
			if err != nil {
				return idempotency.Record{}, false, fmt.Errorf("idempotency postgres: перехват: %w", err)
			}
		}
		if ok {
			rec.Owner = owner
			return rec, true, nil
		}
		existing, err := scanRecord(q.QueryRow(ctx, selectKey, scope, key))
		if errors.Is(err, pgx.ErrNoRows) {
			continue // запись исчезла между шагами — ключ свободен, занимаем заново
		}
		if err != nil {
			return idempotency.Record{}, false, fmt.Errorf("idempotency postgres: чтение ключа: %w", err)
		}
		return existing, false, nil
	}
	return idempotency.Record{}, false, fmt.Errorf("idempotency postgres: ключ исчезал %d раза подряд между шагами резервации", reserveAttempts)
}

// #endregion reserve

// claim выполняет вставку или перехват; ok — строка досталась нам, owner — её новый токен.
func (s *Store) claim(ctx context.Context, q pgtx.Querier, sql, scope, key string, rec idempotency.Record, reservedUntil time.Time, extra ...any) (uuid.UUID, bool, error) {
	args := append([]any{scope, key, rec.Fingerprint[:], reservedUntil, rec.ExpiresAt}, extra...)
	var owner uuid.UUID
	err := q.QueryRow(ctx, sql, args...).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, false, nil
	}
	if err != nil {
		return uuid.UUID{}, false, err
	}
	return owner, true, nil
}

// scanRecord читает строку platform.idempotency_keys в запись порта.
func scanRecord(row pgx.Row) (idempotency.Record, error) {
	var (
		fingerprint []byte
		state       string
		status      *int
		header      http.Header
		rec         idempotency.Record
	)
	if err := row.Scan(&fingerprint, &state, &rec.Owner, &status, &header, &rec.Body, &rec.ExpiresAt); err != nil {
		return idempotency.Record{}, err
	}
	// CHECK в базе держит 32 байта; другая длина — повреждённая строка, а не «другой запрос».
	if len(fingerprint) != sha256.Size {
		return idempotency.Record{}, fmt.Errorf("повреждённая запись: отпечаток %d байт", len(fingerprint))
	}
	copy(rec.Fingerprint[:], fingerprint)
	rec.Done = state == stateDone
	if status != nil {
		rec.Status = *status
	}
	rec.Header = header
	rec.ExpiresAt = rec.ExpiresAt.UTC()
	return rec, nil
}

const (
	// Complete и Release трогают строку, только пока её держит тот же владелец: см.
	// idempotency.Record.Owner.
	completeKey = `UPDATE platform.idempotency_keys
SET state = 'done', status = $4, header = $5, body = $6, expires_at = $7
WHERE scope = $1 AND key = $2 AND owner = $3`
	releaseKey = `DELETE FROM platform.idempotency_keys
WHERE scope = $1 AND key = $2 AND owner = $3`
)

func (s *Store) Complete(ctx context.Context, scope, key string, rec idempotency.Record) error {
	tag, err := pgtx.Q(ctx, s.pool).Exec(ctx, completeKey, scope, key, rec.Owner, rec.Status, rec.Header, rec.Body, rec.ExpiresAt)
	if err != nil {
		return fmt.Errorf("idempotency postgres: сохранение ответа: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return idempotency.ErrReservationLost
	}
	return nil
}

func (s *Store) Release(ctx context.Context, scope, key string, owner uuid.UUID) error {
	if _, err := pgtx.Q(ctx, s.pool).Exec(ctx, releaseKey, scope, key, owner); err != nil {
		return fmt.Errorf("idempotency postgres: освобождение ключа: %w", err)
	}
	return nil
}

// #region sweep
// deleteExpired — порция просроченных. LIMIT у DELETE в PostgreSQL нет, поэтому порцию
// выбирает подзапрос по индексу expires_at, а удаляет DELETE по ctid — физическому адресу
// строки, без второго поиска по ключу. Строку, которую между выбором и удалением обновил
// Reserve (перехват даёт новую версию с новым ctid), DELETE не тронет.
const deleteExpired = `DELETE FROM platform.idempotency_keys
WHERE ctid IN (
    SELECT ctid FROM platform.idempotency_keys WHERE expires_at <= $1 LIMIT $2
)`

// Sweep удаляет записи с ExpiresAt <= now порциями по sweepBatch — каждая своей
// транзакцией, так что Reserve и Complete идут между порциями, а не ждут всю очистку.
func (s *Store) Sweep(ctx context.Context, now time.Time) (int, error) {
	removed := 0
	for {
		// Отмена проверяется между порциями: остановка не ждёт очистки большой таблицы.
		if err := ctx.Err(); err != nil {
			return removed, fmt.Errorf("idempotency postgres: очистка: %w", err)
		}
		tag, err := pgtx.Q(ctx, s.pool).Exec(ctx, deleteExpired, now, sweepBatch)
		if err != nil {
			return removed, fmt.Errorf("idempotency postgres: очистка: %w", err)
		}
		n := int(tag.RowsAffected())
		removed += n
		if n < sweepBatch {
			return removed, nil
		}
	}
}

// #endregion sweep
