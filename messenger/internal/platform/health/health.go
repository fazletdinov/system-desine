package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

const (
	LivePath  = "/livez"
	ReadyPath = "/readyz"
	// readyTimeout — срок всех проверок готовности вместе. Kubernetes по умолчанию ждёт
	// ответа пробы 1 с (timeoutSeconds): отвечать дольше бесполезно — проба уже провалена.
	readyTimeout = time.Second
)

// Checker — зависимость, без которой экземпляр не может обслуживать запросы
// (Postgres). Check обязан уважать ctx.
type Checker interface {
	Check(ctx context.Context) error
}

// CheckerFunc — функция как Checker (как http.HandlerFunc): composition root собирает
// проверку из замыкания над пулом, и health не знает, что проверяет.
type CheckerFunc func(ctx context.Context) error

func (f CheckerFunc) Check(ctx context.Context) error { return f(ctx) }

// Check — проверка с именем: имя попадает в ответ /readyz, причина — только в журнал.
type Check struct {
	Name    string
	Checker Checker
}

// #region probes
// Probes отвечает на пробы. draining поднимается по SIGTERM: с этого момента /readyz
// отвечает 503 и балансировщик выводит экземпляр из ротации, пока тот ещё дообслуживает
// начатые запросы.
type Probes struct {
	log      *slog.Logger
	checks   []Check
	draining atomic.Bool
}

func New(log *slog.Logger, checks ...Check) (*Probes, error) {
	if log == nil {
		return nil, errors.New("health: нужен журнал")
	}
	seen := make(map[string]bool, len(checks))
	for _, c := range checks {
		if c.Name == "" || c.Checker == nil {
			return nil, errors.New("health: у проверки нужны имя и Checker")
		}
		if seen[c.Name] {
			return nil, fmt.Errorf("health: проверка %q указана дважды", c.Name)
		}
		seen[c.Name] = true
	}
	return &Probes{log: log, checks: checks}, nil
}

// StartDraining переводит экземпляр в «останавливаюсь»: обратного перехода нет.
func (p *Probes) StartDraining() { p.draining.Store(true) }

// live не проверяет зависимостей: если база недоступна, перезапуск процесса её не
// починит, а одновременный перезапуск всех экземпляров превратит сбой базы в отказ
// всего сервиса. Живость — «процесс отвечает», не больше.
func (p *Probes) live(w http.ResponseWriter, _ *http.Request) {
	p.write(w, http.StatusOK, status{Status: "ok"})
}

func (p *Probes) ready(w http.ResponseWriter, r *http.Request) {
	if failed := p.failures(r.Context()); len(failed) > 0 {
		p.write(w, http.StatusServiceUnavailable, status{Status: "unavailable", Checks: failed})
		return
	}
	p.write(w, http.StatusOK, status{Status: "ok"})
}

// failures возвращает имена непрошедших проверок. Проверки идут по очереди: их единицы,
// а параллельный запуск усложнил бы код без выигрыша.
func (p *Probes) failures(ctx context.Context) map[string]string {
	if p.draining.Load() {
		return map[string]string{"shutdown": "draining"}
	}
	ctx, cancel := context.WithTimeout(ctx, readyTimeout)
	defer cancel()
	failed := map[string]string{}
	for _, c := range p.checks {
		if err := c.Checker.Check(ctx); err != nil {
			// В тексте ошибки бывают адреса и строки подключения: наружу — только имя.
			p.log.Warn("проверка готовности не прошла", "check", c.Name, "err", err)
			failed[c.Name] = "fail"
		}
	}
	return failed
}

// #endregion probes

type status struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// Register вешает пробы на mux. notAllowed отвечает на остальные методы: формат
// ошибок задаёт composition root, как у middleware.
func (p *Probes) Register(mux *http.ServeMux, notAllowed http.HandlerFunc) {
	mux.Handle(LivePath, onlyGet(http.HandlerFunc(p.live), notAllowed))
	mux.Handle(ReadyPath, onlyGet(http.HandlerFunc(p.ready), notAllowed))
}

func onlyGet(h http.Handler, notAllowed http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			notAllowed(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (p *Probes) write(w http.ResponseWriter, code int, s status) {
	w.Header().Set("Content-Type", "application/json")
	// Состояние меняется в любой момент: ни один кэш по пути не должен его запомнить.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(s); err != nil {
		p.log.Warn("запись ответа пробы", "err", err)
	}
}
