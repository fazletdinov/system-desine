package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// Component — долгоживущая часть процесса: HTTP-сервер, фоновая задача.
type Component interface {
	Name() string
	// Run работает до отмены ctx и тогда возвращает nil; ошибка до отмены — сбой.
	// После отмены context.Canceled (идиома «вернуть ctx.Err()») тоже считается
	// штатной остановкой; любая другая ошибка — сбой, даже после отмены.
	Run(ctx context.Context) error
}

// Stages — компоненты в порядке остановки: сначала все Front, затем все Back.
//
// Run не ограничивает срок остановки Front: каждый Front обязан ограничить его сам
// (HTTP — через httpserver.Stop.Timeout), иначе зависший Front держит остановку
// до SIGKILL. Срок Back ограничивает Run — BackStopTimeout.
type Stages struct{ Front, Back []Component }

// BackStopTimeout — срок выхода Back после их отмены. Фоновой задаче достаточно
// закончить текущую итерацию; 2 с — с запасом для очистки в памяти и коротких
// запросов в базу, и при этом вместе с DRAIN_DELAY + SHUTDOWN_TIMEOUT укладывается
// в grace period платформы (это проверяет loadTimeouts при старте). Не уложившийся
// Back продолжает работать и после возврата Run — см. контракт Run.
const BackStopTimeout = 2 * time.Second

// ErrStoppedEarly — компонент вернул nil, хотя его не останавливали: его работа
// прекратилась молча, а процесс продолжил бы жить без неё.
var ErrStoppedEarly = errors.New("завершился до остановки")

// #region lifecycle-run
// Run запускает компоненты и ждёт остановки: по отмене ctx (сигнал) или по сбою
// любого компонента. Порядок остановки один и тот же в обоих случаях:
//
//  1. Front получают отмену и выходят (HTTP проходит дренаж и Shutdown из 1.17);
//  2. только после этого Back получают отмену и обязаны выйти за BackStopTimeout.
//
// Почему Front раньше Back: начатые запросы могут опираться на фоновые компоненты
// (очередь, кеш, очистка), а фоновые компоненты на запросы — нет. Останови их
// одновременно — и последние запросы дренажа увидели бы уже остановленный фон.
//
// Возвращает первую по времени ошибку компонента (с его именем) или ошибку срока;
// nil — штатная остановка, даже если ctx отменён.
//
// Контракт для вызывающего: если Run вернул ошибку срока, зависший Back (и горутина,
// которая ждёт его выхода) переживает Run — остановить горутину снаружи в Go нельзя.
// Поэтому после ошибки Run вызывающий обязан завершить процесс (main — os.Exit(1)),
// а не запускать компоненты заново.
func Run(ctx context.Context, log *slog.Logger, s Stages) error {
	// failCtx отменяется сигналом (через ctx) или сбоем любого компонента — это
	// команда «останавливаемся» для Front.
	failCtx, fail := context.WithCancel(ctx)
	defer fail()
	// Back не наследуют отмену ctx: их отменяет только stopBack после выхода Front.
	// WithoutCancel оставляет значения ctx (трассировка, логгер), но не отмену.
	backCtx, stopBack := context.WithCancel(context.WithoutCancel(ctx))
	defer stopBack()

	var first firstError
	var front, back errgroup.Group
	backRunning := &running{names: make(map[string]int)}
	// start — каждый компонент сообщает о сбое сам, в момент сбоя: первая ошибка
	// по времени — причина остановки, а не следствие (например, ошибка Shutdown).
	start := func(g *errgroup.Group, ctx context.Context, c Component, onExit func()) {
		g.Go(func() error {
			defer onExit()
			err := c.Run(ctx)
			switch {
			case err == nil && ctx.Err() == nil:
				err = ErrStoppedEarly
			case errors.Is(err, context.Canceled) && ctx.Err() != nil:
				// Компонент вернул ошибку своей же отмены — это штатная остановка.
				err = nil
			}
			if err != nil {
				err = fmt.Errorf("компонент %s: %w", c.Name(), err)
				log.Error("сбой компонента", "component", c.Name(), "err", err)
				first.set(err)
				fail()
			}
			return err
		})
	}
	for _, c := range s.Front {
		start(&front, failCtx, c, func() {})
	}
	for _, c := range s.Back {
		backRunning.add(c.Name())
		start(&back, backCtx, c, func() { backRunning.remove(c.Name()) })
	}

	// Сначала — команда остановиться (сигнал или сбой), потом — выход всех Front.
	// Ждать только Front мало: без Front их Wait вернулся бы сразу, и Back
	// остановились бы на старте. Ошибки уже записаны в first: Wait нужен как «все вышли».
	<-failCtx.Done()
	_ = front.Wait()
	stopBack()

	backDone := make(chan struct{})
	// Владелец — Run; горутина выходит, когда выйдут все Back. Если какой-то Back
	// завис, она переживает Run — поэтому после ошибки Run процесс завершается.
	go func() { _ = back.Wait(); close(backDone) }()
	timer := time.NewTimer(BackStopTimeout)
	defer timer.Stop()
	select {
	case <-backDone:
		log.Info("фоновые компоненты остановлены")
		return first.get()
	case <-timer.C:
		return errors.Join(first.get(), fmt.Errorf("компоненты %s не остановились за %s после отмены", backRunning.list(), BackStopTimeout))
	}
}

// #endregion lifecycle-run

// firstError запоминает первую ошибку: её Run и вернёт. Замок, а не sync.Once:
// зависший Back может сообщить о сбое уже после того, как Run прочитал ошибку.
type firstError struct {
	mu  sync.Mutex // защищает err
	err error
}

func (f *firstError) set(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err == nil {
		f.err = err
	}
}

func (f *firstError) get() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

// running — имена Back, которые ещё не вышли: их называет ошибка срока. Счётчик,
// а не множество: два компонента с одним именем не должны «вычеркнуть» друг друга.
type running struct {
	mu    sync.Mutex // защищает names
	names map[string]int
}

func (r *running) add(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names[name]++
}

func (r *running) remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.names[name]--; r.names[name] == 0 {
		delete(r.names, name)
	}
}

func (r *running) list() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.names))
	for n := range r.names {
		names = append(names, n)
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}
