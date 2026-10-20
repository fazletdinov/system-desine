package flightrec_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"messenger/internal/platform/flightrec"
)

// fakeSnapshot — снимок без настоящего рекордера: пишет "trace" и считает вызовы.
type fakeSnapshot struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeSnapshot) WriteTo(w io.Writer) (int64, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	n, err := io.WriteString(w, "trace")
	return int64(n), err
}

func (f *fakeSnapshot) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// wallClock — часы пузыря synctest: time.Now там виртуальное.
type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

const threshold = time.Second

var discard = slog.New(slog.DiscardHandler)

func newDumper(t *testing.T, src flightrec.Snapshotter, dir string) *flightrec.Dumper {
	t.Helper()
	d, err := flightrec.NewDumper(src, dir, threshold, wallClock{}, discard)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// slowFor — обработчик, который отвечает через d виртуального времени. Ждёт таймер, а
// не time.Sleep: в пузыре это одно и то же, но таймер нагляднее говорит «ждём срок».
func slowFor(d time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-time.After(d)
		w.WriteHeader(http.StatusOK)
	})
}

// request проходит через middleware в текущей горутине и возвращает статус ответа.
func request(t *testing.T, h http.Handler) int {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/chats", nil))
	return rec.Code
}

// runDumper запускает Run в пузыре; возвращённая функция останавливает его и отдаёт
// результат Run.
func runDumper(t *testing.T, d *flightrec.Dumper) (stop func() error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	return func() error {
		cancel()
		return <-done
	}
}

// traces — файлы снимков в каталоге.
func traces(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "flight-*.trace"))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestNewDumperValidates(t *testing.T) {
	src, dir := &fakeSnapshot{}, t.TempDir()
	cases := []struct {
		name      string
		src       flightrec.Snapshotter
		dir       string
		threshold time.Duration
		clk       flightrec.Clock
		log       *slog.Logger
	}{
		{"без источника снимков", nil, dir, threshold, wallClock{}, discard},
		{"без каталога", src, "", threshold, wallClock{}, discard},
		{"нулевой порог", src, dir, 0, wallClock{}, discard},
		{"порог равен окну рекордера", src, dir, flightrec.Window, wallClock{}, discard},
		{"без часов", src, dir, threshold, nil, discard},
		{"без журнала", src, dir, threshold, wallClock{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := flightrec.NewDumper(c.src, c.dir, c.threshold, c.clk, c.log); err == nil {
				t.Fatal("ждали ошибку")
			}
		})
	}
}

func TestDumperWritesOnSlowRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		d := newDumper(t, &fakeSnapshot{}, dir)
		stop := runDumper(t, d)

		request(t, d.Middleware(slowFor(threshold+time.Millisecond)))
		synctest.Wait()

		files := traces(t, dir)
		if len(files) != 1 {
			t.Fatalf("снимков %d (%v), ждали 1", len(files), files)
		}
		body, err := os.ReadFile(files[0])
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "trace" {
			t.Fatalf("содержимое %q, ждали снимок источника", body)
		}
		if err := stop(); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
}

func TestDumperSkipsFastRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		src := &fakeSnapshot{}
		d := newDumper(t, src, dir)
		stop := runDumper(t, d)

		request(t, d.Middleware(slowFor(threshold-time.Millisecond)))
		synctest.Wait()

		if files := traces(t, dir); len(files) != 0 || src.count() != 0 {
			t.Fatalf("быстрый запрос: снимков %v, вызовов WriteTo %d", files, src.count())
		}
		if err := stop(); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
}

func TestDumperRateLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		d := newDumper(t, &fakeSnapshot{}, dir)
		stop := runDumper(t, d)
		h := d.Middleware(slowFor(threshold + time.Millisecond))

		for range 5 {
			request(t, h)
			synctest.Wait()
		}
		if files := traces(t, dir); len(files) != 1 {
			t.Fatalf("5 медленных подряд: снимков %d, ждали 1 (не чаще раза в %s)", len(files), flightrec.MinInterval)
		}

		synctest.Sleep(flightrec.MinInterval)
		request(t, h)
		synctest.Wait()

		if files := traces(t, dir); len(files) != 2 {
			t.Fatalf("после MinInterval: снимков %d, ждали 2", len(files))
		}
		if err := stop(); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})
}

// TestMiddlewareNeverBlocks — Run не запущен, сигнал никто не читает: ответы всё равно
// уходят. Заблокируйся отправка — пузырь synctest остался бы с вечно ждущей горутиной
// и тест упал бы с deadlock; лишних горутин после него synctest.Test тоже не пропустит.
func TestMiddlewareNeverBlocks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := newDumper(t, &fakeSnapshot{}, t.TempDir())
		h := d.Middleware(slowFor(threshold + time.Millisecond))

		for i := range 3 {
			if code := request(t, h); code != http.StatusOK {
				t.Fatalf("запрос %d: статус %d, ждали 200", i+1, code)
			}
		}
	})
}

func TestDumperStopsOnCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := newDumper(t, &fakeSnapshot{}, t.TempDir())
		stop := runDumper(t, d)
		synctest.Wait()

		if err := stop(); err != nil {
			t.Fatalf("Run после отмены: %v, want nil", err)
		}
	})
}
