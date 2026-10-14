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
	"messenger/internal/platform/httpauth"
	"messenger/internal/platform/httpserver"
	"messenger/internal/platform/idgen"
	"messenger/internal/platform/middleware"
	"messenger/internal/platform/problem"
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

// publicRoutes — маршруты, открытые без аутентификации: метод и путь без параметров.
// Всё остальное закрыто: забытый здесь маршрут получит 401, а не откроется всем.
var publicRoutes = []string{"POST /v1/users"}

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

	// #region chain
	// Первый — самый внешний. Request ID нужен всем, кто пишет в журнал; Recover — внутри
	// AccessLog, чтобы паника попала в журнал как 500; аутентификация — до маршрутизатора.
	handler := middleware.Chain(
		middleware.RouteErrors(mux, routeError(log)),
		middleware.RequestID(ids),
		middleware.AccessLog(log),
		middleware.Recover(log, internalError(log)),
		httpauth.Middleware(httpauth.HeaderStub{}, publicRoutes, unauthenticated(log)),
	)
	// #endregion chain

	srv := httpserver.New(addr, handler, log)
	log.Info("слушаю", "addr", addr)
	if err := httpserver.Run(ctx, srv, shutdownTimeout); err != nil {
		return err
	}
	log.Info("сервер остановлен")
	return nil
}

// #endregion wiring

// Ответы конвейера — в том же формате RFC 9457, что ошибки адаптеров: клиенту всё равно,
// кто отказал — маршрутизатор, аутентификация или домен. Middleware формата не знают,
// поэтому функции ответа собираются здесь.

func routeError(log *slog.Logger) func(http.ResponseWriter, *http.Request, int) {
	return func(w http.ResponseWriter, r *http.Request, status int) {
		if status == http.StatusMethodNotAllowed {
			writeProblem(log, w, r, problem.Problem{Code: problem.CodeMethodNotAllowed, Detail: "метод не поддерживается для этого пути"})
			return
		}
		// Пустой шаблон без Allow — «не найдено». Даже редирект на очищенный путь
		// (/v1//x → /v1/x) ServeMux отдаёт с пустым шаблоном, только когда и там пусто.
		writeProblem(log, w, r, problem.Problem{Code: problem.CodeNotFound, Detail: "маршрут не найден"})
	}
}

func internalError(log *slog.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		writeProblem(log, w, r, problem.Problem{Code: problem.CodeInternal})
	}
}

// unauthenticated не различает причины отказа: клиенту хватит «представьтесь», а
// подробности помогли бы только подбирать учётные данные.
func unauthenticated(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, _ error) {
		writeProblem(log, w, r, problem.Problem{Code: problem.CodeUnauthenticated, Detail: "нужен заголовок " + httpauth.UserIDHeader})
	}
}

func writeProblem(log *slog.Logger, w http.ResponseWriter, r *http.Request, p problem.Problem) {
	if err := problem.Write(w, r, p); err != nil {
		log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
