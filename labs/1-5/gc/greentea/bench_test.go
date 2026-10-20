// Пакет greentea — бенчмарк разметки кучи к уроку 1.5.9: Green Tea (по умолчанию с Go 1.26)
// против прежней разметки по объектам. Прежняя включается только флагом сборки, поэтому бенчмарк
// запускают дважды и сравнивают benchstat:
//
//	go test -run='^$' -bench=. -count=10 ./1-5/gc/greentea/ > greentea.txt
//	GOEXPERIMENT=nogreenteagc go test -run='^$' -bench=. -count=10 ./1-5/gc/greentea/ > classic.txt
//	benchstat classic.txt greentea.txt
package greentea

import (
	"math/rand/v2"
	"runtime"
	"runtime/metrics"
	"testing"
)

// heapBytes — размер живой кучи в каждом подтесте: одинаковый, чтобы сравнивались размеры объектов,
// а не объём.
const heapBytes = 64 << 20

const metricGCCPU = "/cpu/classes/gc/total:cpu-seconds"

// small — 32 байта, два указателя: типичный узел дерева или списка.
type small struct {
	left, right *small
	_           [2]int64
}

// medium — 256 байт, два указателя и данные: структура запроса или сущности.
type medium struct {
	left, right *medium
	_           [30]int64
}

// #region greentea

// link связывает объекты случайными указателями: соседние по адресу объекты указывают в разные
// места кучи, как в настоящем графе, и разметка по объектам прыгает по памяти. Green Tea копит
// найденные объекты по спанам и сканирует спан целиком, когда в нём набралось несколько.
func link[T any](nodes []*T, set func(n, l, r *T)) {
	rnd := rand.New(rand.NewPCG(1, 2)) // фиксированное зерно: одинаковый граф в обоих прогонах
	for _, n := range nodes {
		set(n, nodes[rnd.IntN(len(nodes))], nodes[rnd.IntN(len(nodes))])
	}
}

// benchMark держит граф живым и на каждой итерации запускает полный цикл GC. Кроме времени цикла
// печатает процессорное время сборщика на цикл: Green Tea экономит именно его.
func benchMark[T any](b *testing.B, size int, set func(n, l, r *T)) {
	nodes := make([]*T, heapBytes/size)
	for i := range nodes {
		nodes[i] = new(T)
	}
	link(nodes, set)
	runtime.GC()

	cpu := []metrics.Sample{{Name: metricGCCPU}}
	metrics.Read(cpu)
	before := cpu[0].Value.Float64()
	cycles := 0
	for b.Loop() {
		runtime.GC()
		cycles++
	}
	metrics.Read(cpu)
	b.ReportMetric((cpu[0].Value.Float64()-before)*1e3/float64(cycles), "gc-cpu-ms/op")
	runtime.KeepAlive(nodes)
}

func BenchmarkMark(b *testing.B) {
	b.Run("objects=32B", func(b *testing.B) {
		benchMark(b, 32, func(n, l, r *small) { n.left, n.right = l, r })
	})
	b.Run("objects=256B", func(b *testing.B) {
		benchMark(b, 256, func(n, l, r *medium) { n.left, n.right = l, r })
	})
}

// #endregion greentea
