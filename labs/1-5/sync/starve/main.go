// Программа starve показывает режим голодания sync.Mutex: одна горутина держит замок
// подолгу и сразу берёт его снова, вторая берёт его на миг. В нормальном режиме вторая
// может проигрывать борьбу за замок раз за разом; режим голодания ограничивает ожидание.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"labs/1-5/labenv"
)

const (
	runFor        = time.Second
	holdTime      = 2 * time.Millisecond // «долго» с точки зрения порога голодания
	starvationGap = time.Millisecond     // порог из sync.Mutex: ждёшь дольше — замок передают тебе напрямую
)

// waitBuckets — верхние границы корзин гистограммы ожиданий. 1–3 мс — «одно удержание»
// (holdTime), 3–5 мс — «два удержания»: так видно, сколько раундов проиграла быстрая горутина.
var waitBuckets = []time.Duration{100 * time.Microsecond, time.Millisecond, 3 * time.Millisecond, 5 * time.Millisecond}

var bucketNames = []string{"<0,1 мс", "0,1–1 мс", "1–3 мс", "3–5 мс", "≥5 мс"}

type stats struct {
	acquisitions int
	maxWait      time.Duration
	longWaits    int   // ожидания дольше starvationGap
	hist         []int // по корзине на waitBuckets и одна сверх последней границы
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
// #region histogram
func (s *stats) record(wait time.Duration) {
	s.acquisitions++
	s.maxWait = max(s.maxWait, wait)
	if wait > starvationGap {
		s.longWaits++
	}
	if s.hist == nil {
		s.hist = make([]int, len(waitBuckets)+1)
	}
	i := 0
	for i < len(waitBuckets) && wait >= waitBuckets[i] {
		i++
	}
	s.hist[i]++
}

// #endregion histogram

func (s *stats) histogram() string {
	parts := make([]string, len(bucketNames))
	for i, name := range bucketNames {
		n := 0
		if s.hist != nil {
			n = s.hist[i]
		}
		parts[i] = fmt.Sprintf("%s: %d", name, n)
	}
	return strings.Join(parts, ", ")
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
	fmt.Printf("ожидания быстрой горутины: %s\n", light.histogram())
}
