// Программа syscall показывает, что горутина в блокирующем системном вызове занимает
// поток ОС, а горутина в time.Sleep — нет.
package main

import (
	"fmt"
	"os"
	"runtime/metrics"
	"sync"
	"syscall"
	"time"

	"labs/1-5/labenv"
)

const (
	blockers    = 8
	blockFor    = 200 * time.Millisecond
	sampleAfter = 50 * time.Millisecond // середина блокировки: все горутины уже внутри вызова
	threadsName = "/sched/threads/total:threads"
)

func threads() uint64 {
	s := []metrics.Sample{{Name: threadsName}}
	metrics.Read(s)
	return s[0].Value.Uint64()
}

// measure запускает blockers горутин с телом block и возвращает число потоков до и во время.
func measure(block func()) (before, during uint64) {
	before = threads()
	var wg sync.WaitGroup
	for range blockers {
		wg.Go(block)
	}
	time.Sleep(sampleAfter)
	during = threads()
	wg.Wait()
	return before, during
}

// #region syscall
func nanosleep() {
	// Прямой системный вызов: рантайм не знает, что он «просто ждёт», и держит поток занятым.
	ts := syscall.NsecToTimespec(blockFor.Nanoseconds())
	_ = syscall.Nanosleep(&ts, nil)
}

func main() {
	labenv.Print(os.Stdout)

	before, during := measure(nanosleep)
	fmt.Printf("syscall.Nanosleep x%d: потоков до %d, во время %d (+%d)\n", blockers, before, during, during-before)

	before, during = measure(func() { time.Sleep(blockFor) })
	fmt.Printf("time.Sleep        x%d: потоков до %d, во время %d (+%d)\n", blockers, before, during, during-before)
}

// #endregion syscall
