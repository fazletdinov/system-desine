package main

import (
	"math/rand/v2"
	"slices"
	"time"
)

// lineBytes — размер строки кэша x86. Один узел цепочки занимает ровно строку:
// каждое обращение — отдельная строка, соседние узлы не приезжают «бесплатно» вместе с текущим.
const lineBytes = 64

// node — узел цепочки указателей. Хранится индекс следующего узла, а не указатель:
// сборщику мусора нечего сканировать в 256 МБ, и он не мешает замеру.
type node struct {
	next uint64
	_    [lineBytes/8 - 1]uint64
}

// sattolo возвращает перестановку p из n элементов, образующую ОДИН цикл длины n
// (алгоритм Саттоло). Обычное перемешивание Фишера — Йетса дало бы несколько циклов,
// и обход мог бы застрять в маленьком цикле, который целиком помещается в кэш.
func sattolo(n int, rng *rand.Rand) []uint64 {
	p := make([]uint64, n)
	for i := range p {
		p[i] = uint64(i)
	}
	for i := n - 1; i > 0; i-- {
		j := rng.IntN(i) // j < i, а не j <= i: в этом и отличие от Фишера — Йетса
		p[i], p[j] = p[j], p[i]
	}
	return p
}

// newChain раскладывает цикл по массиву узлов размером sizeBytes: из узла i
// обход идёт в узел perm[i]. Порядок случайный, поэтому аппаратный prefetcher
// не угадывает следующий адрес и каждое обращение платит полную задержку уровня.
func newChain(sizeBytes int, seed uint64) []node {
	n := sizeBytes / lineBytes
	perm := sattolo(n, rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)))
	nodes := make([]node, n)
	for i := range nodes {
		nodes[i].next = perm[i]
	}
	return nodes
}

// chase делает steps переходов по цепочке. Каждый следующий адрес известен только
// после загрузки текущего узла, поэтому загрузки не перекрываются: время шага —
// задержка одного обращения, а не пропускная способность.
func chase(nodes []node, steps int) uint64 {
	var p uint64
	for range steps {
		p = nodes[p].next
	}
	return p
}

// sink не даёт компилятору выбросить результат обхода как неиспользуемый.
var sink uint64

// chaseLatency прогревает цепочку и возвращает медиану из rounds замеров, нс на обращение.
// Медиана, а не среднее: одно вытеснение потока или прерывание не сдвигает результат.
func chaseLatency(nodes []node, steps, rounds int) float64 {
	sink += chase(nodes, len(nodes)) // прогрев: страницы и строки уже затронуты
	per := make([]float64, rounds)
	for r := range per {
		t0 := time.Now()
		sink += chase(nodes, steps)
		per[r] = float64(time.Since(t0).Nanoseconds()) / float64(steps)
	}
	return median(per)
}

func median(xs []float64) float64 {
	s := slices.Clone(xs)
	slices.Sort(s)
	return s[len(s)/2]
}
