// Пакет synclab — бенчмарки лабы 1.5: цена Mutex, RWMutex и atomic под нагрузкой,
// длинная секция чтения, CAS-цикл, шардированный счётчик и ложное разделение строки кэша (false sharing).
package synclab

import (
	"runtime"
	"strconv"
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

// Пара для RWMutex: те же чтения под обычным Mutex.
func BenchmarkCounterMutexRead(b *testing.B) {
	var mu sync.Mutex
	var n int
	b.RunParallel(func(pb *testing.PB) {
		sum := 0
		for pb.Next() {
			mu.Lock()
			sum += n
			mu.Unlock()
		}
		_ = sum
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

// #region longread
// Длинная секция чтения: читатель под замком обходит срез целиком. Сравнимая пара —
// LongReadMutex и LongReadRWMutex с одинаковой работой; размер среза задаёт длину секции.
var longReadSizes = []int{256, 4096}

func sumInts(xs []int) int {
	s := 0
	for _, v := range xs {
		s += v
	}
	return s
}

func BenchmarkLongReadMutex(b *testing.B) {
	for _, n := range longReadSizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			var mu sync.Mutex
			data := make([]int, n)
			b.RunParallel(func(pb *testing.PB) {
				s := 0
				for pb.Next() {
					mu.Lock()
					s += sumInts(data)
					mu.Unlock()
				}
				_ = s
			})
		})
	}
}

func BenchmarkLongReadRWMutex(b *testing.B) {
	for _, n := range longReadSizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			var mu sync.RWMutex
			data := make([]int, n)
			b.RunParallel(func(pb *testing.PB) {
				s := 0
				for pb.Next() {
					mu.RLock()
					s += sumInts(data)
					mu.RUnlock()
				}
				_ = s
			})
		})
	}
}

// #endregion longread

// #region casloop
// addCAS — приращение CAS-циклом: прочитать, вычислить, попробовать заменить; если ячейку
// успели изменить, повторить. Возвращает число неудачных попыток.
func addCAS(n *atomic.Int64, delta int64) (retries int) {
	for {
		old := n.Load()
		if n.CompareAndSwap(old, old+delta) {
			return retries
		}
		retries++
	}
}

// Пара для CounterAtomic: тот же счётчик, но неблокирующий алгоритм вместо одной инструкции Add.
func BenchmarkCounterCASLoop(b *testing.B) {
	var n, retries atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		r := 0
		for pb.Next() {
			r += addCAS(&n, 1)
		}
		retries.Add(int64(r))
	})
	b.ReportMetric(float64(retries.Load())/float64(b.N), "retries/op")
}

// #endregion casloop

// #region sharded
// slot — ячейка шардированного счётчика: паддинг уводит соседнюю ячейку в другую строку кэша.
type slot struct {
	n atomic.Int64
	_ [cacheLine - 8]byte
}

// Пара для CounterAtomic: у каждой горутины RunParallel своя ячейка, общей строки кэша нет.
// Сумма — обход ячеек; её здесь не считаем, меряем только запись.
func BenchmarkShardedCounter(b *testing.B) {
	slots := make([]slot, runtime.GOMAXPROCS(0)) // RunParallel запускает по горутине на P
	var next atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		mine := &slots[int(next.Add(1)-1)%len(slots)]
		for pb.Next() {
			mine.n.Add(1)
		}
	})
}

// #endregion sharded
