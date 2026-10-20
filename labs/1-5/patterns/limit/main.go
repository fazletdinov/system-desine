// Программа limit запускает 200 задач по 20 мс пятью способами — горутина на задачу,
// семафор на канале, пул воркеров, errgroup.SetLimit и взвешенный семафор x/sync, все на 8 —
// и печатает время и пик числа горутин. Последний прогон — errgroup, где одна задача падает.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"labs/1-5/labenv"
)

const (
	taskCount      = 200
	taskDuration   = 20 * time.Millisecond
	concurrency    = 8
	sampleInterval = time.Millisecond
	// failingTask — номер задачи, которая падает в прогоне firstError.
	failingTask = 50
)

func task() { time.Sleep(taskDuration) }

// taskCtx — та же задача, но умеет прерываться: ждёт таймер или отмену ctx.
func taskCtx(ctx context.Context) error {
	t := time.NewTimer(taskDuration)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// #region limit
// perTask — горутина на задачу: пик равен числу задач.
func perTask() {
	var wg sync.WaitGroup
	for range taskCount {
		wg.Go(task)
	}
	wg.Wait()
}

// channelSemaphore — буферизованный канал как семафор. Место занимаем ДО запуска горутины:
// если занимать внутри, все 200 горутин уже созданы и ограничено только выполнение,
// а не число горутин.
func channelSemaphore() {
	slots := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for range taskCount {
		slots <- struct{}{} // блокирует цикл, пока не освободится место
		wg.Go(func() {
			defer func() { <-slots }()
			task()
		})
	}
	wg.Wait()
}

// #endregion limit

// #region pool
// pool — пул воркеров: concurrency долгоживущих горутин читают задачи из общего канала.
// Производитель (здесь main) блокируется на отправке, когда все воркеры заняты.
func pool() {
	jobs := make(chan func())
	var wg sync.WaitGroup
	for range concurrency {
		wg.Go(func() {
			for job := range jobs { // воркер выходит, когда jobs закрыт и пуст
				job()
			}
		})
	}
	for range taskCount {
		jobs <- task
	}
	close(jobs) // закрывает производитель — единственный писатель
	wg.Wait()
}

// #endregion pool

// #region errgroup
// errgroupLimit — то же через errgroup: SetLimit делает внутри ровно наш семафор на канале,
// а Go блокирует цикл, пока в группе concurrency активных горутин.
func errgroupLimit() {
	var g errgroup.Group
	g.SetLimit(concurrency)
	for range taskCount {
		g.Go(func() error {
			task()
			return nil
		})
	}
	_ = g.Wait() // задачи здесь не возвращают ошибок
}

// #endregion errgroup

// #region weighted
// weighted — семафор x/sync: Acquire ждёт с ctx и умеет «вес» (здесь у всех задач вес 1).
func weighted() {
	ctx := context.Background()
	sem := semaphore.NewWeighted(concurrency)
	var wg sync.WaitGroup
	for range taskCount {
		if err := sem.Acquire(ctx, 1); err != nil {
			break // только при отмене ctx; у Background её не бывает
		}
		wg.Go(func() {
			defer sem.Release(1)
			task()
		})
	}
	wg.Wait()
}

// #endregion weighted

// #region first-error
// firstError — errgroup.WithContext: задача failingTask падает, ctx группы отменяется,
// задачи в работе выходят по ctx, а цикл перестаёт запускать новые.
func firstError() (completed int64, err error) {
	var done atomic.Int64
	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(concurrency)
	for i := range taskCount {
		if ctx.Err() != nil {
			break // Go не смотрит на ctx: проверяем сами, иначе запустим все 200
		}
		g.Go(func() error {
			if i == failingTask {
				return fmt.Errorf("задача %d: %w", i, errTask)
			}
			if err := taskCtx(ctx); err != nil {
				return err
			}
			done.Add(1)
			return nil
		})
	}
	err = g.Wait() // первая ошибка, а не context.Canceled от остальных
	return done.Load(), err
}

// #endregion first-error

var errTask = errors.New("отказ")

// peakGoroutines опрашивает runtime.NumGoroutine, пока не отменён ctx, и возвращает максимум в stop().
func peakGoroutines(ctx context.Context) (stop func() int) {
	peak := make(chan int, 1)
	go func() {
		highest := 0
		ticker := time.NewTicker(sampleInterval)
		defer ticker.Stop()
		for {
			highest = max(highest, runtime.NumGoroutine())
			select {
			case <-ctx.Done():
				peak <- highest
				return
			case <-ticker.C:
			}
		}
	}()
	return func() int { return <-peak }
}

func measure(name string, run func()) {
	base := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	stop := peakGoroutines(ctx)
	t0 := time.Now()
	run()
	elapsed := time.Since(t0)
	cancel()
	// main и наблюдатель входят в пик, но задачами не являются: base — это main, +1 — наблюдатель.
	peak := stop()
	fmt.Printf("%-20s время %4d мс, пик горутин %3d (из них свои: %3d)\n", name, elapsed.Milliseconds(), peak, peak-base-1)
}

func main() {
	labenv.Print(os.Stdout)
	fmt.Printf("задач: %d по %v, лимит: %d\n", taskCount, taskDuration, concurrency)
	measure("горутина на задачу:", perTask)
	measure("семафор на канале:", channelSemaphore)
	measure("пул воркеров:", pool)
	measure("errgroup.SetLimit:", errgroupLimit)
	measure("semaphore.Weighted:", weighted)

	var completed int64
	var err error
	measure("errgroup, отказ:", func() { completed, err = firstError() })
	fmt.Printf("errgroup, отказ:     выполнено задач %d из %d, Wait вернул %q\n", completed, taskCount, err)
}
