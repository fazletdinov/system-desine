package flightrec

import (
	"fmt"
	"io"
	"runtime/trace"
	"sync"
	"time"
)

// Параметры окна рекордера (урок 1.5.10).
const (
	// Window — сколько последних секунд трассы держит рекордер (MinAge). Порог
	// медленного запроса обязан быть меньше: снимок пишется после ответа, и окно
	// должно ещё помнить начало запроса — иначе в файле только его хвост.
	Window = 10 * time.Second
	// MaxBytes — потолок памяти буфера трассы. MaxBytes важнее MinAge: под большой
	// нагрузкой окно сожмётся, но процесс не съест лишние мегабайты из 256 МБ контейнера.
	MaxBytes = 16 << 20
	// MinInterval — не чаще одного файла в минуту: медленный период (GC под давлением,
	// перегрузка хешера) делает медленными сотни запросов подряд, и без ограничения
	// снимки по 16 МБ завалили бы диск, а запись их — CPU, которого и так не хватает.
	MinInterval = time.Minute
)

// Recorder — единственный flight recorder процесса. Рантайм не даёт запустить второй
// активный рекордер, поэтому его создаёт composition root, а потребители (Dumper,
// служебный порт) получают только порт WriteTo.
type Recorder struct {
	// mu делает снимки последовательными: trace.FlightRecorder.WriteTo не ждёт
	// параллельный вызов, а сразу отвечает ошибкой «already in progress», и снимок по
	// медленному запросу пропал бы из-за того, что кто-то снимает /debug/flight.
	mu sync.Mutex
	fr *trace.FlightRecorder
}

// Start создаёт рекордер с окном Window и потолком MaxBytes и запускает запись.
// Ошибка — если в процессе уже есть активный рекордер.
func Start() (*Recorder, error) {
	fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{MinAge: Window, MaxBytes: MaxBytes})
	if err := fr.Start(); err != nil {
		return nil, fmt.Errorf("flightrec: запуск рекордера: %w", err)
	}
	return &Recorder{fr: fr}, nil
}

// WriteTo пишет в w снимок окна — трассу последних секунд в формате go tool trace.
func (r *Recorder) WriteTo(w io.Writer) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, err := r.fr.WriteTo(w)
	if err != nil {
		return n, fmt.Errorf("flightrec: снимок: %w", err)
	}
	return n, nil
}

// Stop останавливает запись и освобождает буфер; повторный вызов ничего не делает.
// Ждёт идущий снимок: мьютекс тот же.
func (r *Recorder) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fr.Stop()
}
