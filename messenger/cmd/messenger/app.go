package main

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"messenger/internal/auth/adapters/argon2id"
	authhttp "messenger/internal/auth/adapters/httpapi"
	"messenger/internal/auth/adapters/jwt"
	authmemory "messenger/internal/auth/adapters/memory"
	"messenger/internal/auth/adapters/userbridge"
	authusecase "messenger/internal/auth/usecase"
	chathttp "messenger/internal/chat/adapters/httpapi"
	chatgen "messenger/internal/chat/adapters/httpapi/gen"
	chatmemory "messenger/internal/chat/adapters/memory"
	"messenger/internal/chat/adapters/userchecker"
	chatusecase "messenger/internal/chat/usecase"
	"messenger/internal/platform/clock"
	"messenger/internal/platform/httpauth"
	"messenger/internal/platform/idempotency"
	idemmemory "messenger/internal/platform/idempotency/memory"
	"messenger/internal/platform/idgen"
	"messenger/internal/platform/middleware"
	"messenger/internal/platform/problem"
	userhttp "messenger/internal/user/adapters/httpapi"
	usermemory "messenger/internal/user/adapters/memory"
	userusecase "messenger/internal/user/usecase"
)

const (
	// Loopback по умолчанию: сервис говорит по голому HTTP, и токены в нём шли бы по
	// сети открытым текстом. TLS — на шлюзе (модуль 6.7); в Docker (1c) слушать все
	// интерфейсы — явное решение: MESSENGER_ADDR=:8080.
	defaultAddr = "127.0.0.1:8080"
	keyFileEnv  = "MESSENGER_JWT_KEY_FILE"
)

// publicRoutes — маршруты, открытые без аутентификации: метод и путь без параметров.
// Всё остальное закрыто: забытый здесь маршрут получит 401, а не откроется всем.
var publicRoutes = []string{"POST /v1/users", "POST /v1/tokens"}

// config — всё, что сборка берёт снаружи. HasherParams меняет только сквозной тест:
// боевые параметры хеширования не настраиваются из окружения.
type config struct {
	Addr         string
	SigningKey   ed25519.PrivateKey
	HasherParams argon2id.Params
}

// loadConfig читает окружение. Без ключа подписи сервер не стартует: сгенерировать
// ключ «на лету» значило бы молча обесценить все токены при каждом перезапуске.
func loadConfig() (config, error) {
	cfg := config{Addr: os.Getenv("MESSENGER_ADDR"), HasherParams: argon2id.OWASPMinimum}
	if cfg.Addr == "" {
		cfg.Addr = defaultAddr
	}
	path := os.Getenv(keyFileEnv)
	if path == "" {
		return config{}, errors.New("задайте " + keyFileEnv + " — ключ создаёт go run ./cmd/keygen -out jwt.pem")
	}
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return config{}, fmt.Errorf("ключ подписи: %w", err)
	}
	if cfg.SigningKey, err = jwt.ParsePrivateKey(pemBytes); err != nil {
		return config{}, fmt.Errorf("ключ подписи %s: %w", path, err)
	}
	return cfg, nil
}

// #region wiring
// newApp — composition root: единственное место, где конкретные реализации (часы,
// генератор id, хранилища, JWT, HTTP) соединяются с use case'ами через их порты.
func newApp(cfg config, log *slog.Logger) (http.Handler, error) {
	clk, ids := clock.System{}, idgen.UUIDv7{}
	users := usermemory.NewStore()

	getUser, err := userusecase.NewGetUser(users)
	if err != nil {
		return nil, fmt.Errorf("сборка: %w", err)
	}
	userAPI, err := userhttp.New(getUser, log)
	if err != nil {
		return nil, fmt.Errorf("сборка: %w", err)
	}
	authAPI, verifier, err := newAuth(cfg, users, clk, ids, log)
	if err != nil {
		return nil, fmt.Errorf("сборка auth: %w", err)
	}
	chatAPI, err := newChat(getUser, clk, ids, log)
	if err != nil {
		return nil, fmt.Errorf("сборка chat: %w", err)
	}

	mux := http.NewServeMux()
	authAPI.Register(mux)
	userAPI.Register(mux)
	// #region idempotency-wiring
	// Ключ только у создания чата: у сообщений — естественный ключ client_message_id.
	// Middleware операции работает после маршрутизатора и аутентификации: ему нужны
	// шаблон маршрута (r.Pattern) и личность — область ключа.
	idemStore := idemmemory.NewStore()
	chatAPI.Register(mux, chatgen.MiddlewareFunc(idempotency.Middleware(idemStore, clk, log, "POST /v1/chats")))
	// #endregion idempotency-wiring

	bearer, err := httpauth.NewBearer(verifier)
	if err != nil {
		return nil, fmt.Errorf("сборка: %w", err)
	}
	// #region chain
	// Первый — самый внешний. Request ID нужен всем, кто пишет в журнал; Recover — внутри
	// AccessLog, чтобы паника попала в журнал как 500; аутентификация — до маршрутизатора.
	return middleware.Chain(
		middleware.RouteErrors(mux, routeError(log)),
		middleware.RequestID(ids),
		middleware.AccessLog(log),
		middleware.Recover(log, internalError(log)),
		httpauth.Middleware(bearer, publicRoutes, unauthenticated(log)),
	), nil
	// #endregion chain
}

