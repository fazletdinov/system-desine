// Программа preempt показывает асинхронное вытеснение (Go 1.14): при GOMAXPROCS(1)
// горутина в цикле без вызовов функций не даёт main проснуться, если вытеснение выключено.
package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"labs/1-5/labenv"
)

const (
	spinBudget  = 2 * time.Second // потолок цикла, чтобы без вытеснения программа всё же кончилась
	mainSleep   = 10 * time.Millisecond
	calibration = 200_000_000 // итераций для замера скорости цикла
)

var sink int

// spin крутит цикл без вызовов функций: в нём нет точек синхронного вытеснения
// (пролога функции с проверкой стека), остаётся только сигнал SIGURG.
func spin(iterations int) {
	x := 0
	for i := range iterations {
		x += i
	}
	sink = x
}

// iterationsFor подбирает число итераций так, чтобы цикл занял около d на этой машине.
// Часы в самом цикле нельзя: time.Now — вызов функции, то есть точка вытеснения.
func iterationsFor(d time.Duration) int {
	t0 := time.Now()
	spin(calibration)
	perIteration := float64(time.Since(t0)) / calibration
	return int(float64(d) / perIteration)
}

// #region preempt
func main() {
	runtime.GOMAXPROCS(1)
	labenv.Print(os.Stdout)

	iterations := iterationsFor(spinBudget)
	done := make(chan time.Duration)
	go func() {
		t0 := time.Now()
		spin(iterations)
		done <- time.Since(t0)
	}()
	start := time.Now()
	time.Sleep(mainSleep) // main засыпает и отдаёт единственный P циклу
	fmt.Printf("main проснулся через %d мс (просил %d мс)\n", time.Since(start).Milliseconds(), mainSleep.Milliseconds())
	fmt.Printf("цикл работал %d мс\n", (<-done).Milliseconds())
}

// #endregion preempt
