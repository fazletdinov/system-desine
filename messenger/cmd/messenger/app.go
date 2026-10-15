package main

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

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
	"messenger/internal/platform/health"
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
	// сети открытым текстом. TLS — на шлюзе (модуль 6.7); в образе (урок 1.16)
	// MESSENGER_ADDR=:8080 задан в Dockerfile.
	defaultAddr = "127.0.0.1:8080"
	addrEnv     = "MESSENGER_ADDR"
	keyFileEnv  = "MESSENGER_JWT_KEY_FILE"
	// timeoutDetail — тот же текст, что problem.Respond отдаёт на DeadlineExceeded:
	// клиенту всё равно, кто заметил истёкший срок — адаптер или middleware.
	timeoutDetail          = "повторите запрос позже"
	methodNotAllowedDetail = "метод не поддерживается для этого пути"
)

// #region config
const (
	defaultRequestTimeout  = 10 * time.Second
	defaultShutdownTimeout = 10 * time.Second
	// defaultDrainDelay — две пробы готовности с периодом по умолчанию (Kubernetes —
	// 10 с, наш compose — 1–2 с) обычно меньше; 5 с — компромисс для compose и kind.
	defaultDrainDelay = 5 * time.Second
	// terminationGrace — сколько Kubernetes (и наш compose) ждёт после SIGTERM до
	// SIGKILL. Дренаж и остановка должны уложиться в него с запасом.
	terminationGrace = 30 * time.Second
)

// durationEnv читает срок из окружения; пустая переменная — значение по умолчанию.
func durationEnv(name string, def time.Duration, allowZero bool) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if d < 0 || (d == 0 && !allowZero) {
		return 0, fmt.Errorf("%s: срок должен быть больше нуля, получено %s", name, raw)
	}
	return d, nil
}

func loadTimeouts(cfg *config) error {
	var err error
	if cfg.RequestTimeout, err = durationEnv("MESSENGER_REQUEST_TIMEOUT", defaultRequestTimeout, false); err != nil {
		return err
	}
	if cfg.ShutdownTimeout, err = durationEnv("MESSENGER_SHUTDOWN_TIMEOUT", defaultShutdownTimeout, false); err != nil {
		return err
	}
	if cfg.DrainDelay, err = durationEnv("MESSENGER_DRAIN_DELAY", defaultDrainDelay, true); err != nil {
		return err
	}
	if total := cfg.DrainDelay + cfg.ShutdownTimeout; total >= terminationGrace {
		return fmt.Errorf("MESSENGER_DRAIN_DELAY + MESSENGER_SHUTDOWN_TIMEOUT = %s: должно быть меньше %s, иначе процесс убьют SIGKILL посреди остановки", total, terminationGrace)
	}
	return nil
}

// #endregion config

// publicRoutes — маршруты, открытые без аутентификации: метод и путь без параметров.
// Всё остальное закрыто: забытый здесь маршрут получит 401, а не откроется всем.
var publicRoutes = []string{"POST /v1/users", "POST /v1/tokens"}

// config — всё, что сборка берёт снаружи. HasherParams меняет только сквозной тест:
// боевые параметры хеширования не настраиваются из окружения.
type config struct {
	Addr         string
	SigningKey   ed25519.PrivateKey
	HasherParams argon2id.Params
	// RequestTimeout — срок обработки одного запроса (middleware.Timeout).
	RequestTimeout time.Duration
	// ShutdownTimeout — сколько ждать начатые запросы после дренажа.
	ShutdownTimeout time.Duration
	// DrainDelay — сколько после SIGTERM ещё принимать запросы, отвечая /readyz 503.
	DrainDelay time.Duration
}

// app — то, что сборка отдаёт main: обработчик и пробы (main переводит их в дренаж по SIGTERM).
type app struct {
	handler http.Handler
	probes  *health.Probes
}

// addrFromEnv — адрес сервера; его же читает подкоманда healthcheck, чтобы постучаться
// в тот порт, который слушает сервер.
func addrFromEnv() string {
	if addr := os.Getenv(addrEnv); addr != "" {
		return addr
	}
	return defaultAddr
}

// loadConfig читает окружение. Без ключа подписи сервер не стартует: сгенерировать
// ключ «на лету» значило бы молча обесценить все токены при каждом перезапуске.
func loadConfig() (config, error) {
	cfg := config{Addr: addrFromEnv(), HasherParams: argon2id.OWASPMinimum}
	if err := loadTimeouts(&cfg); err != nil {
		return config{}, err
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
func newApp(cfg config, log *slog.Logger) (*app, error) {
	if cfg.RequestTimeout <= 0 {
		return nil, errors.New("сборка: нужен таймаут запроса")
	}
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
	// Timeout — внутри Recover, чтобы паника по сроку тоже стала 500; до аутентификации —
	// срок есть у всего, включая проверку токена.
	api := middleware.Chain(
		middleware.RouteErrors(mux, routeError(log)),
		middleware.RequestID(ids),
		middleware.AccessLog(log),
		middleware.Recover(log, internalError(log)),
		middleware.Timeout(cfg.RequestTimeout, timeoutError(log)),
		httpauth.Middleware(bearer, publicRoutes, unauthenticated(log)),
	)
	// #endregion chain

	// #region root-mux
	// Пробы — мимо цепочки: без аутентификации, журнала доступа и таймаута запроса.
	// Оркестратор дёргает их каждые несколько секунд, и журнал на 90 % состоял бы из них.
	probes, err := health.New(log)
	if err != nil {
		return nil, fmt.Errorf("сборка: %w", err)
	}
	root := http.NewServeMux()
	probes.Register(root, methodNotAllowed(log))
	root.Handle("/", api)
	return &app{handler: root, probes: probes}, nil
	// #endregion root-mux
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
			methodNotAllowed(log)(w, r)
			return
		}
		// Пустой шаблон без Allow — «не найдено». Даже редирект на очищенный путь
		// (/v1//x → /v1/x) ServeMux отдаёт с пустым шаблоном, только когда и там пусто.
		writeProblem(log, w, r, problem.Problem{Code: problem.CodeNotFound, Detail: "маршрут не найден"})
	}
}

// methodNotAllowed — 405 и для API, и для проб; заголовок Allow ставит тот, кто знает
// разрешённые методы (ServeMux или health).
func methodNotAllowed(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeProblem(log, w, r, problem.Problem{Code: problem.CodeMethodNotAllowed, Detail: methodNotAllowedDetail})
	}
}

// timeoutError — ответ middleware.Timeout, когда обработчик вернулся по сроку молча.
func timeoutError(log *slog.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		writeProblem(log, w, r, problem.Problem{Code: problem.CodeTimeout, Detail: timeoutDetail})
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
