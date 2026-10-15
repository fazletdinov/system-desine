// Command hog занимает память кусками по 1 МиБ и трогает каждую страницу. Только
// тронутая страница становится настоящей памятью процесса: make без записи — обещание,
// и OOM-killer его не увидит (урок 1.15).
package main

import (
	"flag"
	"fmt"
	"runtime"
)

const (
	chunk    = 1 << 20
	pageSize = 4096
	// reportEvery — печатать прогресс каждые 16 МиБ, чтобы в выводе было видно, где убили.
	reportEvery = 16
)

func main() {
	mib := flag.Int("mib", 256, "сколько МиБ занять")
	flag.Parse()
	chunks := make([][]byte, 0, *mib)
	for i := 1; i <= *mib; i++ {
		b := make([]byte, chunk)
		for j := 0; j < len(b); j += pageSize {
			b[j] = 1
		}
		chunks = append(chunks, b)
		if i%reportEvery == 0 {
			fmt.Printf("занято %d МиБ\n", i)
		}
	}
	fmt.Printf("занято %d МиБ — лимит не сработал\n", *mib)
	runtime.KeepAlive(chunks)
}
