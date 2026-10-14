# System Design на практике

Личный курс по проектированию высоконагруженных систем на Go: сайт курса с пошаговыми визуализациями (`course/`) и сквозной проект-мессенджер (`messenger/`, с модуля 1).

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
npm run check:messenger # gofmt, go vet, golangci-lint, go test -race мессенджера и ссылки <<< в уроках
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

## Мессенджер

Сквозной проект курса лежит в `messenger/` — отдельный Go-модуль (`messenger/go.mod`, Go 1.26). Код раскладывается по контекстам (`internal/user`, `internal/chat`, `internal/auth`) и слоям `domain` ← `usecase` ← `adapters` ← `cmd`; правило зависимостей проверяет линтер `depguard` (настройка — `messenger/.golangci.yml`, решения — `messenger/docs/adr/`).

```bash
cd messenger
go run ./cmd/keygen -out jwt.pem          # один раз: закрытый ключ подписи JWT (Ed25519, права 0600)
MESSENGER_JWT_KEY_FILE=jwt.pem go run ./cmd/messenger                        # HTTP-сервер на 127.0.0.1:8080
MESSENGER_JWT_KEY_FILE=jwt.pem MESSENGER_ADDR=:8080 go run ./cmd/messenger   # слушать все интерфейсы — явное решение
```

Без `MESSENGER_JWT_KEY_FILE` сервер не стартует. `keygen` не перезаписывает существующий файл: новый ключ сделал бы недействительными все выданные токены. Файлы `*.pem` в git не попадают (`messenger/.gitignore`). Клиент регистрируется (`POST /v1/users` с паролем), получает токен (`POST /v1/tokens`) и передаёт его в `Authorization: Bearer …`. Данные живут в памяти и пропадают при остановке; по умолчанию сервер слушает только loopback. Сквозной сценарий с `curl` — в уроке 1.14.

API описан контрактом `messenger/api/openapi.yaml` (contract-first, ADR-0006): типы запросов и ответов и интерфейсы обработчиков сгенерированы из него в пакеты `internal/*/adapters/httpapi/gen`. После правки контракта — `go generate ./...` из `messenger/`; сгенерированные файлы коммитятся, а `npm run check:messenger` падает, если они отстали от контракта.

Проверка — из `course/`: `npm run check:labs` (gofmt, `go vet`, `go test -race` по `labs/` и `docker compose config`).

Код лабораторных попадает в уроки только импортом региона: в Go-файле регион размечен `// #region имя` … `// #endregion имя`, в уроке — `<<< @/../labs/0b/cmd/tcpecho/main.go#server{go}`. Так урок всегда показывает тот код, который проверен тестами. Вывод команд в уроках — реальный, с пометкой «у вас числа будут другими, порядок — такой».

## Мессенджер

Сквозной проект курса лежит в `messenger/` — отдельный Go-модуль (`messenger/go.mod`, Go 1.26). Код раскладывается по контекстам (`internal/user`, `internal/chat`) и слоям `domain` ← `usecase` ← `adapters` ← `cmd`; правило зависимостей проверяет линтер `depguard` (настройка — `messenger/.golangci.yml`, решения — `messenger/docs/adr/`).

```bash
cd messenger
go run ./cmd/messenger                    # HTTP-сервер на 127.0.0.1:8080
MESSENGER_ADDR=:8080 go run ./cmd/messenger  # слушать все интерфейсы — явное решение
```

По умолчанию сервер слушает только loopback: пока вместо аутентификации заглушка `X-User-ID`, сборку не должно быть видно из сети (ADR-0002). Данные живут в памяти и пропадают при остановке. Сценарий с `curl` — в уроке 1.7.

Проверка — из `course/`: `npm run check:messenger` — `gofmt`, свежесть `go generate`, `go vet`, `golangci-lint run`, `go test -race` по `messenger/` и ссылки `<<< @/../messenger/…#регион` во всех уроках (нет файла, нет региона или строка с `<<<`, которую не разобрать, — ошибка). Нужен **golangci-lint v2** (`version: "2"` в конфиге; v1 его не прочитает), например `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`; скрипт ищет его сначала в `~/go/bin`, потом в `PATH`.

Каждый урок модуля 1 заканчивается тегом `m01-lNN` (сейчас — от `m01-l01` до `m01-l14`) — состоянием кода после урока: `git checkout m01-l03` показывает код после урока 1.3, а `git -c core.quotePath=false diff --stat m01-l02 m01-l03 -- messenger` — что в нём изменилось. На каждом теге `go build ./...` и `go test -race ./...` проходят.

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
