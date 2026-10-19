// Пакет sched — бенчмарки цены переключения: между горутинами и между потоками ОС.
package sched

import (
	"runtime"
	"sync"
	"testing"
)

func BenchmarkSpawnGoroutine(b *testing.B) {
	var wg sync.WaitGroup
	for range b.N {
		wg.Go(func() {})
	}
	wg.Wait()
}

// pingPong гоняет значение между двумя горутинами по небуферизованным каналам:
// каждое обращение — это переключение с одной горутины на другую.
func pingPong(b *testing.B, pinThreads bool) {
	ping, pong := make(chan struct{}), make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		if pinThreads {
			runtime.LockOSThread()
		}
		for range ping {
			pong <- struct{}{}
		}
	})
	if pinThreads {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}
	b.ResetTimer()
	for range b.N {
		ping <- struct{}{}
		<-pong
	}
	b.StopTimer()
	close(ping)
	wg.Wait()
}

// Переключение горутин в одном потоке: планировщик Go, без ядра ОС.
func BenchmarkSwitchGoroutines(b *testing.B) { pingPong(b, false) }

// Те же горутины, закреплённые за разными потоками: каждое переключение — пробуждение потока через futex.
func BenchmarkSwitchOSThreads(b *testing.B) { pingPong(b, true) }
