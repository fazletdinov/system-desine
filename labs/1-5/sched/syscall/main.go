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
	// Прямой системный вызов: горутина держит свой поток ОС (M) до возврата из ядра,
	// а sysmon отбирает у него процессор (P), чтобы остальной Go-код продолжал работать.
	ts := syscall.NsecToTimespec(blockFor.Nanoseconds())
	_ = syscall.Nanosleep(&ts, nil)
}

func main() {
	labenv.Print(os.Stdout)

	// time.Sleep идёт первым: пул потоков ещё не раздут блокирующими вызовами,
	// и «+0» — свойство time.Sleep, а не порядка запуска.
	before, during := measure(func() { time.Sleep(blockFor) })
	fmt.Printf("time.Sleep        x%d: потоков до %d, во время %d (+%d)\n", blockers, before, during, during-before)

	before, during = measure(nanosleep)
	fmt.Printf("syscall.Nanosleep x%d: потоков до %d, во время %d (+%d)\n", blockers, before, during, during-before)
}

// #endregion syscall
