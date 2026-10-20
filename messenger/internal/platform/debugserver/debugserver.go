package debugserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime"
	"strconv"
	"time"

	"messenger/internal/platform/httpserver"
)

const (
	// MaxProfileDuration — потолок ?seconds у CPU-профиля, трассы и дельта-профилей.
	// Минуты CPU-профиля хватает с запасом (по умолчанию pprof снимает 30 с), а без
	// потолка ?seconds=600 держал бы соединение и профилировщик десять минут.
	MaxProfileDuration = 60 * time.Second
	// StopTimeout — сколько остановка ждёт начатые запросы. Меньше
	// lifecycle.BackStopTimeout (2 с), чтобы уложиться в срок фоновых компонентов:
	// идущий профиль на 30 с обрывается — это дешевле, чем задержать остановку.
	StopTimeout = time.Second
	// MutexProfileFraction — в mutex-профиль попадает в среднем 1 из 100 событий
	// конкуренции за замок: цена — доли процента, а картина за минуту уже видна.
	MutexProfileFraction = 100
	// BlockProfileRate — в block-профиль попадает в среднем одно событие на 10 мкс
	// блокировки (в наносекундах): короткие ожидания каналов не платят за запись
	// стека, а долгие видны все.
	BlockProfileRate = 10_000
)

const (
	// readHeaderTimeout — как у основного сервера (httpserver, урок 0b.6): обрывает
	// Slowloris. Константа httpserver не экспортирована, а повод тот же.
	readHeaderTimeout = 5 * time.Second
	// writeMargin — запас записи сверх самого длинного профиля: профиль на
	// MaxProfileDuration ещё надо отдать клиенту. pprof сам отказывает, если seconds
	// больше WriteTimeout сервера, — наш потолок срабатывает раньше и понятнее.
	writeMargin = 10 * time.Second
	// idleTimeout — простой keep-alive соединения между запросами, как у основного.
	idleTimeout = 60 * time.Second
)

// Snapshotter — снимок flight recorder; порт у потребителя: обработчику /debug/flight
// нужен только WriteTo, а не рекордер целиком.
type Snapshotter interface {
	WriteTo(w io.Writer) (int64, error)
}

// #region debug-handler
// NewHandler — маршруты служебного порта на своём ServeMux. Именованные профили
// (heap, goroutine, goroutineleak, mutex, block, allocs…) отдаёт pprof.Index по пути
// /debug/pprof/<имя>. flight == nil — рекордера нет, /debug/flight отвечает 404.
func NewHandler(flight Snapshotter) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/debug/pprof/", limitSeconds(http.HandlerFunc(pprof.Index)))
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.Handle("/debug/pprof/profile", limitSeconds(http.HandlerFunc(pprof.Profile)))
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.Handle("/debug/pprof/trace", limitSeconds(http.HandlerFunc(pprof.Trace)))
	if flight != nil {
		mux.Handle("GET /debug/flight", flightSnapshot(flight))
	}
	return mux
}

// flightSnapshot отдаёт снимок файлом для go tool trace. Снимок сначала собирается в
// память (до MaxBytes рекордера): ошибка на середине иначе пришла бы после статуса
// 200 и выглядела бы у клиента как обрезанный, но «успешный» файл.
func flightSnapshot(flight Snapshotter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		var buf bytes.Buffer
		if _, err := flight.WriteTo(&buf); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="flight.trace"`)
		_, _ = buf.WriteTo(w) // клиент ушёл — сообщить уже некому
	})
}

// #endregion debug-handler

// #region profile-limit
// limitSeconds отвергает ?seconds вне (0, MaxProfileDuration] до вызова pprof. Читает
// параметр так же, как pprof (FormValue), иначе проверка и обработчик видели бы разные
// значения. Сравнение записано «в положительную сторону»: NaN не проходит ни одно.
func limitSeconds(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if raw := r.FormValue("seconds"); raw != "" {
			sec, err := strconv.ParseFloat(raw, 64)
			if err != nil || !(sec > 0 && sec <= MaxProfileDuration.Seconds()) {
				http.Error(w, fmt.Sprintf("seconds: ожидается число от 0 до %.0f, получено %q", MaxProfileDuration.Seconds(), raw), http.StatusBadRequest)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// #endregion profile-limit

// EnableContentionProfiles включает выборку для профилей mutex и block — по
// умолчанию они пусты. Вызывает composition root, когда служебный порт включён.
func EnableContentionProfiles() {
	runtime.SetMutexProfileFraction(MutexProfileFraction)
	runtime.SetBlockProfileRate(BlockProfileRate)
}

// Server — служебный порт как lifecycle.Component.
type Server struct {
	srv *http.Server
	log *slog.Logger
}

// New собирает сервер с таймаутами; WriteTimeout покрывает самый длинный профиль.
func New(addr string, h http.Handler, log *slog.Logger) (*Server, error) {
	switch {
	case h == nil:
		return nil, errors.New("debugserver: нужен обработчик")
	case log == nil:
		return nil, errors.New("debugserver: нужен журнал")
	}
	return &Server{
		srv: &http.Server{
			Addr:              addr,
			Handler:           h,
			ReadHeaderTimeout: readHeaderTimeout,
			WriteTimeout:      MaxProfileDuration + writeMargin,
			IdleTimeout:       idleTimeout,
			ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
		},
		log: log,
	}, nil
}

func (s *Server) Name() string { return "debug" }

// Run занимает адрес и обслуживает запросы до отмены ctx.
func (s *Server) Run(ctx context.Context) error {
	return s.stopped(ctx, httpserver.Run(ctx, s.srv, httpserver.Stop{Timeout: StopTimeout}))
}

// Serve — то же на готовом слушателе (тесты подставляют сеть в памяти).
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	return s.stopped(ctx, httpserver.Serve(ctx, s.srv, ln, httpserver.Stop{Timeout: StopTimeout}))
}

// stopped переводит итог остановки. Не дождались запросов за StopTimeout — значит,
// оборвали идущий профиль: так и задумано, и это не сбой процесса (иначе снятый в
// момент выкладки профиль превращал бы штатную остановку в код выхода 1).
func (s *Server) stopped(ctx context.Context, err error) error {
	switch {
	case err == nil:
	case ctx.Err() != nil && errors.Is(err, context.DeadlineExceeded):
		s.log.Warn("служебный сервер: незавершённые запросы оборваны", "stop_timeout", StopTimeout.String())
	default:
		return err
	}
	s.log.Info("служебный сервер остановлен")
	return nil
}