// #endregion wiring

// #region auth-wiring
// newAuth собирает контекст auth: регистрацию (создаёт профиль в user через мост
// userbridge — ADR-0004), вход и JWT. Наружу отдаёт HTTP-адаптер и проверку токенов —
// её использует middleware аутентификации для всех остальных маршрутов.
func newAuth(cfg config, users *usermemory.Store, clk clock.System, ids idgen.UUIDv7, log *slog.Logger) (*authhttp.Handler, *jwt.Verifier, error) {
	registerUser, err := userusecase.NewRegisterUser(users, clk, ids)
	if err != nil {
		return nil, nil, err
	}
	findByUsername, err := userusecase.NewFindByUsername(users)
	if err != nil {
		return nil, nil, err
	}
	bridge, err := userbridge.New(registerUser, findByUsername)
	if err != nil {
		return nil, nil, err
	}
	hasher, err := argon2id.New(cfg.HasherParams)
	if err != nil {
		return nil, nil, err
	}
	creds := authmemory.NewStore()
	signUp, err := authusecase.NewSignUp(hasher, bridge, creds)
	if err != nil {
		return nil, nil, err
	}
	// Подписывает только auth; остальным достаточно открытого ключа.
	signer, err := jwt.NewSigner(cfg.SigningKey, clk, ids)
	if err != nil {
		return nil, nil, err
	}
	verifier, err := jwt.NewVerifier(clk, cfg.SigningKey.Public().(ed25519.PublicKey))
	if err != nil {
		return nil, nil, err
	}
	login, err := authusecase.NewLogin(bridge, creds, hasher, signer)
	if err != nil {
		return nil, nil, err
	}
	api, err := authhttp.New(signUp, login, clk, log)
	if err != nil {
		return nil, nil, err
	}
	return api, verifier, nil
}

// #endregion auth-wiring

// newChat собирает контекст chat. О пользователях он спрашивает через свой порт:
// мост userchecker переводит вопрос в use case контекста user.
func newChat(getUser *userusecase.GetUser, clk clock.System, ids idgen.UUIDv7, log *slog.Logger) (*chathttp.Handler, error) {
	checker, err := userchecker.New(getUser)
	if err != nil {
		return nil, err
	}
	// Одно хранилище chats реализует сразу несколько узких портов.
	chats := chatmemory.NewStore()
	createChat, err := chatusecase.NewCreateChat(chats, chats, checker, clk, ids)
	if err != nil {
		return nil, err
	}
	sendMessage, err := chatusecase.NewSendMessage(chats, chats, clk, ids)
	if err != nil {
		return nil, err
	}
	listMessages, err := chatusecase.NewListMessages(chats, chats)
	if err != nil {
		return nil, err
	}
	listChats, err := chatusecase.NewListChats(chats)
	if err != nil {
		return nil, err
	}
	getChat, err := chatusecase.NewGetChat(chats)
	if err != nil {
		return nil, err
	}
	return chathttp.New(chathttp.UseCases{
		CreateChat:   createChat,
		SendMessage:  sendMessage,
		ListMessages: listMessages,
		ListChats:    listChats,
		GetChat:      getChat,
	}, log)
}

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

// unauthenticated отвечает одинаково на любую причину отказа: клиенту хватит
// «представьтесь» и признака «токен был, но не годится» в WWW-Authenticate (RFC 6750 §3).
// Чем именно не годится — просрочен, чужая подпись, чужой aud — подсказало бы атакующему.
func unauthenticated(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("WWW-Authenticate", httpauth.Challenge(err))
		writeProblem(log, w, r, problem.Problem{Code: problem.CodeUnauthenticated, Detail: "нужен действующий токен в заголовке Authorization: Bearer"})
	}
}

func writeProblem(log *slog.Logger, w http.ResponseWriter, r *http.Request, p problem.Problem) {
	if err := problem.Write(w, r, p); err != nil {
		log.Error("запись ответа", "path", r.URL.Path, "err", err)
	}
}
