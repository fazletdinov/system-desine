package pgtx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// #region querier
// Querier — то, что адаптеру нужно для запросов: его реализуют и пул, и транзакция.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var (
	_ Querier = (*pgxpool.Pool)(nil)
	_ Querier = (pgx.Tx)(nil)
)

// #endregion querier

// rollbackTimeout — срок на ROLLBACK, отвязанный от ctx запроса. С отменённым ctx pgx
// даже не отправит ROLLBACK, а закроет соединение как «в неизвестном состоянии» —
// пул потеряет его, и следующий запрос заплатит за новое (TCP, TLS, аутентификация).
// Откат — одна короткая команда без данных: ответ приходит за миллисекунды, 5 с —
// с запасом на занятый сервер. Не ответил и за это время — пусть pgx закроет
// соединение: дольше держать горутину запроса незачем.
const rollbackTimeout = 5 * time.Second

// txKey — ключ транзакции в ctx. Неэкспортируемый тип: положить или подменить
// транзакцию в обход WithinTx снаружи пакета нельзя.
type txKey struct{}

// Transactor открывает транзакции на пуле. Реализует порты Transactor use case'ов.
type Transactor struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) (*Transactor, error) {
	if pool == nil {
		return nil, errors.New("pgtx: nil-пул")
	}
	return &Transactor{pool: pool}, nil
}

// #region within-tx
// WithinTx выполняет fn в одной транзакции: ошибка fn — откат, паника в fn — откат и
// та же паника дальше, runtime.Goexit в fn — тоже откат. Ошибку fn возвращает как есть: вызывающий проверяет её через
// errors.Is/As так же, как без транзакции. Если ctx уже несёт транзакцию, fn
// выполняется в ней (см. doc.go: вложенный вызов без SAVEPOINT).
func (t *Transactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pgtx: начало транзакции: %w", err)
	}
	// finished — fn вернулся обычным путём, и транзакцию закроет код ниже. Иначе fn
	// ушёл паникой или runtime.Goexit (t.Fatal в тесте), и откатывает defer: соединение
	// не должно остаться в транзакции и пропасть для пула. Панику передаём дальше —
	// решать, что с ней делать, не нам (middleware.Recover ответит 500).
	finished := false
	defer func() {
		if finished {
			return
		}
		p := recover()
		_ = rollback(ctx, tx) // паника важнее ошибки отката; при Goexit ошибку некому вернуть
		if p != nil {
			panic(p)
		}
	}()
	err = fn(context.WithValue(ctx, txKey{}, tx))
	finished = true
	if err != nil {
		if rbErr := rollback(ctx, tx); rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("pgtx: фиксация: %w", err)
	}
	return nil
}

// #endregion within-tx

// rollback откатывает tx и возвращает соединение в пул даже тогда, когда ctx запроса
// уже отменён (клиент ушёл, истёк срок): см. rollbackTimeout.
func rollback(ctx context.Context, tx pgx.Tx) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
	defer cancel()
	if err := tx.Rollback(ctx); err != nil {
		return fmt.Errorf("pgtx: откат: %w", err)
	}
	return nil
}

// #region q
// Q — исполнитель запросов для адаптера: транзакция из ctx, если её открыл WithinTx,
// иначе пул (каждый запрос — своя неявная транзакция).
func Q(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

// #endregion q
