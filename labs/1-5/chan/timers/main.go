// Программа timers сравнивает select с новым time.After на каждой итерации и select
// с одним переиспользуемым time.Timer: память кучи и число циклов GC.
package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"labs/1-5/labenv"
)

const (
	iterations = 1_000_000
	timeout    = time.Second
)

type heapSnapshot struct {
	heapAlloc  uint64
	totalAlloc uint64
	numGC      uint32
}

func snapshot() heapSnapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return heapSnapshot{m.HeapAlloc, m.TotalAlloc, m.NumGC}
}

func mib(b uint64) float64 { return float64(b) / (1 << 20) }

// fastChannel отдаёт iterations значений и закрывается: select всегда успевает раньше таймера.
func fastChannel() <-chan int {
	ch := make(chan int, 1024)
	go func() {
		defer close(ch)
		for i := range iterations {
			ch <- i
		}
	}()
	return ch
}

// #region timers
// Как менялось поведение таймеров:
//   - до Go 1.23 таймер из time.After, не сработавший к концу итерации, оставался
//     в куче рантайма и ссылался на канал, пока не пройдёт timeout: миллион итераций
//     держали миллион таймеров, и сборщик мусора их не трогал;
//   - с Go 1.23 такие таймеры и каналы доступны сборщику сразу, как только на них
//     никто не ссылается, поэтому time.After в цикле перестал течь;
//   - в Go 1.27 убран GODEBUG asynctimerchan: канал таймера всегда синхронный
//     (без буфера, Reset и Stop не оставляют в нём устаревшее значение).
//
// Одно значение Timer с Reset всё равно дешевле: ни одной аллокации на итерацию.

func afterEachIteration() {
	fast := fastChannel()
	for {
		select {
		case _, ok := <-fast:
			if !ok {
				return
			}
		case <-time.After(timeout): // новый таймер и новый канал на каждой итерации
			panic("быстрый канал молчал целую секунду")
		}
	}
}

func reusedTimer() {
	fast := fastChannel()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		timer.Reset(timeout)
		select {
		case _, ok := <-fast:
			if !ok {
				return
			}
		case <-timer.C:
			panic("быстрый канал молчал целую секунду")
		}
	}
}

// #endregion timers

func report(name string, run func()) {
	runtime.GC()
	before := snapshot()
	run()
	after := snapshot()
	fmt.Printf("%-22s HeapAlloc до %6.2f МиБ, после %6.2f МиБ, выделено за прогон %7.1f МиБ, циклов GC %d\n",
		name, mib(before.heapAlloc), mib(after.heapAlloc), mib(after.totalAlloc-before.totalAlloc), after.numGC-before.numGC)
}

func main() {
	labenv.Print(os.Stdout)
	fmt.Printf("итераций: %d\n", iterations)
	report("time.After в цикле:", afterEachIteration)
	report("один Timer + Reset:", reusedTimer)
}
