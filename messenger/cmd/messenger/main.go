// Command messenger — HTTP-сервер мессенджера. Сборка зависимостей — в app.go
// (newApp); здесь только конфигурация, запуск и остановка по сигналу; служебные
// команды — в cli.go.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"messenger/internal/platform/httpserver"
)

func main() {
	if code, handled := subcommand(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
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
	a, err := newApp(cfg, log)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := httpserver.New(cfg.Addr, a.handler, log, cfg.RequestTimeout)
	log.Info("слушаю", "addr", cfg.Addr, "version", version)
	err = httpserver.Run(ctx, srv, httpserver.Stop{
		Drain: func() {
			a.probes.StartDraining()
			log.Info("останавливаюсь: /readyz отвечает 503", "drain_delay", cfg.DrainDelay.String())
		},
		DrainDelay: cfg.DrainDelay,
		Timeout:    cfg.ShutdownTimeout,
	})
	if err != nil {
		return err
	}
	log.Info("сервер остановлен")
	return nil
}
