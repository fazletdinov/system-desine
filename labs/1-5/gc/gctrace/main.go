// Программа gctrace держит живую кучу заданного размера и с заданной скоростью производит мусор, а
// затем печатает, сколько циклов сборки это стоило (урок 1.5.9). Запускается под разными GOGC и
// GOMEMLIMIT и с GODEBUG=gctrace=1 — тогда рантайм сам пишет строку на каждый цикл в stderr:
//
//	GODEBUG=gctrace=1 GOGC=50 go run ./1-5/gc/gctrace -live 64MiB -rate 1GiB -dur 3s
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"runtime/metrics"
	"strconv"
	"strings"
	"time"

	"labs/1-5/labenv"
)

const (
	metricCycles   = "/gc/cycles/total:gc-cycles"
	metricGCCPU    = "/cpu/classes/gc/total:cpu-seconds"
	metricTotalCPU = "/cpu/classes/total:cpu-seconds"
	metricIdleCPU  = "/cpu/classes/idle:cpu-seconds"
	metricGoal     = "/gc/heap/goal:bytes"
	metricPauses   = "/gc/pauses:seconds"
	metricLimit    = "/gc/gomemlimit:bytes"

	// garbageChunk — размер одного «мусорного» объекта: как буфер запроса средней руки.
	garbageChunk = 8 << 10
	// tick — шаг, с которым программа догоняет заданную скорость и снимает цель кучи.
	tick = time.Millisecond
)

// node — объект живой кучи: 64 байта с указателем, чтобы разметке было что обходить.
type node struct {
	next *node
	_    [56]byte
}

// #region gctrace

// churn производит мусор со скоростью rate байт/с в течение dur и возвращает пик цели кучи.
// Живую кучу держит вызывающий; каждый новый garbageChunk тут же становится мусором.
func churn(rate float64, dur time.Duration) (allocated, peakGoal uint64) {
	goal := []metrics.Sample{{Name: metricGoal}}
	// Кольцо держит последние куски: значение, переживающее итерацию цикла, компилятор обязан
	// выделить в куче, а не на стеке.
	var ring [16][]byte
	start := time.Now()
	for el := time.Duration(0); el < dur; el = time.Since(start) {
		for float64(allocated) < rate*el.Seconds() {
			ring[allocated/garbageChunk%uint64(len(ring))] = make([]byte, garbageChunk)
			allocated += garbageChunk
		}
		metrics.Read(goal)
		peakGoal = max(peakGoal, goal[0].Value.Uint64())
		time.Sleep(tick)
	}
	runtime.KeepAlive(&ring)
	return allocated, peakGoal
}

// #endregion gctrace

type gcSnapshot struct {
	cycles             uint64
	gcCPU, total, idle float64
}

func readGC() gcSnapshot {
	s := []metrics.Sample{{Name: metricCycles}, {Name: metricGCCPU}, {Name: metricTotalCPU}, {Name: metricIdleCPU}}
	metrics.Read(s)
	return gcSnapshot{
		cycles: s[0].Value.Uint64(),
		gcCPU:  s[1].Value.Float64(),
		total:  s[2].Value.Float64(),
		idle:   s[3].Value.Float64(),
	}
}

// maxPause — верхняя граница самой старшей непустой корзины гистограммы пауз «остановки мира».
func maxPause() time.Duration {
	s := []metrics.Sample{{Name: metricPauses}}
	metrics.Read(s)
	h := s[0].Value.Float64Histogram()
	for i := len(h.Counts) - 1; i >= 0; i-- {
		if h.Counts[i] > 0 {
			return time.Duration(h.Buckets[i+1] * float64(time.Second))
		}
	}
	return 0
}

// parseBytes понимает 64MiB, 1GiB, 512KiB и просто байты.
func parseBytes(s string) (uint64, error) {
	units := []struct {
		suffix string
		mult   uint64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}}
	mult := uint64(1)
	for _, u := range units {
		if rest, ok := strings.CutSuffix(s, u.suffix); ok {
			s, mult = rest, u.mult
			break
		}
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("размер %q: нужно число с KiB, MiB или GiB: %w", s, err)
	}
	return n * mult, nil
}

func bytesFlag(name string, def uint64, usage string) *uint64 {
	v := def
	flag.Func(name, usage, func(s string) error {
		n, err := parseBytes(s)
		v = n
		return err
	})
	return &v
}

func mib(b uint64) float64 { return float64(b) / (1 << 20) }

func main() {
	live := bytesFlag("live", 64<<20, "живая куча (KiB, MiB, GiB), по умолчанию 64MiB")
	rate := bytesFlag("rate", 1<<30, "скорость мусора в секунду (KiB, MiB, GiB), по умолчанию 1GiB")
	dur := flag.Duration("dur", 3*time.Second, "длительность")
	flag.Parse()
	labenv.Print(os.Stdout)

	nodes := make([]*node, *live/uint64(64))
	for i := range nodes {
		nodes[i] = &node{}
		if i > 0 {
			nodes[i].next = nodes[i-1]
		}
	}
	runtime.GC()

	gogc := os.Getenv("GOGC")
	if gogc == "" {
		gogc = "100 (по умолчанию)"
	}
	limit := "не задан"
	if s := []metrics.Sample{{Name: metricLimit}}; true {
		metrics.Read(s)
		if v := s[0].Value.Uint64(); v != math.MaxInt64 {
			limit = fmt.Sprintf("%.0f МиБ", mib(v))
		}
	}
	fmt.Printf("живая куча: %.0f МиБ (%d объектов по 64 Б), мусор: %.0f МиБ/с, длительность %s\n", mib(*live), len(nodes), mib(*rate), *dur)
	fmt.Printf("GOGC=%s, GOMEMLIMIT: %s\n", gogc, limit)

	before := readGC()
	start := time.Now()
	allocated, peakGoal := churn(float64(*rate), *dur)
	elapsed := time.Since(start)
	after := readGC()
	runtime.KeepAlive(nodes)

	gcCPU := after.gcCPU - before.gcCPU
	total := after.total - before.total
	busy := total - (after.idle - before.idle)
	fmt.Printf("выделено: %.0f МиБ (%.0f МиБ/с)\n", mib(allocated), mib(allocated)/elapsed.Seconds())
	fmt.Printf("циклов GC: %d\n", after.cycles-before.cycles)
	fmt.Printf("доля CPU на GC: %.1f %% всего CPU (GOMAXPROCS × время), %.1f %% занятого\n", 100*gcCPU/total, 100*gcCPU/busy)
	fmt.Printf("пик цели кучи: %.0f МиБ\n", mib(peakGoal))
	fmt.Printf("самая долгая пауза: до %.0f мкс\n", float64(maxPause())/float64(time.Microsecond))
}
