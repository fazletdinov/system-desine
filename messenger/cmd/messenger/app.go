package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"messenger/internal/auth/adapters/argon2id"
	"messenger/internal/auth/adapters/hashlimit"
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
	"messenger/internal/platform/debugserver"
	"messenger/internal/platform/flightrec"
	"messenger/internal/platform/health"
	"messenger/internal/platform/httpauth"
	"messenger/internal/platform/idempotency"
	idemmemory "messenger/internal/platform/idempotency/memory"
	"messenger/internal/platform/idgen"
	"messenger/internal/platform/lifecycle"
	"messenger/internal/platform/middleware"
	"messenger/internal/platform/periodic"
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
	// timeoutDetail — тот же текст, что problem.Respond отдаёт на DeadlineExceeded:
	// клиенту всё равно, кто заметил истёкший срок — адаптер или middleware.
	timeoutDetail          = "повторите запрос позже"
	methodNotAllowedDetail = "метод не поддерживается для этого пути"
)

// Переменные окружения — константами: имя повторяется в чтении, в тексте ошибки и в
// тестах, и опечатка в литерале молча оставила бы значение по умолчанию.
const (
	envAddr                  = "MESSENGER_ADDR"
	envJWTKeyFile            = "MESSENGER_JWT_KEY_FILE"
	envRequestTimeout        = "MESSENGER_REQUEST_TIMEOUT"
	envShutdownTimeout       = "MESSENGER_SHUTDOWN_TIMEOUT"
	envDrainDelay            = "MESSENGER_DRAIN_DELAY"
	envIdempotencySweepEvery = "MESSENGER_IDEMPOTENCY_SWEEP_EVERY"
	envHashConcurrency       = "MESSENGER_HASH_CONCURRENCY"
	envHashQueue             = "MESSENGER_HASH_QUEUE"
	envDebugAddr             = "MESSENGER_DEBUG_ADDR"
	envFlightDir             = "MESSENGER_FLIGHT_DIR"
	envFlightThreshold       = "MESSENGER_FLIGHT_THRESHOLD"
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
	if cfg.RequestTimeout, err = durationEnv(envRequestTimeout, defaultRequestTimeout, false); err != nil {
		return err
	}
	if cfg.ShutdownTimeout, err = durationEnv(envShutdownTimeout, defaultShutdownTimeout, false); err != nil {
		return err
	}
	if cfg.DrainDelay, err = durationEnv(envDrainDelay, defaultDrainDelay, true); err != nil {
		return err
	}
	// После HTTP останавливаются фоновые компоненты — их срок тоже в бюджете остановки.
	if total := cfg.DrainDelay + cfg.ShutdownTimeout + lifecycle.BackStopTimeout; total >= terminationGrace {
		return fmt.Errorf("%s + %s + срок фоновых компонентов %s = %s: должно быть меньше %s, иначе процесс убьют SIGKILL посреди остановки",
			envDrainDelay, envShutdownTimeout, lifecycle.BackStopTimeout, total, terminationGrace)
	}
	return nil
}

// #endregion config

const (
	// defaultIdempotencySweepEvery — период фоновой очистки ключей идемпотентности.
	// Ключ живёт сутки (idempotency.TTL), так что лишняя минута жизни просроченной
	// записи ничего не стоит, а проход по карте раз в минуту — копейки CPU.
	defaultIdempotencySweepEvery = time.Minute
	// sweepJitter — разброс периода очистки ±10 %: экземпляры, стартовавшие вместе
	// после выкладки, не чистят общее хранилище (Postgres в 2a) одновременно.
	sweepJitter = 0.1
	// defaultHashQueueFactor — очередь хешера по умолчанию: 4 ждущих на каждое место
	// вычисления. При ≈ 18 мс на хеш в одиночку (до ~40 мс при N параллельных, урок 1.5.6) последний в очереди ждёт порядка 4 × 40 мс — заметно
	// меньше срока запроса, а всплеск входов сверх этого получает 503 сразу, а не
	// копит горутины и память.
	defaultHashQueueFactor = 4
	// maxHashConcurrency — потолок одновременных хешей: 256 × 19 МиБ ≈ 4,75 ГиБ памяти
	// только на хеши — больше не бывает осмысленным ни на одной машине, а опечатка
	// в переменной (лишние нули) иначе дала бы переполнение очереди или OOM.
	maxHashConcurrency = 256
	// maxHashQueue — потолок очереди: те же 4 ждущих на место при максимуме мест.
	// Каждый ждущий — горутина с запросом; бесконечная очередь — та же перегрузка.
	maxHashQueue = defaultHashQueueFactor * maxHashConcurrency
)

