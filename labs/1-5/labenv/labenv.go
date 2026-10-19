// Package labenv печатает шапку, с которой начинается вывод каждой программы лабы 1.5:
// без версии Go и числа ядер числа из уроков нельзя сравнить с числами читателя.
package labenv

import (
	"fmt"
	"io"
	"runtime"
)

// Print пишет версию Go, GOMAXPROCS и число логических процессоров.
func Print(w io.Writer) {
	fmt.Fprintf(w, "go version: %s\n", runtime.Version())
	fmt.Fprintf(w, "GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))
	fmt.Fprintf(w, "NumCPU: %d\n", runtime.NumCPU())
}
