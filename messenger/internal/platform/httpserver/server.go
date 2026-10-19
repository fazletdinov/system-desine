package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Таймауты сервера (подробно — урок 0b.6). Без них один медленный клиент держит
// горутину и соединение сколько угодно: http.Server по умолчанию не ограничивает ничего.
const (
	// Заголовки — сотни байт; 5 с хватает даже очень медленной сети, а атаку
	// Slowloris (заголовки по байту в минуту) обрывает.
	readHeaderTimeout = 5 * time.Second
	// Всё чтение запроса, с телом до 1 МиБ (httpjson.MaxBodyBytes). Таймаут запроса
	// (MESSENGER_REQUEST_TIMEOUT) начинается раньше, чем обработчик читает тело, но
	// r.Body.Read на контекст не смотрит: зависшее чтение тела прерывает только этот
	// дедлайн чтения соединения.
	readTimeout = 10 * time.Second
	// Запас, чтобы ответ 503 timeout успел записаться до дедлайна записи соединения:
	// WriteTimeout = таймаут запроса + writeMargin и отсчитывается от конца чтения
	// заголовков до конца записи ответа. Без запаса дедлайн записи наступил бы
	// одновременно со сроком запроса, и клиент получил бы обрыв вместо 503.
	writeMargin = 5 * time.Second
	// Простой keep-alive соединения между запросами (урок 0b.6 — пулы соединений).
	idleTimeout = 60 * time.Second
)

// New собирает http.Server с таймаутами; requestTimeout — срок обработки запроса
// (middleware.Timeout), от него считается WriteTimeout. Нулевой срок отвергается:
// WriteTimeout стал бы одним запасом, и ответ на долгий запрос обрывался бы молча.
// Внутренние ошибки сервера (например, сбой TLS-рукопожатия) идут в тот же slog на
// уровне Warn, а не в log.
func New(addr string, h http.Handler, log *slog.Logger, requestTimeout time.Duration) (*http.Server, error) {
	if requestTimeout <= 0 {
		return nil, fmt.Errorf("httpserver: таймаут запроса должен быть больше нуля, получено %s", requestTimeout)
	}
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      requestTimeout + writeMargin,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}, nil
}

// Stop — как останавливаться по сигналу.
type Stop struct {
	// Drain вызывается сразу по сигналу: экземпляр объявляет себя неготовым (/readyz → 503).
	Drain func()
	// DrainDelay — сколько ещё принимать новые запросы после Drain. Балансировщик узнаёт
	// о неготовности только со следующей пробой и до того продолжает слать трафик:
	// закрыть порт сразу — значит отказать этим запросам.
	DrainDelay time.Duration
	// Timeout — сколько Shutdown ждёт начатые запросы.
	Timeout time.Duration
}

// #region drain
// Run занимает адрес srv.Addr и обслуживает запросы через Serve.
func Run(ctx context.Context, srv *http.Server, stop Stop) error {
	addr := srv.Addr
	if addr == "" {
		addr = ":http" // как у ListenAndServe
	}
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("httpserver: %w", err)
	}
	return Serve(ctx, srv, ln, stop)
}

// Serve обслуживает запросы на ln, пока не отменят ctx, затем останавливается в три
// шага: Drain → ожидание DrainDelay (запросы ещё принимаются) → Shutdown не дольше
// Timeout. Подробно — урок 1.17. Слушатель — параметр, чтобы тесты подставляли сеть
// в памяти и проверяли сроки остановки в виртуальном времени (synctest, урок 1.5.6).
func Serve(ctx context.Context, srv *http.Server, ln net.Listener, stop Stop) error {
	// Владелец горутины — Serve, способ остановки — Shutdown: после него srv.Serve
	// сразу возвращается. Буфер 1 — чтобы горутина не зависла на отправке, если Serve
	// уже вышел по ошибке Shutdown и результат никто не прочитает.
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		// Сервер упал сам, не дождавшись отмены, — например, порт занят.
		return fmt.Errorf("httpserver: %w", err)
	case <-ctx.Done():
	}

	if stop.Drain != nil {
		stop.Drain()
	}
	delay := time.NewTimer(stop.DrainDelay)
	select {
	case err := <-serveErr:
		delay.Stop()
		return fmt.Errorf("httpserver: во время дренажа: %w", err)
	case <-delay.C:
	}

	// ctx уже отменён; WithoutCancel оставляет его значения, но не отмену — иначе
	// Shutdown получил бы мёртвый контекст и не стал бы ждать запросы вовсе.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stop.Timeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Не дождались: оставшиеся запросы обрываем сами. Иначе зависшее соединение
		// пережило бы Serve и держало процесс до SIGKILL.
		if cerr := srv.Close(); cerr != nil {
			err = errors.Join(err, cerr)
		}
		return fmt.Errorf("httpserver: остановка: %w", err)
	}
	// После Shutdown srv.Serve возвращает http.ErrServerClosed — это не ошибка.
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("httpserver: %w", err)
	}
	return nil
}

// #endregion drain