// intEnv читает целое из окружения в пределах [minimum, maximum]; пустая переменная —
// значение по умолчанию.
func intEnv(name string, def, minimum, maximum int) (int, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: ожидается целое число, получено %q", name, raw)
	}
	if n < minimum || n > maximum {
		return 0, fmt.Errorf("%s: должно быть от %d до %d, получено %d", name, minimum, maximum, n)
	}
	return n, nil
}

// loadConcurrency читает период фоновой очистки и ограничение хеширования.
func loadConcurrency(cfg *config) error {
	var err error
	if cfg.IdempotencySweepEvery, err = durationEnv(envIdempotencySweepEvery, defaultIdempotencySweepEvery, false); err != nil {
		return err
	}
	// Хеш — чистая работа CPU: больше GOMAXPROCS одновременных вычислений не ускорят,
	// а память (19 МиБ на хеш) умножат.
	// Умолчание тоже под потолком: GOMAXPROCS бывает больше 256 на крупных машинах.
	if cfg.HashConcurrency, err = intEnv(envHashConcurrency, min(runtime.GOMAXPROCS(0), maxHashConcurrency), 1, maxHashConcurrency); err != nil {
		return err
	}
	// Очередь 0 допустима: «все места заняты — отказ сразу».
	if cfg.HashQueue, err = intEnv(envHashQueue, defaultHashQueueFactor*cfg.HashConcurrency, 0, maxHashQueue); err != nil {
		return err
	}
	return nil
}

// defaultFlightThreshold — запрос дольше секунды при сроке в 10 с уже ненормален
// (типичный ответ — миллисекунды), а запас до окна рекордера (10 с) оставляет в снимке
// и сам запрос, и то, что происходило до него.
const defaultFlightThreshold = time.Second

// loadDebug читает служебный порт и снимки flight recorder. По умолчанию всё выключено:
// pprof раскрывает стеки и командную строку, и открывать его — явное решение (ADR-0013).
func loadDebug(cfg *config) error {
	cfg.DebugAddr = os.Getenv(envDebugAddr)
	if cfg.DebugAddr != "" {
		if err := checkListenAddr(cfg.DebugAddr); err != nil {
			return fmt.Errorf("%s: %w", envDebugAddr, err)
		}
	}
	cfg.FlightDir = os.Getenv(envFlightDir)
	if cfg.FlightDir != "" {
		// Рекордер запускается вместе со служебным портом: без него снимать нечего.
		if cfg.DebugAddr == "" {
			return fmt.Errorf("%s требует %s: flight recorder работает, только когда включён служебный порт", envFlightDir, envDebugAddr)
		}
		info, err := os.Stat(cfg.FlightDir)
		if err != nil {
			return fmt.Errorf("%s: %w", envFlightDir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s: %s — не каталог", envFlightDir, cfg.FlightDir)
		}
	}
	var err error
	if cfg.FlightThreshold, err = durationEnv(envFlightThreshold, defaultFlightThreshold, false); err != nil {
		return err
	}
	if cfg.FlightThreshold >= flightrec.Window {
		return fmt.Errorf("%s: порог должен быть меньше %s (окно flight recorder), получено %s", envFlightThreshold, flightrec.Window, cfg.FlightThreshold)
	}
	return nil
}

// checkListenAddr — host:port с числовым портом 1–65535. SplitHostPort один пропустил
// бы "localhost:" (сервер молча занял бы случайный порт, и профили искали бы не там)
// и ":abc" (ошибка всплыла бы только при запуске слушателя).
func checkListenAddr(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("ожидается host:port или :port: %w", err)
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("порт должен быть числом от 1 до 65535, получено %q", port)
	}
	return nil
}

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
	// IdempotencySweepEvery — период фоновой очистки ключей идемпотентности.
	IdempotencySweepEvery time.Duration
	// HashConcurrency — сколько хешей паролей считать одновременно.
	HashConcurrency int
	// HashQueue — сколько вызовов хеширования ждут места; сверх — 503 overloaded.
	HashQueue int
	// DebugAddr — адрес служебного порта (pprof, /debug/flight); пусто — выключен.
	DebugAddr string
	// FlightDir — каталог снимков flight recorder по медленным запросам; пусто — не писать.
	FlightDir string
	// FlightThreshold — с какой длительности запрос считается медленным.
	FlightThreshold time.Duration
}

