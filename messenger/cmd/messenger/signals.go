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
// Почему не «stop() от NotifyContext возвращает поведение по умолчанию»: процесс умер
// бы молча и с кодом, который зависит от окружения, — вне контейнера от сигнала
// (130/143), а в роли PID 1 рантайм Go 1.27 выходит сам с exit(128+sig), Go 1.26 —
// с exit(2), неотличимым от паники. Здесь — строка в журнале и всегда код 1.
// os.Exit не выполняет defer: незавершённый дренаж бросается намеренно, этого второй
// сигнал и просит.
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
