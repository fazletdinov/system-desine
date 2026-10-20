package flightrec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Snapshotter — откуда брать снимок; в сборке это *Recorder. Порт объявлен здесь, у
// потребителя: Dumper не владеет рекордером и не может его остановить.
type Snapshotter interface {
	WriteTo(w io.Writer) (int64, error)
}

// Clock — источник времени для имён файлов и ограничения частоты.
type Clock interface {
	Now() time.Time
}

// fileTimeLayout — время снимка в имени файла: UTC, без двоеточий (их не любят
// файловые системы и инструменты), сортируется как строка.
const fileTimeLayout = "20060102T150405Z"

// Dumper пишет снимок рекордера в каталог, когда запрос оказался медленнее порога.
// Middleware только замечает медленный запрос и подаёт сигнал; файл пишет Run —
// lifecycle.Component с владельцем-lifecycle, так что запись нескольких мегабайт не
// задерживает ответ и не плодит горутин без владельца.
type Dumper struct {
	src       Snapshotter
	dir       string
	threshold time.Duration
	clk       Clock
	log       *slog.Logger
	// trigger — сигнал «был медленный запрос». Ёмкость 1 и неблокирующая отправка:
	// пока Run пишет файл, следующие сигналы схлопываются в один ждущий, а лишние
	// отбрасываются — снимок всё равно покрывает последние Window секунд, а
	// заблокированный на канале обработчик сам стал бы медленным запросом.
	trigger chan struct{}
	// last — время последнего снимка; читает и пишет только горутина Run.
	last time.Time
}

// NewDumper проверяет зависимости и порог: порог в (0, Window), иначе окно рекордера
// к моменту снимка уже забыло начало медленного запроса.
func NewDumper(src Snapshotter, dir string, threshold time.Duration, clk Clock, log *slog.Logger) (*Dumper, error) {
	switch {
	case src == nil:
		return nil, errors.New("flightrec: нужен источник снимков")
	case dir == "":
		return nil, errors.New("flightrec: нужен каталог для снимков")
	case threshold <= 0 || threshold >= Window:
		return nil, fmt.Errorf("flightrec: порог должен быть больше нуля и меньше окна %s, получено %s", Window, threshold)
	case clk == nil:
		return nil, errors.New("flightrec: нужны часы")
	case log == nil:
		return nil, errors.New("flightrec: нужен журнал")
	}
	return &Dumper{src: src, dir: dir, threshold: threshold, clk: clk, log: log, trigger: make(chan struct{}, 1)}, nil
}

// #region flight-middleware
// Middleware замеряет запрос целиком и после медленного ответа подаёт сигнал Run.
// Длительность — по time.Now, а не по Clock: монотонные часы не прыгают при
// коррекции системного времени, а Clock отдаёт время без монотонной части (UTC).
func (d *Dumper) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if time.Since(start) <= d.threshold {
			return
		}
		select {
		case d.trigger <- struct{}{}:
		default: // сигнал уже ждёт Run — этот ничего не добавит
		}
	})
}

// #endregion flight-middleware

func (d *Dumper) Name() string { return "flight-dumper" }

// #region flight-dumper
// Run ждёт сигналов о медленных запросах и пишет снимок в <dir>/flight-<UTC>.trace не
// чаще раза в MinInterval; по отмене ctx возвращает nil. Ошибка записи — Warn в
// журнал: снимок — диагностика, и полный диск не повод ронять сервис.
func (d *Dumper) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-d.trigger:
		}
		now := d.clk.Now()
		if !d.last.IsZero() && now.Sub(d.last) < MinInterval {
			continue
		}
		// Отметка — и при неудаче: иначе каждый медленный запрос снова бился бы в
		// полный диск.
		d.last = now
		if err := d.dump(now); err != nil {
			d.log.Warn("снимок flight recorder не записан", "err", err)
		}
	}
}

// dump пишет один снимок; недописанный файл удаляет — обрезанная трасса не читается
// go tool trace и только сбила бы с толку.
func (d *Dumper) dump(now time.Time) (err error) {
	path := filepath.Join(d.dir, "flight-"+now.UTC().Format(fileTimeLayout)+".trace")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("flightrec: %w", err)
	}
	n, err := d.src.WriteTo(f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return errors.Join(fmt.Errorf("flightrec: запись %s: %w", path, err), os.Remove(path))
	}
	d.log.Info("снимок flight recorder записан", "file", path, "bytes", n)
	return nil
}

// #endregion flight-dumper
