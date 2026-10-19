package pipeline

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func square(x int) int { return x * x }

func seq(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// #region cancel
// Потребитель забрал три значения из тысячи, отменил ctx и ушёл, не дочитав out.
// synctest.Test сам проверяет утечки: если после возврата функции в пузыре осталась
// заблокированная горутина, тест падает с «main bubble goroutine has exited but blocked
// goroutines remain».
func TestPipelineStopsWhenConsumerLeaves(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		out := ParallelMap(ctx, Generate(ctx, seq(1000)...), 4, square)
		for range 3 {
			<-out
		}
		cancel()
		synctest.Wait() // все горутины конвейера вышли или заблокированы навсегда
		if _, ok := <-out; ok {
			t.Fatal("out не закрыт: воркеры не вышли по отмене")
		}
	})
}

// #endregion cancel

// #region synctest
// Восемь задач по секунде на четырёх воркерах — ровно две секунды виртуального времени.
// Настоящего ожидания нет: часы пузыря прыгают вперёд, когда все его горутины заблокированы.
func TestParallelMapVirtualTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		slow := func(x int) int {
			time.Sleep(time.Second)
			return square(x)
		}
		start := time.Now()
		var got []int
		for v := range ParallelMap(t.Context(), Generate(t.Context(), seq(8)...), 4, slow) {
			got = append(got, v)
		}
		if elapsed := time.Since(start); elapsed != 2*time.Second {
			t.Errorf("виртуальное время %v, ждали ровно 2s", elapsed)
		}
		slices.Sort(got) // fan-in не сохраняет порядок
		if want := []int{0, 1, 4, 9, 16, 25, 36, 49}; !slices.Equal(got, want) {
			t.Errorf("результаты %v, ждали %v", got, want)
		}
	})
}

// #endregion synctest

// Задача i спит (10-i) мс: заканчивают в обратном порядке, а результат всё равно по порядку.
func TestOrderedMapKeepsOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := func(_ context.Context, x int) (int, error) {
			time.Sleep(time.Duration(10-x) * time.Millisecond)
			return square(x), nil
		}
		got, err := OrderedMap(t.Context(), seq(10), 3, f)
		if err != nil {
			t.Fatalf("OrderedMap: %v", err)
		}
		if want := []int{0, 1, 4, 9, 16, 25, 36, 49, 64, 81}; !slices.Equal(got, want) {
			t.Errorf("результаты %v, ждали %v", got, want)
		}
	})
}

// #region first-error
// Задача 2 падает через секунду; остальные ждали бы час, но выходят по отменённому ctx.
func TestOrderedMapFirstErrorCancelsOthers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		errBoom := errors.New("задача 2 упала")
		var calls atomic.Int32
		f := func(ctx context.Context, x int) (int, error) {
			calls.Add(1)
			if x == 2 {
				time.Sleep(time.Second)
				return 0, errBoom
			}
			select {
			case <-time.After(time.Hour):
				return x, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
		start := time.Now()
		_, err := OrderedMap(t.Context(), seq(10), 4, f)
		if !errors.Is(err, errBoom) {
			t.Fatalf("ошибка %v, ждали первую: %v", err, errBoom)
		}
		if elapsed := time.Since(start); elapsed != time.Second {
			t.Errorf("виртуальное время %v, ждали 1s: остальные задачи должны выйти по отмене", elapsed)
		}
		// 0–3 запущены сразу, задача 4 уже ждала места внутри g.Go и запустилась после отмены
		// (Go не смотрит на ctx), а 5–9 отсёк break по ctx.Err().
		if n := calls.Load(); n != 5 {
			t.Errorf("f вызвана %d раз, ждали 5", n)
		}
	})
}

// #endregion first-error
