package httpserver_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"messenger/internal/platform/httpserver"
)

// newServer — сервер с таймаутом запроса в 1 с; New с ним не ошибается.
func newServer(t *testing.T, addr string, h http.Handler) *http.Server {
	t.Helper()
	srv, err := httpserver.New(addr, h, slog.New(slog.DiscardHandler), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestNewRejectsNonPositiveRequestTimeout(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		if _, err := httpserver.New("127.0.0.1:0", http.NotFoundHandler(), slog.New(slog.DiscardHandler), d); err == nil {
			t.Errorf("New с таймаутом %v: ждали ошибку", d)
		}
	}
}

func TestNewWriteTimeoutCoversRequestTimeout(t *testing.T) {
	srv, err := httpserver.New("127.0.0.1:0", http.NotFoundHandler(), slog.New(slog.DiscardHandler), 7*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	// Ответ о таймауте (503) должен успеть уйти до того, как сервер оборвёт запись.
	if srv.WriteTimeout <= 7*time.Second {
		t.Fatalf("WriteTimeout %v не больше таймаута запроса", srv.WriteTimeout)
	}
	if srv.ReadHeaderTimeout == 0 || srv.ReadTimeout == 0 || srv.IdleTimeout == 0 {
		t.Fatalf("таймауты не выставлены: %+v", srv)
	}
}

// Run — тонкая обёртка «занять порт + Serve»: два теста на настоящей сети проверяют
// именно её. Остановка проверяется на Serve в сети в памяти, под synctest.
func TestRunStopsOnCancel(t *testing.T) {
	srv := newServer(t, "127.0.0.1:0", http.NotFoundHandler())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- httpserver.Run(ctx, srv, httpserver.Stop{Timeout: time.Second}) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v, want nil после отмены", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run не завершился после отмены контекста")
	}
}

func TestRunReturnsListenError(t *testing.T) {
	// Занимаем порт сами: Run на нём должен упасть сразу.
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	srv := newServer(t, ln.Addr().String(), http.NotFoundHandler())

	if err := httpserver.Run(context.Background(), srv, httpserver.Stop{Timeout: time.Second}); err == nil {
		t.Fatal("Run: want error, порт занят")
	}
}

// pipeListener — сеть в памяти для Serve: dial создаёт net.Pipe и отдаёт серверный
// конец в Accept. Всё ожидание — на каналах пузыря synctest, поэтому время в тестах
// виртуальное, а synctest.Wait видит, когда сервер и клиент «устоялись». Это не
// httptest.NewTestServer: тот сам вызывает Serve и Shutdown, а проверяем мы именно их.
type pipeListener struct {
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{conns: make(chan net.Conn), closed: make(chan struct{})}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

// errRefused — порт закрыт: Serve уже не принимает соединения.
var errRefused = errors.New("pipe: соединение отклонено")

func (l *pipeListener) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	client, server := net.Pipe()
	select {
	case l.conns <- server:
		return client, nil
	case <-l.closed:
	case <-ctx.Done():
	}
	_, _ = client.Close(), server.Close()
	return nil, errRefused
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// running — Serve, запущенный в горутине пузыря, и клиент к нему.
type running struct {
	client *http.Client
	cancel context.CancelFunc // «сигнал»: отменяет ctx Serve
	done   chan struct{}      // закрыт, когда Serve вернулся
	err    error              // результат Serve; читать после done
}

// serve запускает Serve на сети в памяти. Вызывать внутри synctest.Test.
func serve(t *testing.T, h http.Handler, stop httpserver.Stop) *running {
	t.Helper()
	ln := newPipeListener()
	ctx, cancel := context.WithCancel(t.Context())
	r := &running{
		client: &http.Client{Transport: &http.Transport{DialContext: ln.dial}},
		cancel: cancel,
		done:   make(chan struct{}),
	}
	srv := newServer(t, "", h)
	go func() {
		defer close(r.done)
		r.err = httpserver.Serve(ctx, srv, ln, stop)
	}()
	return r
}

// returned — вернулся ли Serve к этому моменту; после synctest.Wait — без гонки.
func (r *running) returned() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

// wait ждёт возврата Serve и закрывает простаивающие соединения клиента, чтобы их
// горутины не пережили пузырь.
func (r *running) wait() error {
	<-r.done
	r.client.CloseIdleConnections()
	return r.err
}

// get шлёт запрос в горутине; результат — в канале.
func (r *running) get(ctx context.Context) <-chan error {
	errc := make(chan error, 1)
	go func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://messenger/", nil)
		if err != nil {
			errc <- err
			return
		}
		resp, err := r.client.Do(req)
		if err == nil {
			err = resp.Body.Close()
		}
		errc <- err
	}()
	return errc
}