// app — то, что сборка отдаёт main: обработчик, пробы (main переводит их в дренаж по
// SIGTERM) и фоновые компоненты — их main запускает и останавливает через lifecycle.
type app struct {
	handler http.Handler
	probes  *health.Probes
	back    []lifecycle.Component
	// recorder — flight recorder процесса; nil, если служебный порт выключен. Его
	// останавливает main после остановки компонентов.
	recorder *flightrec.Recorder
}

// addrFromEnv — адрес сервера; его же читает подкоманда healthcheck, чтобы постучаться
// в тот порт, который слушает сервер.
func addrFromEnv() string {
	if addr := os.Getenv(envAddr); addr != "" {
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
	if err := loadConcurrency(&cfg); err != nil {
		return config{}, err
	}
	if err := loadDebug(&cfg); err != nil {
		return config{}, err
	}
	path := os.Getenv(envJWTKeyFile)
	if path == "" {
		return config{}, errors.New("задайте " + envJWTKeyFile + " — ключ создаёт go run ./cmd/keygen -out jwt.pem")
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
func newApp(cfg config, log *slog.Logger) (_ *app, err error) {
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
	sweeper, err := newIdempotencySweeper(idemStore, clk, cfg.IdempotencySweepEvery, log)
	if err != nil {
		return nil, fmt.Errorf("сборка: %w", err)
	}

	bearer, err := httpauth.NewBearer(verifier)
	if err != nil {
		return nil, fmt.Errorf("сборка: %w", err)
	}
	dbg, err := newDebug(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("сборка: %w", err)
	}
	// Рекордер уже пишет трассу: если сборка дальше не удалась, его некому остановить.
	defer func() {
		if err != nil && dbg.recorder != nil {
			dbg.recorder.Stop()
		}
	}()
	// #region chain
	// Первый — самый внешний. Замер медленных запросов (dbg.outer, если включён) —
	// снаружи всего: длительность всего запроса, с журналом и аутентификацией. Request ID
	// нужен всем, кто пишет в журнал; Recover — внутри AccessLog, чтобы паника попала в
	// журнал как 500; аутентификация — до маршрутизатора. Timeout — внутри Recover, чтобы
	// паника по сроку тоже стала 500; до аутентификации — срок есть у всего, включая
	// проверку токена.
	api := middleware.Chain(
		middleware.RouteErrors(mux, routeError(log)),
		append(dbg.outer,
			middleware.RequestID(ids),
			middleware.AccessLog(log),
			middleware.Recover(log, internalError(log)),
			middleware.Timeout(cfg.RequestTimeout, timeoutError(log)),
			httpauth.Middleware(bearer, publicRoutes, unauthenticated(log)),
		)...,
	)
	// #endregion chain

	// #region root-mux
	// Пробы — мимо цепочки: без аутентификации, журнала доступа и таймаута запроса.
	// Оркестратор дёргает их каждые несколько секунд, и журнал на 90 % состоял бы из них.
	probes, err := health.New(log)
	if err != nil {
		return nil, fmt.Errorf("сборка: %w", err)
	}
	probeMux := http.NewServeMux()
	probes.Register(probeMux, methodNotAllowed(log))
	// Развилка — по точному пути, а не общим ServeMux с маршрутом "/": такой mux
	// отвечает голым 307 на любой «нечистый» путь (/v1//x) ещё до цепочки — без
	// request ID, журнала и problem+json. Всё, что не проба, целиком решает API.
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case health.LivePath, health.ReadyPath:
			probeMux.ServeHTTP(w, r)
		default:
			api.ServeHTTP(w, r)
		}
	})
	return &app{handler: root, probes: probes, back: append([]lifecycle.Component{sweeper}, dbg.back...), recorder: dbg.recorder}, nil
	// #endregion root-mux
}

