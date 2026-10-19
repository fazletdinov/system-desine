// Package pipeline — конвейер из стадий на каналах: генератор, стадия, раздача воркерам
// (fan-out) со слиянием (fan-in) и параллельная обработка с сохранением порядка.
// Каждая стадия владеет своим выходным каналом, закрывает его сама и выходит по ctx.
package pipeline

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// #region generator
// Generate отдаёт items в канал и закрывает его. Если потребитель ушёл (ctx отменён),
// горутина не висит на отправке, а выходит.
func Generate[T any](ctx context.Context, items ...T) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for _, it := range items {
			select {
			case out <- it:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// #endregion generator

// #region stage
// Map — одна стадия: читает in до закрытия, применяет f и пишет в свой out.
func Map[In, Out any](ctx context.Context, in <-chan In, f func(In) Out) <-chan Out {
	out := make(chan Out)
	go func() {
		defer close(out) // вход кончился или ctx отменён: выход закрывает его владелец
		forward(ctx, in, out, f)
	}()
	return out
}

// forward — тело стадии. Обе блокирующие операции, и чтение, и запись, стоят в select
// рядом с ctx.Done(): иначе горутина зависнет, если сосед по конвейеру ушёл.
func forward[In, Out any](ctx context.Context, in <-chan In, out chan<- Out, f func(In) Out) {
	for {
		var v In
		select {
		case x, ok := <-in:
			if !ok {
				return
			}
			v = x
		case <-ctx.Done():
			return
		}
		r := f(v)
		select {
		case out <- r:
		case <-ctx.Done():
			return
		}
	}
}

// #endregion stage

// #region fanout
// ParallelMap раздаёт элементы in воркерам (fan-out) и сливает их результаты в один
// канал (fan-in). Порядок результатов не сохраняется: кто раньше закончил, тот раньше и отдал.
func ParallelMap[In, Out any](ctx context.Context, in <-chan In, workers int, f func(In) Out) <-chan Out {
	out := make(chan Out)
	var wg sync.WaitGroup
	for range workers {
		// Все воркеры читают один in (канал сам раздаёт значения по одному) и пишут в один out.
		wg.Go(func() { forward(ctx, in, out, f) })
	}
	go func() {
		wg.Wait() // out закрывает не воркер, а тот, кто дождался всех воркеров
		close(out)
	}()
	return out
}

// #endregion fanout

// #region ordered
// OrderedMap обрабатывает items не более чем в limit горутинах и возвращает результаты
// в исходном порядке: каждая задача пишет в свою ячейку results[i], гонки нет.
// Первая ошибка отменяет ctx остальных задач, и OrderedMap возвращает именно её.
func OrderedMap[In, Out any](ctx context.Context, items []In, limit int, f func(context.Context, In) (Out, error)) ([]Out, error) {
	results := make([]Out, len(items))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)
	for i, it := range items {
		if ctx.Err() != nil {
			break // группа уже отменена: новые задачи не запускаем
		}
		g.Go(func() error { // блокирует, пока в группе limit активных горутин
			r, err := f(ctx, it)
			if err != nil {
				return err
			}
			results[i] = r
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

// #endregion ordered