// blocking — обработчик, который ждёт release; entered закрывается, когда запрос внутри.
func blocking() (h http.Handler, entered, release chan struct{}) {
	entered, release = make(chan struct{}), make(chan struct{})
	h = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	return h, entered, release
}

func noContent() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func TestRunWaitsForInFlightRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h, entered, release := blocking()
		r := serve(t, h, httpserver.Stop{Timeout: 5 * time.Second})
		reqErr := r.get(t.Context())
		<-entered

		r.cancel()
		synctest.Sleep(time.Second)
		if r.returned() {
			t.Fatalf("Serve вернулся (%v), не дождавшись запроса", r.err)
		}
		close(release)

		if err := <-reqErr; err != nil {
			t.Errorf("запрос оборвался: %v", err)
		}
		if err := r.wait(); err != nil {
			t.Errorf("Serve: %v, want nil", err)
		}
	})
}

func TestRunForceClosesAfterShutdownTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h, entered, release := blocking()
		const timeout = 50 * time.Millisecond
		r := serve(t, h, httpserver.Stop{Timeout: timeout})
		reqErr := r.get(t.Context())
		<-entered
		start := time.Now()

		r.cancel()

		if err := r.wait(); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Serve: %v, want DeadlineExceeded", err)
		}
		if elapsed := time.Since(start); elapsed != timeout {
			t.Fatalf("Serve вернулся через %v, ждали Timeout = %v", elapsed, timeout)
		}
		// Close оборвал соединение: клиент получил ошибку, а не ждёт вечно.
		if err := <-reqErr; err == nil {
			t.Error("запрос завершился успешно, хотя соединение должно быть закрыто")
		}
		// Обработчик Close не прерывает: отпускаем его, чтобы пузырь завершился.
		close(release)
	})
}

func TestRunDrainsBeforeShutdown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const drainDelay = 5 * time.Second
		var drainedAt time.Time
		drains := 0
		r := serve(t, noContent(), httpserver.Stop{
			// Drain вызывает горутина Serve; читаем после её возврата.
			Drain:      func() { drains++; drainedAt = time.Now() },
			DrainDelay: drainDelay,
			Timeout:    time.Second,
		})
		if err := <-r.get(t.Context()); err != nil {
			t.Fatal(err)
		}
		signaled := time.Now()

		r.cancel()

		if err := r.wait(); err != nil {
			t.Fatalf("Serve: %v", err)
		}
		if drains != 1 || !drainedAt.Equal(signaled) {
			t.Fatalf("Drain вызван %d раз, через %v после отмены; ждали один раз сразу", drains, drainedAt.Sub(signaled))
		}
		// Shutdown без начатых запросов возвращается сразу: Serve выходит ровно через DrainDelay.
		if elapsed := time.Since(drainedAt); elapsed != drainDelay {
			t.Fatalf("Shutdown через %v после Drain, ждали DrainDelay = %v", elapsed, drainDelay)
		}
	})
}

func TestRunServesDuringDrainDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const drainDelay = 5 * time.Second
		drained := make(chan struct{})
		r := serve(t, noContent(), httpserver.Stop{Drain: func() { close(drained) }, DrainDelay: drainDelay, Timeout: time.Second})

		r.cancel()
		<-drained
		// Балансировщик ещё не знает о дренаже и шлёт запросы — их надо обслужить.
		synctest.Sleep(drainDelay - time.Second)
		if err := <-r.get(t.Context()); err != nil {
			t.Fatalf("запрос во время дренажа: %v", err)
		}

		if err := r.wait(); err != nil {
			t.Fatalf("Serve: %v", err)
		}
		// После Shutdown порт закрыт: новый запрос не проходит.
		if err := <-r.get(t.Context()); err == nil || !strings.Contains(err.Error(), errRefused.Error()) {
			t.Fatalf("запрос после остановки: %v, ждали отказ в соединении", err)
		}
	})
}
