// Программа starve показывает режим голодания sync.Mutex: одна горутина держит замок
// подолгу и сразу берёт его снова, вторая берёт его на миг. В нормальном режиме вторая
// может проигрывать борьбу за замок раз за разом; режим голодания ограничивает ожидание.
package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"labs/1-5/labenv"
)

const (
	runFor        = time.Second
	holdTime      = 2 * time.Millisecond // «долго» с точки зрения порога голодания
	starvationGap = time.Millisecond     // порог из sync.Mutex: ждёшь дольше — замок передают тебе напрямую
)

type stats struct {
	acquisitions int
	maxWait      time.Duration
	longWaits    int // ожидания дольше starvationGap
}

// #region starve
func holder(ctx context.Context, mu *sync.Mutex, st *stats) {
	for ctx.Err() == nil {
		t0 := time.Now()
		mu.Lock()
		st.record(time.Since(t0))
		time.Sleep(holdTime) // держим замок долго
		mu.Unlock()
	}
}

func quick(ctx context.Context, mu *sync.Mutex, st *stats) {
	for ctx.Err() == nil {
		t0 := time.Now()
		mu.Lock()
		st.record(time.Since(t0))
		mu.Unlock()
	}
}

// #endregion starve

// record вызывается под замком mu и пишет в stats только горутина-владелец,
// поэтому собственной синхронизации stats не нужно.
func (s *stats) record(wait time.Duration) {
	s.acquisitions++
	s.maxWait = max(s.maxWait, wait)
	if wait > starvationGap {
		s.longWaits++
	}
}

func milliseconds(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func main() {
	labenv.Print(os.Stdout)

	ctx, cancel := context.WithTimeout(context.Background(), runFor)
	defer cancel()

	var mu sync.Mutex
	var heavy, light stats
	var wg sync.WaitGroup
	wg.Go(func() { holder(ctx, &mu, &heavy) })
	wg.Go(func() { quick(ctx, &mu, &light) })
	wg.Wait()

	fmt.Printf("держатель (держит %v): захватов %d, макс. ожидание %.2f мс\n", holdTime, heavy.acquisitions, milliseconds(heavy.maxWait))
	fmt.Printf("быстрая горутина:       захватов %d, макс. ожидание %.2f мс, ожиданий дольше %v: %d\n",
		light.acquisitions, milliseconds(light.maxWait), starvationGap, light.longWaits)
}
