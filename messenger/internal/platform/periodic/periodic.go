package periodic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"
)

// Config — что и как часто делать.
type Config struct {
	Name  string
	Every time.Duration // > 0
	// Jitter — доля случайного разброса периода, [0, 1): при 0.1 интервал случаен в
	// пределах Every ± 10 %. Без разброса экземпляры, стартовавшие вместе после
	// выкладки, так и ходили бы в общую базу одновременно («эффект стада»).
	Jitter float64
	// Fn — одна итерация. Ошибка журналируется и не останавливает задачу: пропущенная
	// очистка — не повод ронять процесс, следующая итерация попробует снова.
	Fn  func(ctx context.Context) error
	Log *slog.Logger
}

// Task — периодическая задача; реализует lifecycle.Component.
type Task struct {
	cfg Config
	// rnd — источник джиттера. Им пользуется только горутина Run, поэтому замок не
	// нужен; тест подменяет его генератором с известным сидом.
	rnd *rand.Rand
}

// New проверяет конфиг: задача с нулевым периодом крутилась бы без пауз, а без Fn
// или журнала упала бы на первой итерации — лучше отказать при сборке.
func New(c Config) (*Task, error) {
	switch {
	case c.Name == "":
		return nil, errors.New("periodic: нужно имя задачи")
	case c.Every <= 0:
		return nil, fmt.Errorf("periodic %s: период должен быть больше нуля, получено %s", c.Name, c.Every)
	case c.Jitter < 0 || c.Jitter >= 1:
		return nil, fmt.Errorf("periodic %s: джиттер должен быть в [0, 1), получено %v", c.Name, c.Jitter)
	case c.Fn == nil:
		return nil, fmt.Errorf("periodic %s: нужна функция итерации", c.Name)
	case c.Log == nil:
		return nil, fmt.Errorf("periodic %s: нужен журнал", c.Name)
	}
	return &Task{cfg: c, rnd: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))}, nil
}

func (t *Task) Name() string { return t.cfg.Name }

// #region periodic-run
// Run вызывает Fn через каждый интервал Every ± Jitter до отмены ctx и тогда
// возвращает nil. Первая итерация — через интервал, а не сразу: все экземпляры
// после выкладки иначе начали бы с одновременной очистки.
//
// Почему time.Timer, а не time.Ticker: у Ticker период фиксирован, а джиттер нужен
// на каждый интервал — Timer взводится заново (Reset) со своим случайным сроком.
// Заодно итерации не наслаиваются: таймер взводится после конца итерации, так что
// итерация дольше периода просто сдвигает следующую.
func (t *Task) Run(ctx context.Context) error {
	timer := time.NewTimer(t.interval())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		// Ошибка из-за остановки посреди итерации — не сбой: молчим и выходим на
		// следующем круге по ctx.Done.
		if err := t.cfg.Fn(ctx); err != nil && ctx.Err() == nil {
			t.cfg.Log.Warn("периодическая задача: ошибка итерации", "task", t.cfg.Name, "err", err)
		}
		timer.Reset(t.interval())
	}
}

// interval — Every со случайным сдвигом в пределах ±Jitter.
func (t *Task) interval() time.Duration {
	shift := t.cfg.Jitter * (2*t.rnd.Float64() - 1) // [-Jitter, +Jitter)
	return time.Duration(float64(t.cfg.Every) * (1 + shift))
}

// #endregion periodic-run
