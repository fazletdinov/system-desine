// Command messenger — composition root: единственное место, где конкретные реализации
// (часы, генератор id, хранилища, HTTP) соединяются с use case'ами через их порты.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	chathttp "messenger/internal/chat/adapters/httpapi"
	chatmemory "messenger/internal/chat/adapters/memory"
	"messenger/internal/chat/adapters/userchecker"
	chatusecase "messenger/internal/chat/usecase"
	"messenger/internal/platform/clock"
	"messenger/internal/platform/httpserver"
	"messenger/internal/platform/idgen"
	userhttp "messenger/internal/user/adapters/httpapi"
	usermemory "messenger/internal/user/adapters/memory"
	userusecase "messenger/internal/user/usecase"
)

const (
	// Только loopback: сборка с X-User-ID вместо аутентификации не должна быть видна
	// из сети (ADR-0002). Слушать все интерфейсы — явное решение: MESSENGER_ADDR=:8080.
	defaultAddr = "127.0.0.1:8080"
	// shutdownTimeout меньше 30 с, которые Kubernetes по умолчанию ждёт после SIGTERM
	// до SIGKILL: сервер должен успеть закончить сам, а не быть убитым посреди ответа.
	shutdownTimeout = 10 * time.Second
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// os.Exit не выполняет defer, поэтому вся работа — в run: её defer'ы отработают
	// до того, как main выйдет с кодом 1.
	if err := run(log); err != nil {
		log.Error("сервер завершился с ошибкой", "err", err)
		os.Exit(1)
	}
}

// #region wiring
func run(log *slog.Logger) error {
	addr := os.Getenv("MESSENGER_ADDR")
	if addr == "" {
		addr = defaultAddr
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	clk, ids := clock.System{}, idgen.UUIDv7{}
	users, chats := usermemory.NewStore(), chatmemory.NewStore()

	registerUser, err := userusecase.NewRegisterUser(users, clk, ids)
	if err != nil {
		return fmt.Errorf("сборка: %w", err)
	}
	getUser, err := userusecase.NewGetUser(users)
	if err != nil {
		return fmt.Errorf("сборка: %w", err)
	}
	// Мост между контекстами: chat спрашивает о пользователях через свой порт.
	checker, err := userchecker.New(getUser)
	if err != nil {
		return fmt.Errorf("сборка: %w", err)
	}
	// Одно хранилище chats реализует сразу несколько узких портов.
	createChat, err := chatusecase.NewCreateChat(chats, chats, checker, clk, ids)
	if err != nil {
		return fmt.Errorf("сборка: %w", err)
	}
	sendMessage, err := chatusecase.NewSendMessage(chats, chats, clk, ids)
	if err != nil {
		return fmt.Errorf("сборка: %w", err)
	}
	listMessages, err := chatusecase.NewListMessages(chats, chats)
	if err != nil {
		return fmt.Errorf("сборка: %w", err)
	}

	userAPI, err := userhttp.New(registerUser, getUser, log)
	if err != nil {
		return fmt.Errorf("сборка: %w", err)
	}
	chatAPI, err := chathttp.New(createChat, sendMessage, listMessages, log)
	if err != nil {
		return fmt.Errorf("сборка: %w", err)
	}
	mux := http.NewServeMux()
	userAPI.Register(mux)
	chatAPI.Register(mux)

	srv := httpserver.New(addr, mux, log)
	log.Info("слушаю", "addr", addr)
	if err := httpserver.Run(ctx, srv, shutdownTimeout); err != nil {
		return err
	}
	log.Info("сервер остановлен")
	return nil
}

// #endregion wiring
