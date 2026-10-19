package main

import (
	"log/slog"
	"os"
)

// #region second-signal
// exitOnSecondSignal завершает процесс с кодом 1 на втором SIGINT/SIGTERM, пока идёт
// остановка: человек нажал Ctrl+C дважды или оркестратор торопит — ждать дренаж
// дальше не нужно. Так же делает SetupSignalHandler в Kubernetes.
//
// sig получает каждый сигнал, включая первый: signal.Notify раздаёт сигнал всем
// подписанным каналам, и первый обрабатывает NotifyContext — он закрывает first.
// Порядок раздачи не определён, поэтому копия первого сигнала приходит в sig то до,
// то после закрытия first. Выход — только на втором сигнале в sig и только после first.
//
// Почему не «stop() от NotifyContext возвращает обработчик по умолчанию»: вне
// контейнера процесс умер бы от сигнала (код 130/143), а в контейнере мессенджер —
// PID 1, и ядро не доставляет PID 1 сигналы с обработчиком по умолчанию — второй
// SIGTERM просто пропал бы. os.Exit не выполняет defer: незавершённый дренаж
// бросается намеренно, этого второй сигнал и просит.
//
// Горутина выходит, когда закрывают sig (run — по defer, после signal.Stop).
func exitOnSecondSignal(sig <-chan os.Signal, first <-chan struct{}, exit func(int), log *slog.Logger) {
	received, stopping := 0, false
	for received < 2 || !stopping {
		select {
		case _, ok := <-sig:
			if !ok {
				return
			}
			received++
		case <-first:
			stopping = true
			first = nil // закрытый канал готов всегда; nil выключает эту ветку select
		}
	}
	log.Error("второй сигнал: выхожу немедленно")
	exit(1)
}

// #endregion second-signal
