// Command messenger — HTTP-сервер мессенджера. Сборка зависимостей — в app.go
// (newApp); здесь только конфигурация, запуск и остановка по сигналу.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"messenger/internal/platform/httpserver"
)

// shutdownTimeout меньше 30 с, которые Kubernetes по умолчанию ждёт после SIGTERM
// до SIGKILL: сервер должен успеть закончить сам, а не быть убитым посреди ответа.
const shutdownTimeout = 10 * time.Second

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// os.Exit не выполняет defer, поэтому вся работа — в run: её defer'ы отработают
	// до того, как main выйдет с кодом 1.
	if err := run(log); err != nil {
		log.Error("сервер завершился с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	handler, err := newApp(cfg, log)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := httpserver.New(cfg.Addr, handler, log)
	log.Info("слушаю", "addr", cfg.Addr)
	if err := httpserver.Run(ctx, srv, shutdownTimeout); err != nil {
		return err
	}
	log.Info("сервер остановлен")
	return nil
}
