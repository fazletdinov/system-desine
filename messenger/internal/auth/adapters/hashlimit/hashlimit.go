package hashlimit

import (
	"context"
	"errors"
	"fmt"

	"messenger/internal/auth/domain"
	"messenger/internal/auth/usecase"
)

// Limiter реализует usecase.PasswordHasher поверх другого хешера. Hash и Verify делят
// одни и те же места: вход стоит столько же, сколько регистрация, и перебор паролей
// нагружает именно Verify.
type Limiter struct {
	next usecase.PasswordHasher
	// slots — допуск: ёмкость N+Q, по жетону на каждый вызов внутри (считает или ждёт).
	slots chan struct{}
	// run — вычисление: ёмкость N, по жетону на каждый вызов, который сейчас считает.
	run chan struct{}
}

// New: concurrency — сколько хешей считается одновременно (> 0); queue — сколько
// вызовов может ждать места (≥ 0; 0 — отказ сразу, если все места заняты).
func New(next usecase.PasswordHasher, concurrency, queue int) (*Limiter, error) {
	switch {
	case next == nil:
		return nil, errors.New("hashlimit: nil-хешер")
	case concurrency <= 0:
		return nil, fmt.Errorf("hashlimit: одновременных вычислений должно быть больше нуля, получено %d", concurrency)
	case queue < 0:
		return nil, fmt.Errorf("hashlimit: очередь не может быть отрицательной, получено %d", queue)
	}
	return &Limiter{
		next:  next,
		slots: make(chan struct{}, concurrency+queue),
		run:   make(chan struct{}, concurrency),
	}, nil
}

func (l *Limiter) Hash(ctx context.Context, p domain.Password) (domain.PasswordHash, error) {
	return limited(ctx, l, func() (domain.PasswordHash, error) { return l.next.Hash(ctx, p) })
}

func (l *Limiter) Verify(ctx context.Context, plaintext string, hash domain.PasswordHash) (bool, error) {
	return limited(ctx, l, func() (bool, error) { return l.next.Verify(ctx, plaintext, hash) })
}

// #region limiter
// limited пропускает fn через два буферизованных канала-семафора. Почему два, а не один:
// у ограничителя два разных вопроса с разной реакцией на «мест нет».
//   - slots (N+Q) — «можно ли вообще встать в очередь?». Захват неблокирующий: мест нет —
//     ErrOverloaded сразу. Ждать места в очереди бессмысленно — это и есть очередь.
//   - run (N) — «можно ли считать прямо сейчас?». Захват блокирующий, но с выходом по
//     ctx: истёк срок запроса или клиент ушёл — перестаём ждать.
//
// Один канал ёмкостью N ответил бы только на второй вопрос, и очередь росла бы без
// предела — каждая ждущая горутина держит запрос, соединение и память.
// Освобождение — через defer, поэтому место возвращается и при раннем выходе по ctx,
// и при панике в fn, если её перехватят выше (middleware.Recover).
func limited[T any](ctx context.Context, l *Limiter, fn func() (T, error)) (T, error) {
	var zero T
	select {
	case l.slots <- struct{}{}:
	default:
		return zero, usecase.ErrOverloaded
	}
	defer func() { <-l.slots }()

	select {
	case l.run <- struct{}{}:
	case <-ctx.Done():
		return zero, fmt.Errorf("hashlimit: ожидание места: %w", ctx.Err())
	}
	defer func() { <-l.run }()

	return fn()
}

// #endregion limiter
