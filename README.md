# System Design на практике

Личный курс по проектированию высоконагруженных систем на Go: сайт курса с пошаговыми визуализациями (`course/`) и сквозной проект-мессенджер (`messenger/`, появится в модуле 1).

## Запуск курса

```bash
cd course
npm install
npm run dev        # http://localhost:5173
```

## Проверки

```bash
npm test           # Vitest: логика плеера, сценарии, расчёты, глоссарий, сайдбар
npm run typecheck  # vue-tsc
npm run build      # сборка; падает на неизвестном термине <T id="...">
npm run check:go   # компилирует Go-фрагменты уроков, помеченные <!-- go-check -->
npm run check:labs # gofmt, go vet, go test -race лабораторных и compose config
```

## Лабораторные

Практика модулей лежит в `labs/` — отдельный Go-модуль (`labs/go.mod`, Go 1.26, без внешних зависимостей). Каталог модуля 0b:

- `labs/0b/compose.yaml` и `Dockerfile` — стенд из трёх контейнеров в сети `172.28.0.0/24`: `client`, `server` и `dns` (CoreDNS с зоной `messenger.example`); в образ собраны программы лабораторной и инструменты (`tcpdump`, `ss`, `dig`, `curl` с HTTP/3, `openssl`, `strace`);
- `labs/0b/netem.sh 40ms 1%` — задержка и потери через netem на `client` и `server` (половина RTT на каждом конце), `labs/0b/netem.sh off` — снять; нужен модуль ядра `sch_netem`;
- `labs/0b/cmd/` — программы (`tcpecho`, `udpping`, `httpbench`, `h2server`, `tlsdemo`), `labs/0b/captures/` — записи `.pcap` и `ca.pem` (в git не попадают);
- `labs/0b/README.md` — все команды по урокам сверху вниз и результаты на нашем стенде.

```bash
docker compose -f labs/0b/compose.yaml up -d --build
alias lab='docker compose -f labs/0b/compose.yaml'
labs/0b/netem.sh 40ms 0%
# … команды урока …
labs/0b/netem.sh off
docker compose -f labs/0b/compose.yaml down
```

Проверка — из `course/`: `npm run check:labs` (gofmt, `go vet`, `go test -race` по `labs/` и `docker compose config`).

Код лабораторных попадает в уроки только импортом региона: в Go-файле регион размечен `// #region имя` … `// #endregion имя`, в уроке — `<<< @/../labs/0b/cmd/tcpecho/main.go#server{go}`. Так урок всегда показывает тот код, который проверен тестами. Вывод команд в уроках — реальный, с пометкой «у вас числа будут другими, порядок — такой».

## Как добавить урок

1. Файл `course/modules/NN-slug/NN-lesson.md` (из пути вычисляется строка «Модуль N · Урок M» над заголовком) с frontmatter `title:` — он сам появится в меню.
   Папки модулей начинаются с двух цифр (`00-foundation`, `00b-network`, `01-go-service`, `01-5-concurrency`), остальные папки в меню не попадают. Порядок модулей задаёт числовое поле `order:` в `index.md` модуля (`0`, `0.5`, `1`, `1.5`…): имена вроде `02.5-` и `02a-` по алфавиту сортируются неверно. Без `order:` берётся число из первых двух цифр имени.
2. Термины: `<T id="...">слово</T>`, определение — в `course/glossary/terms.yaml`. `id` пишется только литералом в двойных кавычках; поле `long` в глоссарии не должно содержать `<T>`.
3. Визуализация: сценарий в `course/viz/simulations/` (чистая функция + тест), рендерер в `course/viz/renderers/`, сцена в `course/viz/scenes/`; цвета — из `course/viz/palette.css` (см. `course/viz/README.md`). Сцена подключается в самом уроке через `<script setup>` с `import`.
4. После добавления термина или урока перезапустите `npm run dev`: список терминов и меню считаются при загрузке конфига.
5. Перед коммитом — чек-листы подробности и визуализации из спецификации (разделы 2.2 и 2.3).
6. Разметка урока:
   - у каждого `##` стоит метка `<Part kind="..." />`; kind: `problem`, `theory`, `viz`, `mistakes`, `tradeoff`, `code`, `selfcheck`, `sources`, `deep`, `practice` (список — в `course/components/parts.ts`). Метка не меняет якорь заголовка и подпись ссылки-якоря; у заголовков `##` ведущее слово («Проблема:», «Компромисс:») окрашивается в цвет метки, остальной текст — зелёный акцент, а подзаголовки `###` — в цвет метки своего `##` (плагин `.vitepress/part-subheadings.ts`); метка `practice` — для мест, где ученик должен что-то сделать сам;
   - `<N>` — для 3–8 ключевых чисел урока; оборачивает результат формулы, а не её операнды;
   - `<Deep title="...">` — подробности «под капотом», 150–500 слов;
   - `simplified` в `Callout` — для упрощённой модели или кода, с перечнем опущенного.
7. Go-фрагменты помечаются `<!-- go-check -->` или `<!-- go-check: harness=имя -->` перед блоком. Обвязки лежат в `course/scripts/go-harness/`, проверка — `npm run check:go`.
8. Визуализации и схемы оборачиваются в `VizFrame` (полноэкранный режим); блоки mermaid оборачиваются автоматически.

Спецификация: `docs/superpowers/specs/2026-10-09-course-platform-design.md`.
