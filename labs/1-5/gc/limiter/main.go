// Программа limiter показывает ограничитель CPU сборщика мусора (урок 1.5.9). Живая куча растёт
// окно за окном и переходит за GOMEMLIMIT, который задаётся снаружи; горутины на всех P тем временем
// производят мусор. Выше лимита сборщик работал бы без перерыва и отнимал бы у программы почти весь
// процессор (спираль смерти), но ограничитель держит его долю около 50 %:
//
//	GOMEMLIMIT=64MiB go run ./1-5/gc/limiter
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"runtime/metrics"
	"sync"
	"time"

	"labs/1-5/labenv"
)

const (
	metricCycles      = "/gc/cycles/total:gc-cycles"
	metricGCCPU       = "/cpu/classes/gc/total:cpu-seconds"
	metricTotalCPU    = "/cpu/classes/total:cpu-seconds"
	metricLiveHeap    = "/gc/heap/live:bytes"
	metricGoal        = "/gc/heap/goal:bytes"
	metricLimit       = "/gc/gomemlimit:bytes"
	metricLastLimited = "/gc/limiter/last-enabled:gc-cycle"

	// window — окно усреднения доли CPU. Ёмкость ведра ограничителя — секунда процессорного
	// времени на P: окна намного короче показали бы пилу, намного длиннее — смазали бы момент включения.
	window = 500 * time.Millisecond
	// garbageObj — мусорный объект без указателей: аллокация дешёвая, а долг перед сборщиком
	// (mark assist) начисляется за каждый выделенный байт.
	garbageObj = 1024
)

// node — 64 байта из одних указателей: разметке приходится проверять каждое слово, так что
// стоимость цикла растёт вместе с живой кучей.
type node struct {
	refs [8]*node
}

type sample struct {
	cycles, live, goal, lastLimited uint64
	gcCPU, total                    float64
}

func read() sample {
	s := []metrics.Sample{
		{Name: metricCycles}, {Name: metricGCCPU}, {Name: metricTotalCPU},
		{Name: metricLiveHeap}, {Name: metricGoal}, {Name: metricLastLimited},
	}
	metrics.Read(s)
	return sample{
		cycles:      s[0].Value.Uint64(),
		gcCPU:       s[1].Value.Float64(),
		total:       s[2].Value.Float64(),
		live:        s[3].Value.Uint64(),
		goal:        s[4].Value.Uint64(),
		lastLimited: s[5].Value.Uint64(),
	}
}

func mib(b uint64) uint64 { return b >> 20 }

// #region limiter

// produceGarbage занимает все P: каждая горутина без остановки выделяет короткоживущие объекты.
// Выше лимита памяти каждая такая аллокация тянет за собой mark assist — работу сборщика за счёт
// программы, — и именно её ограничитель перестаёт требовать, когда доля GC превышает предел.
func produceGarbage(stop <-chan struct{}) *sync.WaitGroup {
	var wg sync.WaitGroup
	sinks := make([][]byte, runtime.GOMAXPROCS(0)) // у каждой горутины своя ячейка: без гонки
	for i := range sinks {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				garbage := make([]byte, garbageObj)
				garbage[0] = 1
				sinks[i] = garbage // через итерацию — мусор
			}
		})
	}
	return &wg
}

// #endregion limiter

func main() {
	step := flag.Int("step", 32, "на сколько МиБ живая куча растёт за окно")
	windows := flag.Int("windows", 8, "сколько окон по 500 мс")
	flag.Parse()
	labenv.Print(os.Stdout)

	lim := []metrics.Sample{{Name: metricLimit}}
	metrics.Read(lim)
	if lim[0].Value.Uint64() == math.MaxInt64 {
		fmt.Fprintln(os.Stderr, "limiter: GOMEMLIMIT не задан — программа показывает ограничитель CPU сборщика в его главном сценарии, когда живая куча выше лимита памяти (сам ограничитель включён всегда); запустите, например, с GOMEMLIMIT=64MiB")
		os.Exit(2)
	}
	fmt.Printf("GOMEMLIMIT: %d МиБ, живая куча растёт на %d МиБ за окно %s, горутин-источников мусора: %d\n",
		mib(lim[0].Value.Uint64()), *step, window, runtime.GOMAXPROCS(0))

	stop := make(chan struct{})
	workers := produceGarbage(stop)
	var live [][]*node
	nodesPerStep := (*step << 20) / 64
	prev := read()
	maxShare := 0.0
	for w := 1; w <= *windows; w++ {
		windowStart := time.Now()
		chunk := make([]*node, nodesPerStep)
		for i := range chunk {
			n := &node{}
			for j := range n.refs { // ссылки на соседей: разметка идёт по графу, а не по массиву
				if i > j {
					n.refs[j] = chunk[i-j-1]
				}
			}
			chunk[i] = n
		}
		live = append(live, chunk)
		time.Sleep(window - time.Since(windowStart))
		cur := read()
		share := 100 * (cur.gcCPU - prev.gcCPU) / (cur.total - prev.total)
		maxShare = max(maxShare, share)
		fmt.Printf("окно %2d: живая куча %4d МиБ, цель %4d МиБ, циклов GC +%3d, CPU на GC %5.1f %%, ограничитель последний раз — на цикле %d\n",
			w, mib(cur.live), mib(cur.goal), cur.cycles-prev.cycles, share, cur.lastLimited)
		prev = cur
	}
	close(stop)
	workers.Wait()
	runtime.KeepAlive(live)
	fmt.Printf("максимальная доля CPU на GC за окно: %.1f %%\n", maxShare)
	// Рантайм записывает номер цикла, который шёл в момент включения: он может быть на единицу
	// больше числа завершённых.
	fmt.Printf("ограничитель включался последний раз на цикле: %d (завершено циклов: %d)\n", prev.lastLimited, prev.cycles)
}
