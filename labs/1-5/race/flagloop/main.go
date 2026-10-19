//go:build labdemo

// Программа flagloop показывает гонку за флагом: цикл ждёт обычный bool, и модель памяти
// не обещает, что запись когда-нибудь станет видна. Рядом — версия с atomic.Bool.
// Версия с обычным bool может как зависнуть, так и завершиться: gc сегодня не выносит
// чтение флага из цикла, но это свойство компилятора, а не гарантия языка.
package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"labs/1-5/labenv"
)

const (
	setFlagAfter = 10 * time.Millisecond
	giveUpAfter  = 2 * time.Second
)

// #region flagloop
var plainDone bool // читается и пишется из разных горутин без синхронизации

func waitPlainFlag(finished chan<- struct{}) {
	// Гонка: по спецификации компилятор вправе прочитать plainDone один раз до цикла,
	// и тогда цикл бесконечен. Детектор гонок (-race) жалуется на любую такую пару доступов.
	for !plainDone {
	}
	close(finished)
}

func waitAtomicFlag(done *atomic.Bool, finished chan<- struct{}) {
	for !done.Load() {
	}
	close(finished)
}

// outcome ждёт finished не дольше giveUpAfter. Зависшая горутина остаётся жить:
// остановить цикл без выхода из него нечем, процесс завершится вместе с main.
func outcome(finished <-chan struct{}) string {
	ctx, cancel := context.WithTimeout(context.Background(), giveUpAfter)
	defer cancel()
	select {
	case <-finished:
		return "завершилось"
	case <-ctx.Done():
		return "зависло"
	}
}

func main() {
	labenv.Print(os.Stdout)

	finished := make(chan struct{})
	go waitPlainFlag(finished)
	time.Sleep(setFlagAfter)
	plainDone = true
	fmt.Printf("обычный bool:  %s\n", outcome(finished))

	var done atomic.Bool
	finished = make(chan struct{})
	go waitAtomicFlag(&done, finished)
	time.Sleep(setFlagAfter)
	done.Store(true)
	fmt.Printf("atomic.Bool:   %s\n", outcome(finished))
}

// #endregion flagloop
