package main

import (
	"math/rand/v2"
	"testing"
)

// Если перестановка распадётся на несколько циклов, обход по 256 МБ может крутиться
// в маленьком цикле из кэша и показать задержку L1 вместо RAM. Тест ловит именно это.
func TestSattoloIsSingleCycle(t *testing.T) {
	for _, n := range []int{2, 3, 10, 1000} {
		p := sattolo(n, rand.New(rand.NewPCG(uint64(n), 1)))
		var at uint64
		for step := 1; step <= n; step++ {
			at = p[at]
			if at == 0 && step != n {
				t.Fatalf("n=%d: вернулись в 0 за %d шагов — цикл короче n", n, step)
			}
		}
		if at != 0 {
			t.Fatalf("n=%d: за n шагов не вернулись в 0", n)
		}
	}
}

func TestChainVisitsEveryNode(t *testing.T) {
	nodes := newChain(64*lineBytes, 7)
	seen := make(map[uint64]bool)
	var p uint64
	for range len(nodes) {
		seen[p] = true
		p = nodes[p].next
	}
	if len(seen) != len(nodes) {
		t.Fatalf("обход посетил %d узлов из %d", len(seen), len(nodes))
	}
}

func TestSum(t *testing.T) {
	for _, n := range []int{0, 1, 3, 4, 7, 1000} {
		xs := make([]uint64, n)
		var want uint64
		for i := range xs {
			xs[i] = uint64(i * 3)
			want += xs[i]
		}
		if got := sum(xs); got != want {
			t.Errorf("n=%d: sum=%d, ждали %d", n, got, want)
		}
	}
}
