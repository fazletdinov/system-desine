//go:build labdemo

// Программа history показывает, что история детектора гонок конечна. Две горутины по n раз
// пишут в одну переменную без синхронизации — это гонка при любом n. Запускается только с
// -race; число записей задаёт флаг -n, размер журнала — GORACE=history_size. Найдёт ли
// детектор гонку, зависит от того, сохранился ли в журнале стек прошлого обращения.
package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"labs/1-5/labenv"
)

// defaultWrites — столько записей хватает, чтобы на одном P гонка обычно терялась (урок 1.5.3).
const defaultWrites = 1_000_000

// #region history
var shared int // пишут обе горутины без синхронизации

// write вынесена из цикла, чтобы каждая запись была отдельным обращением к памяти,
// которое инструментирует -race, а не одной записью после цикла.
//
//go:noinline
func write(i int) { shared = i }

func main() {
	n := flag.Int("n", defaultWrites, "сколько раз пишет каждая горутина")
	flag.Parse()
	labenv.Print(os.Stdout)

	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			for i := range *n {
				write(i)
			}
		})
	}
	wg.Wait()
	fmt.Printf("записей на горутину: %d\n", *n)
}

// #endregion history
