# System Design на практике

[![ci](https://github.com/fazletdinov/system-desine/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/fazletdinov/system-desine/actions/workflows/ci.yml?query=branch%3Amain)

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
npm run check:messenger # gofmt, свежесть go generate, go vet, golangci-lint, go test -race мессенджера и ссылки <<< в уроках
npm run check:refs      # только ссылки <<< @/../…#регион во всех уроках (файл и регион существуют)
npm run check:images    # образ мессенджера: размер, пользователь, trivy, smoke-тест; варианты naive/alpine/scratch урока 1.16 (Docker, минуты и сеть)
npm run check:workflows # actionlint по .github/workflows
npm run check:base      # сборка курса под подпутём /check-base/, как на GitHub Pages: нет ссылок мимо базового пути
```

Опубликованный курс: https://fazletdinov.github.io/system-desine/ (workflow `pages`, обновляется после каждого зелёного `ci` на `main`). Образ мессенджера: `ghcr.io/fazletdinov/messenger` (теги `main`, `sha-<коммит>`, версии).

## CI

Workflow лежат в `.github/workflows/` (урок 1.18 — `ci.yml`, урок 1.19 — `release.yml` и `pages.yml`; обновления зависимостей — `.github/dependabot.yml`). Все действия закреплены полным SHA.

- `ci` — на каждый pull request и push в `main`. Фильтр путей (`changes`) запускает только затронутые ворота: мессенджер — `golangci-lint`, `go test -race` с покрытием, свежесть `go generate`, `govulncheck`; курс — `npm test`, `typecheck`, `check:go`, `check:refs`, сборка; лабы — `check-labs`; образ — сборка, `trivy`, smoke-тест; workflow — `actionlint`. На push в `main` проверяется всё. Итоговая джоба `ci-ok` — единственная обязательная проверка в защите ветки: она красная, если любые ворота упали или отменены, и зелёная, если они прошли или пропущены фильтром.
- `release` — после зелёного `ci` на `main` и на тег `v*`: мультиархитектурный образ в GHCR с SBOM и подписанной аттестацией происхождения (provenance).
- `pages` — сборка курса под адресом GitHub Pages и публикация.

## Лабораторные

Практика модулей лежит в `labs/` — отдельный Go-модуль (`labs/go.mod`, Go 1.27, без внешних зависимостей). Каталог модуля 0b:

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

Лабораторная модуля 1c — `labs/1c/`: `minicontainer`, учебный контейнер на Go (namespaces UTS, PID, mount, net, IPC, `pivot_root` в rootfs busybox, лимиты cgroup v2), и `hog`, который занимает память для опыта с `memory.max`. Окружение — привилегированный контейнер с `cgroup: private`; `labs/1c/scenario.sh` проводит пять экспериментов урока 1.15 с проверками, команды — в `labs/1c/README.md`.

```bash
cd labs/1c
docker compose build
docker compose run --rm lab        # оболочка внутри окружения лабы; дальше — minicontainer run …
docker compose run --rm -T lab /lab/scenario.sh
```

Проверка — из `course/`: `npm run check:labs` (gofmt, `go vet`, `go test -race` по `labs/`, `docker compose config` и сценарий 1c).

Код лабораторных попадает в уроки только импортом региона: в Go-файле регион размечен `// #region имя` … `// #endregion имя`, в уроке — `<<< @/../labs/0b/cmd/tcpecho/main.go#server{go}`. Так урок всегда показывает тот код, который проверен тестами. Вывод команд в уроках — реальный, с пометкой «у вас числа будут другими, порядок — такой».

## Мессенджер

Сквозной проект курса лежит в `messenger/` — отдельный Go-модуль (`messenger/go.mod`, Go 1.27). Код раскладывается по контекстам (`internal/user`, `internal/chat`, `internal/auth`) и слоям `domain` ← `usecase` ← `adapters` ← `cmd`; правило зависимостей проверяет линтер `depguard` (настройка — `messenger/.golangci.yml`, решения — `messenger/docs/adr/`).

```bash
cd messenger
go run ./cmd/keygen -out jwt.pem          # один раз: закрытый ключ подписи JWT (Ed25519, права 0600)
MESSENGER_JWT_KEY_FILE=jwt.pem go run ./cmd/messenger                        # HTTP-сервер на 127.0.0.1:8080
MESSENGER_JWT_KEY_FILE=jwt.pem MESSENGER_ADDR=:8080 go run ./cmd/messenger   # слушать все интерфейсы — явное решение
```

Без `MESSENGER_JWT_KEY_FILE` сервер не стартует. Сроки задаются окружением: `MESSENGER_REQUEST_TIMEOUT` (по умолчанию 10 с), `MESSENGER_SHUTDOWN_TIMEOUT` (10 с) и `MESSENGER_DRAIN_DELAY` (5 с) — при старте проверяется, что дренаж и остановка вместе укладываются в 30 с до SIGKILL. `keygen` не перезаписывает существующий файл: новый ключ сделал бы недействительными все выданные токены. Файлы `*.pem` в git не попадают (`messenger/.gitignore`). Клиент регистрируется (`POST /v1/users` с паролем), получает токен (`POST /v1/tokens`) и передаёт его в `Authorization: Bearer …`. Данные живут в памяти и пропадают при остановке; по умолчанию сервер слушает только loopback. Сквозной сценарий с `curl` — в уроке 1.14.

API описан контрактом `messenger/api/openapi.yaml` (contract-first, ADR-0006): типы запросов и ответов и интерфейсы обработчиков сгенерированы из него в пакеты `internal/*/adapters/httpapi/gen`. После правки контракта — `go generate ./...` из `messenger/`; сгенерированные файлы коммитятся, а `npm run check:messenger` падает, если они отстали от контракта.

### В Docker

```bash
cd messenger
go run ./cmd/keygen -out jwt.pem && docker compose up --build   # http://127.0.0.1:8080
```

Образ — multi-stage сборка на distroless (`messenger/Dockerfile`, урок 1.16), процесс работает от UID 65532 с файловой системой только для чтения, без capabilities, с лимитами 256 МБ памяти и 1 CPU (`messenger/compose.yaml`, урок 1.17). Ключ подписи приходит секретом-файлом в `/run/secrets/jwt_key`, а не переменной окружения. На Docker Desktop файл `0600` читается процессом контейнера; на «голом» Docker Engine (Linux без Desktop) — нет, и сервер падает с `permission denied`: для учебного ключа `chmod 0444 jwt.pem`, почему — в уроке 1.17. Пробы: `/livez` (процесс жив) и `/readyz` (готов принимать трафик; по SIGTERM сразу 503, затем дренаж и остановка). `HEALTHCHECK` образа вызывает `messenger healthcheck`. Порт опубликован только на loopback хоста.

Проверка — из `course/`: `npm run check:messenger` — `gofmt`, свежесть `go generate`, `go vet`, `golangci-lint run`, `go test -race` по `messenger/` и ссылки `<<< @/../messenger/…#регион` во всех уроках (нет файла, нет региона или строка с `<<<`, которую не разобрать, — ошибка). Нужен **golangci-lint v2** (`version: "2"` в конфиге; v1 его не прочитает), например `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0`; скрипт ищет его сначала в `~/go/bin`, потом в `PATH`.

Каждый урок модуля 1 заканчивается тегом `m01-lNN` (сейчас — от `m01-l01` до `m01-l20`) — состоянием кода после урока: `git checkout m01-l03` показывает код после урока 1.3, а `git -c core.quotePath=false diff --stat m01-l02 m01-l03 -- messenger` — что в нём изменилось. На каждом теге `go build ./...` и `go test -race ./...` проходят в `messenger/` и `labs/`.

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
