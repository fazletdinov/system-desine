// Пакет synclab — бенчмарки лабы 1.5: цена Mutex, RWMutex и atomic под нагрузкой
// и ложное разделение строки кэша (false sharing).
package synclab

import (
	"sync"
	"sync/atomic"
	"testing"
)

// #region counter
func BenchmarkCounterMutex(b *testing.B) {
	var mu sync.Mutex
	var n int
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			n++
			mu.Unlock()
		}
	})
}

// Читатели берут RLock: RWMutex оправдан, когда читают намного чаще, чем пишут,
// и критическая секция длиннее самого RLock/RUnlock.
func BenchmarkCounterRWMutexRead(b *testing.B) {
	var mu sync.RWMutex
	var n int
	b.RunParallel(func(pb *testing.PB) {
		sum := 0
		for pb.Next() {
			mu.RLock()
			sum += n
			mu.RUnlock()
		}
		_ = sum
	})
}

func BenchmarkCounterAtomic(b *testing.B) {
	var n atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n.Add(1)
		}
	})
}

// #endregion counter

// #region padding
// cacheLine — размер строки кэша на x86-64 и большинстве arm64. Паддинг считаем вручную:
// golang.org/x/sys/cpu.CacheLinePad в labs не подключён, а зависимость ради одной константы лишняя.
const cacheLine = 64

// packed: оба счётчика лежат в одной строке кэша.
type packed struct {
	a atomic.Int64
	b atomic.Int64
}

// padded: между счётчиками — остаток строки кэша, каждый живёт в своей.
type padded struct {
	a atomic.Int64
	_ [cacheLine - 8]byte
	b atomic.Int64
}

// hammerTwo запускает две горутины: одна бьёт только по a, другая только по b.
// Данные не общие, но строка кэша — общая, если счётчики лежат рядом.
func hammerTwo(b *testing.B, incA, incB func()) {
	var wg sync.WaitGroup
	b.ResetTimer()
	wg.Go(func() {
		for range b.N {
			incA()
		}
	})
	wg.Go(func() {
		for range b.N {
			incB()
		}
	})
	wg.Wait()
}

func BenchmarkFalseSharingPacked(b *testing.B) {
	var c packed
	hammerTwo(b, func() { c.a.Add(1) }, func() { c.b.Add(1) })
}

func BenchmarkFalseSharingPadded(b *testing.B) {
	var c padded
	hammerTwo(b, func() { c.a.Add(1) }, func() { c.b.Add(1) })
}

// #endregion padding
