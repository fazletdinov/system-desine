package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Таймауты сервера (подробно — урок 0b.6). Без них один медленный клиент держит
// горутину и соединение сколько угодно: http.Server по умолчанию не ограничивает ничего.
const (
	// Заголовки — сотни байт; 5 с хватает даже очень медленной сети, а атаку
	// Slowloris (заголовки по байту в минуту) обрывает.
	readHeaderTimeout = 5 * time.Second
	// Всё чтение запроса, с телом до 1 МиБ (httpjson.MaxBodyBytes).
	readTimeout = 10 * time.Second
	// От конца чтения заголовков до конца записи ответа: наши обработчики работают
	// с памятью и отвечают за миллисекунды, 10 с — с большим запасом.
	writeTimeout = 10 * time.Second
	// Простой keep-alive соединения между запросами (урок 0b.6 — пулы соединений).
	idleTimeout = 60 * time.Second
)

// New собирает http.Server с таймаутами. Внутренние ошибки сервера (например, сбой
// TLS-рукопожатия) идут в тот же slog на уровне Warn, а не в стандартный log.
func New(addr string, h http.Handler, log *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
}

// Run обслуживает запросы, пока не отменят ctx, затем корректно останавливает сервер:
// перестаёт принимать соединения и ждёт текущие запросы не дольше shutdownTimeout.
// Это минимальная версия; подробно о корректном завершении — модуль 1.5.
func Run(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error {
	// Владелец горутины — Run, способ остановки — Shutdown: после него ListenAndServe
	// сразу возвращается. Буфер 1 — чтобы горутина не зависла на отправке, если Run
	// уже вышел по ошибке Shutdown и результат никто не прочитает.
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	select {
	case err := <-serveErr:
		// Сервер упал сам, не дождавшись отмены, — например, порт занят.
		return fmt.Errorf("httpserver: %w", err)
	case <-ctx.Done():
	}

	// ctx уже отменён; WithoutCancel оставляет его значения, но не отмену — иначе
	// Shutdown получил бы мёртвый контекст и не стал бы ждать запросы вовсе.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("httpserver: остановка: %w", err)
	}
	// После Shutdown ListenAndServe возвращает http.ErrServerClosed — это не ошибка.
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("httpserver: %w", err)
	}
	return nil
}
