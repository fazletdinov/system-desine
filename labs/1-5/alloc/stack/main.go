// Программа stack показывает рост стека горутины копированием (урок 1.5.8): горутина уходит в
// рекурсию заданной глубины, и на дне программа смотрит, сколько памяти занимают стеки. Затем —
// как рантайм подстраивает начальный размер стека под средний (Go 1.19).
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/metrics"
	"sync"

	"labs/1-5/labenv"
)

const (
	metricStacks       = "/memory/classes/heap/stacks:bytes"
	metricStartingSize = "/gc/stack/starting-size:bytes"

	// deepGoroutines — сколько горутин одновременно стоят на глубине deepDepth, когда идёт сборка
	// мусора: начальный размер считается по среднему стеку всех горутин, и десятка мелких
	// служебных горутин не должно хватить, чтобы среднее осталось маленьким.
	deepGoroutines = 200
	deepDepth      = 1000
)

var depths = []int{10, 1000, 100_000}

func readUint(name string) uint64 {
	s := []metrics.Sample{{Name: name}}
	metrics.Read(s)
	return s[0].Value.Uint64()
}

// #region stack-growth

// dive уходит в рекурсию на depth уровней; на дне вызывает atBottom. Каждый кадр небольшой
// (аргументы, адрес возврата, pad), но кадров много: стартовые 2 КиБ быстро кончаются, и пролог
// функции вызывает runtime.morestack — рантайм выделяет стек вдвое больше и копирует в него старый.
func dive(depth int, atBottom func()) int {
	var pad [4]int // делает кадр заметнее: так рост виден уже на тысяче уровней
	pad[depth%4] = depth
	if depth == 0 {
		atBottom()
		return pad[0]
	}
	return dive(depth-1, atBottom) + pad[depth%4]
}

// stackAtDepth запускает горутину, которая уходит на глубину depth и ждёт там, пока main
// снимает метрику. Возвращает стеки до запуска и на дне рекурсии.
func stackAtDepth(depth int) (before, atBottom uint64) {
	before = readUint(metricStacks)
	reached, release := make(chan struct{}), make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		dive(depth, func() {
			close(reached)
			<-release
		})
	})
	<-reached
	atBottom = readUint(metricStacks)
	close(release)
	wg.Wait()
	return before, atBottom
}

// #endregion stack-growth

// #region starting-size

// startingSizeWithDeepGoroutines держит deepGoroutines горутин на глубине deepDepth во время
// сборки мусора: GC сканирует их стеки, считает средний размер и делает его начальным для новых
// горутин.
func startingSizeWithDeepGoroutines() uint64 {
	var parked, done sync.WaitGroup
	release := make(chan struct{})
	parked.Add(deepGoroutines)
	for range deepGoroutines {
		done.Go(func() {
			dive(deepDepth, func() {
				parked.Done()
				<-release
			})
		})
	}
	parked.Wait()
	runtime.GC() // разметка сканирует стеки и копит их размер
	runtime.GC() // начальный размер пересчитывается в начале следующего цикла
	size := readUint(metricStartingSize)
	close(release)
	done.Wait()
	return size
}

// #endregion starting-size

func main() {
	labenv.Print(os.Stdout)
	for _, d := range depths {
		before, bottom := stackAtDepth(d)
		fmt.Printf("глубина %6d: стеки до %6d КиБ, на дне %6d КиБ (+%d КиБ)\n", d, before/1024, bottom/1024, (bottom-before)/1024)
	}

	runtime.GC()
	start := readUint(metricStartingSize)
	deep := startingSizeWithDeepGoroutines()
	runtime.GC()
	runtime.GC()
	after := readUint(metricStartingSize)
	fmt.Printf("начальный стек: обычно %d байт, после GC с %d горутинами на глубине %d — %d байт, когда они вышли — %d байт\n",
		start, deepGoroutines, deepDepth, deep, after)
}
