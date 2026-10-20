// Программа cost измеряет цену горутины и потока ОС: память и время создания.
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/metrics"
	"strconv"
	"strings"
	"time"

	"labs/1-5/labenv"
)

const (
	goroutineCount = 100_000
	threadCount    = 1_000
	stacksMetric   = "/memory/classes/heap/stacks:bytes"
)

// #region cost
func stackBytes() uint64 {
	s := []metrics.Sample{{Name: stacksMetric}}
	metrics.Read(s)
	return s[0].Value.Uint64()
}

// parkGoroutines запускает n горутин, ждущих на канале stop, и возвращает
// прирост стеков и время создания. Закрытие stop отпускает их всех.
func parkGoroutines(n int, stop <-chan struct{}, pinThread bool) (stackGrowth uint64, spawn time.Duration) {
	before := stackBytes()
	started := make(chan struct{}, n)
	t0 := time.Now()
	for range n {
		go func() {
			if pinThread {
				runtime.LockOSThread() // горутина держит поток: он не вернётся в пул
			}
			started <- struct{}{}
			<-stop
		}()
	}
	for range n {
		<-started
	}
	spawn = time.Since(t0)
	return stackBytes() - before, spawn
}

func vmRSSKiB() int {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "VmRSS:"); ok {
			kib, _ := strconv.Atoi(strings.Fields(rest)[0])
			return kib
		}
	}
	return 0
}

func main() {
	labenv.Print(os.Stdout)

	stop := make(chan struct{})
	growth, spawn := parkGoroutines(goroutineCount, stop, false)
	fmt.Printf("горутин: %d\n", goroutineCount)
	fmt.Printf("стек на горутину: %d байт\n", growth/goroutineCount)
	fmt.Printf("создание одной горутины: %d нс\n", spawn.Nanoseconds()/goroutineCount)
	close(stop)
	time.Sleep(100 * time.Millisecond) // дать горутинам выйти до замера потоков

	rssBefore := vmRSSKiB()
	stop = make(chan struct{})
	_, spawn = parkGoroutines(threadCount, stop, true)
	rssGrowth := vmRSSKiB() - rssBefore
	fmt.Printf("потоков ОС: %d\n", threadCount)
	fmt.Printf("рост RSS на поток: %d КиБ\n", rssGrowth/threadCount)
	fmt.Printf("создание одного потока: %d мкс\n", spawn.Microseconds()/threadCount)
	close(stop)
}

// #endregion cost
