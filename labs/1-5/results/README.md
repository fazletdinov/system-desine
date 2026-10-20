# Сырые выводы уроков 1.5.7–1.5.10

Настоящие выводы программ и команд, из которых взяты числа уроков 1.5.7–1.5.11. Уроки показывают
выдержки и ссылаются сюда за полным текстом. Снято 2026-10-20 на одной машине (`machine.txt`):
Intel Core i5-12400 (6 ядер, 12 логических процессоров), Ubuntu 24.04, ядро Linux 7.0, go1.27.2;
профили мессенджера — в контейнере стенда `messenger/compose.yaml` (Docker 29.9.1, `cpus: 1`,
`mem_limit: 256m`). На другой машине числа будут другими: сравнивайте порядок величины и
соотношения, а не цифры.

```
results/
├── machine.txt          # описание машины
├── netpoll-*.txt        # урок 1.5.7: strace, цена соединения bufio/lazy, Slab, начальный стек
├── escape-*.txt         # урок 1.5.8: вывод -gcflags=-m, снимок для escape-lab, аллокации рантайма
├── alloc-*.txt          # урок 1.5.8: бенчмарки аллокаций и benchstat, рост стека
├── gctrace-*.txt        # урок 1.5.9: GOGC и GOMEMLIMIT
├── limiter*.txt         # урок 1.5.9: ограничитель CPU сборщика
├── greentea-*.txt       # урок 1.5.9: Green Tea против прежней разметки
├── pgo-*.txt            # урок 1.5.10: бенчмарк и программа без профиля и с default.pgo
└── profile/             # урок 1.5.10: профили и трассы мессенджера под нагрузкой
    ├── *-load.txt, *-overload.txt, *-read-load.txt   # как и когда снята нагрузка, итоги скриптов
    ├── *-top*.txt, *-traces.txt, *-peek*.txt, list-*.txt  # вывод go tool pprof
    ├── *-trace-summary.txt                            # scripts/trace-summary.sh по трассам
    ├── cost-attribution*.txt, cost-debug-port.txt     # цена служебного порта на стенде
    └── cost-bench/                                    # та же цена внутри процесса (benchstat)
```

`profile/cost-bench/` — дополнительный замер, в уроки не вошедший: бенчмарк внутри процесса
(`zz_cost_bench_test.go.txt`, временно клался в `messenger/cmd/messenger/`) и временная правка
`app.go`, которая включала компоненты по отдельности (`variant-app.diff`). Оба файла — текст для
чтения, в сборку они не входят.

## Чего здесь нет и как это получить

В репозиторий не положены двоичные файлы и один большой текст:

- профили `*.pprof` (`cpu`, `heap`, `block`, `mutex`, `goroutine`, `signup-*`, `near-limit-*`,
  `mixed-sched`) — снимаются со служебного порта стенда, например
  `curl -o cpu.pprof '127.0.0.1:6060/debug/pprof/profile?seconds=20'` (урок 1.5.10, раздел с нагрузкой:
  `scripts/read-load.sh 8 30` и `scripts/overload.sh 50` из `messenger/`, как записано в `profile/*-load.txt`);
- трассы `mixed-trace.out`, `near-limit-trace.out` (`/debug/pprof/trace?seconds=5`) и снимок
  flight recorder `flight.trace` (`GET /debug/flight`) — 0,3–3,5 МБ каждая;
- `profile/cpu-traces.txt` (2 МБ) — это `go tool pprof -traces cpu.pprof`; из него программа
  `labs/1-5/fold` строит `course/viz/data/flame-messenger.json`.

Текстовые выводы программ лабы воспроизводятся командами из `labs/1-5/README.md` (из каталога
`labs/`, Go 1.27).
