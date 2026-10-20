package flightrec_test

import (
	"bytes"
	"io"
	"runtime/trace"
	"testing"

	"messenger/internal/platform/flightrec"
)

// Тесты настоящего рекордера не параллельны: в процессе активен только один
// trace.FlightRecorder, и каждый тест останавливает свой в t.Cleanup.

// startRecorder запускает рекордер и останавливает его по окончании теста.
func startRecorder(t *testing.T) *flightrec.Recorder {
	t.Helper()
	r, err := flightrec.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Stop)
	return r
}

// busyWork — немного работы с аллокациями и горутинами, чтобы в трассе были события.
func busyWork() {
	done := make(chan []byte)
	for range 8 {
		go func() { done <- make([]byte, 64<<10) }()
	}
	for range 8 {
		<-done
	}
}

func TestRecorderWriteTo(t *testing.T) {
	r := startRecorder(t)
	busyWork()

	var buf bytes.Buffer
	n, err := r.WriteTo(&buf)
	if err != nil || n <= 0 || int64(buf.Len()) != n {
		t.Fatalf("WriteTo: n = %d, len = %d, err = %v", n, buf.Len(), err)
	}

	r.Stop()
	// Остановленный рекордер освобождает место: следующий стартует.
	startRecorder(t)
}

// TestRecorderIsOnePerProcess — второй активный рекордер рантайм не даёт.
func TestRecorderIsOnePerProcess(t *testing.T) {
	startRecorder(t)
	if r, err := flightrec.Start(); err == nil {
		r.Stop()
		t.Fatal("второй рекордер запустился, ждали ошибку")
	}
}

// TestRecorderCoexistsWithExecutionTracer — /debug/pprof/trace (runtime/trace.Start)
// работает при активном рекордере: трассировщик рантайма один, но у него два
// независимых подписчика — писатель trace.Start и flight recorder.
func TestRecorderCoexistsWithExecutionTracer(t *testing.T) {
	r := startRecorder(t)

	var tr bytes.Buffer
	if err := trace.Start(&tr); err != nil {
		t.Fatalf("trace.Start при активном рекордере: %v", err)
	}
	busyWork()
	trace.Stop()
	if tr.Len() == 0 {
		t.Fatal("trace.Start ничего не записал")
	}
	if n, err := r.WriteTo(io.Discard); err != nil || n <= 0 {
		t.Fatalf("рекордер после trace.Stop: n = %d, err = %v", n, err)
	}
}
