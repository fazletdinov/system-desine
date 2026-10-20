// Command messenger — HTTP-сервер мессенджера. Сборка зависимостей — в app.go
// (newApp); здесь только конфигурация, запуск и остановка по сигналу; служебные
// команды — в cli.go, второй сигнал — в signals.go.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"messenger/internal/platform/httpserver"
	"messenger/internal/platform/lifecycle"
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
	srv, err := httpserver.New(cfg.Addr, a.handler, log, cfg.RequestTimeout)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Второй канал на те же сигналы — для второго сигнала во время остановки
	// (signals.go). Подписка после NotifyContext: сигнал между двумя подписками
	// начнёт остановку, а не пропадёт. Буфер 2 — копия первого сигнала и второй:
	// signal.Notify не ждёт читателя и лишнее отбрасывает.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	// Владелец горутины — run: после signal.Stop сигналов в канал больше не будет,
	// и закрытие канала её останавливает.
	defer func() {
		signal.Stop(sigCh)
		close(sigCh)
	}()
	go exitOnSecondSignal(sigCh, ctx.Done(), os.Exit, log)

	log.Info("слушаю", "addr", cfg.Addr, "version", version)
	// #region stages-wiring
	// HTTP — Front: по сигналу или сбою он первым проходит дренаж и Shutdown; фоновые
	// компоненты (Back) получают отмену после него — начатые запросы могут на них опираться.
	return lifecycle.Run(ctx, log, lifecycle.Stages{
		Front: []lifecycle.Component{httpComponent{
			srv: srv,
			log: log,
			stop: httpserver.Stop{
				Drain: func() {
					a.probes.StartDraining()
					log.Info("останавливаюсь: /readyz отвечает 503", "drain_delay", cfg.DrainDelay.String())
				},
				DrainDelay: cfg.DrainDelay,
				Timeout:    cfg.ShutdownTimeout,
			},
		}},
		Back: a.back,
	})
	// #endregion stages-wiring
}

// httpComponent — HTTP-сервер как lifecycle.Component: адаптер над httpserver.Run.
type httpComponent struct {
	srv  *http.Server
	log  *slog.Logger
	stop httpserver.Stop
}

func (httpComponent) Name() string { return "http" }

func (c httpComponent) Run(ctx context.Context) error {
	if err := httpserver.Run(ctx, c.srv, c.stop); err != nil {
		return err
	}
	c.log.Info("сервер остановлен")
	return nil
}
