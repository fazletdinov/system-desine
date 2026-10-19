// Программа limit запускает 200 задач по 20 мс двумя способами — горутина на задачу
// и семафор на канале на 8 — и печатает время и пик числа горутин.
// errgroup.SetLimit в labs не показан: golang.org/x/sync тут не подключён, он в мессенджере (урок 1.5.6).
package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"labs/1-5/labenv"
)

const (
	taskCount      = 200
	taskDuration   = 20 * time.Millisecond
	concurrency    = 8
	sampleInterval = time.Millisecond
)

func task() { time.Sleep(taskDuration) }

// #region limit
// perTask — горутина на задачу: пик равен числу задач.
func perTask() {
	var wg sync.WaitGroup
	for range taskCount {
		wg.Go(task)
	}
	wg.Wait()
}

// semaphore — буферизованный канал как семафор. Место занимаем ДО запуска горутины:
// если занимать внутри, все 200 горутин уже созданы и ограничено только выполнение,
// а не число горутин.
func semaphore() {
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
	measure("семафор на канале:", semaphore)
}