// #endregion wiring

// #region debug-wiring
// debugParts — служебный порт и flight recorder, собранные для newApp.
type debugParts struct {
	recorder *flightrec.Recorder
	back     []lifecycle.Component   // служебный сервер и запись снимков — в фоновой стадии
	outer    []middleware.Middleware // замер медленных запросов — снаружи цепочки API
}

// newDebug собирает служебный порт, если задан его адрес. Служебный сервер — Back:
// основной HTTP дренируется первым, а профиль зависшей остановки ещё можно снять.
// Рекордер — один на процесс: его WriteTo получают и /debug/flight, и Dumper.
func newDebug(cfg config, log *slog.Logger) (_ debugParts, err error) {
	if cfg.DebugAddr == "" {
		return debugParts{}, nil
	}
	debugserver.EnableContentionProfiles()
	rec, err := flightrec.Start()
	if err != nil {
		return debugParts{}, err
	}
	defer func() {
		if err != nil {
			rec.Stop()
		}
	}()
	srv, err := debugserver.New(cfg.DebugAddr, debugserver.NewHandler(rec), log)
	if err != nil {
		return debugParts{}, err
	}
	parts := debugParts{recorder: rec, back: []lifecycle.Component{srv}}
	if cfg.FlightDir == "" {
		return parts, nil
	}
	dumper, err := flightrec.NewDumper(rec, cfg.FlightDir, cfg.FlightThreshold, clock.System{}, log)
	if err != nil {
		return debugParts{}, err
	}
	parts.back = append(parts.back, dumper)
	parts.outer = []middleware.Middleware{dumper.Middleware}
	return parts, nil
}

// #endregion debug-wiring

// newIdempotencySweeper — фоновая очистка просроченных ключей идемпотентности. Без неё
// ключ, к которому клиент не вернулся, жил бы в памяти вечно.
func newIdempotencySweeper(store idempotency.Sweeper, clk idempotency.Clock, every time.Duration, log *slog.Logger) (*periodic.Task, error) {
	return periodic.New(periodic.Config{
		Name:   "idempotency-sweeper",
		Every:  every,
		Jitter: sweepJitter,
		Log:    log,
		Fn: func(ctx context.Context) error {
			removed, err := store.Sweep(ctx, clk.Now())
			if err != nil {
				return fmt.Errorf("очистка ключей идемпотентности: %w", err)
			}
			log.Debug("очистка ключей идемпотентности", "removed", removed)
			return nil
		},
	})
}

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
	argon, err := argon2id.New(cfg.HasherParams)
	if err != nil {
		return nil, nil, err
	}
	// Ограничитель — один на регистрацию и вход: оба считают одинаково дорогой хеш.
	hasher, err := hashlimit.New(argon, cfg.HashConcurrency, cfg.HashQueue)
	if err != nil {
		return nil, nil, err
	}
	creds := authmemory.NewStore()
	// memory.Tx не откатывает — хранилища в памяти не транзакционны; атомарность
	// регистрации появится с Postgres и platform/pgtx (урок 2.9).
	signUp, err := authusecase.NewSignUp(hasher, bridge, bridge, creds, authmemory.Tx{})
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
	// memory.Tx — без отката, как и в регистрации: до Postgres атомарность попытки
	// держит мьютекс хранилища.
	sendMessage, err := chatusecase.NewSendMessage(chats, chats, chats, chatmemory.Tx{}, clk, ids)
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
		// Пустой шаблон без Allow — «не найдено». На «нечистый» путь ServeMux API отвечает
		// редиректом, только если маршрут есть по очищенному пути (/v1//chats → /v1/chats);
		// если и там пусто (/v1//x), шаблон пустой — и сюда приходит 404.
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
