# System Design на практике

[![ci](https://github.com/fazletdinov/system-desine/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/fazletdinov/system-desine/actions/workflows/ci.yml?query=branch%3Amain)

Личный курс по проектированию высоконагруженных систем на Go: сайт курса с пошаговыми визуализациями (`course/`) и сквозной проект-мессенджер (`messenger/`, с модуля 1).

## Что в курсе

- Модуль 0 и 0b — основы и сети (`course/modules/00-foundation`, `00b-network`).
- Модуль 1 — Go-сервис: мессенджер от домена до образа, CI и публикации (уроки 1.1–1.20).
- Модуль 1.5 — конкурентность и рантайм Go (`course/modules/01-5-go-concurrency`, одиннадцать уроков). Часть 1.5a (уроки 1.5.1–1.5.6): горутины и планировщик, каналы, модель памяти и гонки, `sync` и атомики, паттерны конкурентности, жизненный цикл и фоновая работа. Часть 1.5b (уроки 1.5.7–1.5.10): netpoller и цена соединения, стек, куча и аллокатор, сборщик мусора (`GOGC`, `GOMEMLIMIT`, ограничитель CPU), профилирование мессенджера (`pprof`, трасса, flight recorder, PGO). Урок 1.5.11 — мини-интервью модуля с симулятором вопросов. Лаба — `labs/1-5`; в мессенджере появляются жизненный цикл компонентов, фоновая очистка идемпотентности, ограничение одновременных хешей и служебный порт с профилями и снимками flight recorder.
- Модуль 2 — базы данных (`course/modules/02-database`). Часть 2a (уроки 2.1–2.9) — проектирование данных: модель от требований и ER-диаграмма, нормализация, ключи и типы (UUIDv7), ограничения и перевод SQLSTATE в доменные ошибки, проектирование от паттернов доступа, JSONB, мягкое удаление и история, мультиарендность и антипаттерны, миграции как код. В уроке 2.9 мессенджер переезжает на PostgreSQL 18.6: адаптеры на `pgx`/`pgxpool` за прежними портами, транзакции на уровне use case (`platform/pgtx`), миграции goose, батареи `storetest` против настоящей базы (testcontainers). Лаба — `labs/2a`. Часть 2b — PostgreSQL изнутри — следующая.

## Версия Go

Весь репозиторий — на Go 1.27.2: в `messenger/go.mod` и `labs/go.mod` стоят `go 1.27.0` и `toolchain go1.27.2` (1.27.0 уязвим: CVE-2026-78667, CVE-2026-78669, CVE-2026-97031 исправлены в 1.27.2). Локальный Go по умолчанию тоже должен быть 1.27.2, например `mise use -g go@1.27.2`: иначе более старый Go (скажем, 1.26) по строке `toolchain` сам скачает 1.27.2 при каждом запуске команды, а редактор с `gopls` на старом Go будет пытаться скачать неподходящий инструментарий.

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
npm run check:messenger # gofmt, свежесть go generate, go vet (и с тегом migratedemo), golangci-lint, go test -race мессенджера и ссылки <<< в уроках
npm run check:refs      # только ссылки <<< @/../…#регион во всех уроках (файл и регион существуют)
npm run check:images    # образ мессенджера: размер, пользователь, trivy, smoke-тест; варианты naive/alpine/scratch урока 1.16 (Docker, минуты и сеть)
npm run check:workflows # actionlint по .github/workflows
npm run check:base      # сборка курса под подпутём /check-base/, как на GitHub Pages: нет ссылок мимо базового пути
```

Опубликованный курс: https://fazletdinov.github.io/system-desine/ (workflow `pages`, обновляется после каждого зелёного `ci` на `main`). Образ мессенджера: `ghcr.io/fazletdinov/messenger` (теги `main`, `sha-<коммит>`, версии).

## CI

Workflow лежат в `.github/workflows/` (урок 1.18 — `ci.yml`, урок 1.19 — `release.yml` и `pages.yml`; обновления зависимостей — `.github/dependabot.yml`). Все действия закреплены полным SHA.

- `ci` — на каждый pull request и push в `main`. Фильтр путей (`changes`) запускает только затронутые ворота: мессенджер — `golangci-lint` и `go vet -tags migratedemo`, `go test -race -short` без Docker (`messenger-test`), полный `go test -race` с PostgreSQL в testcontainers и покрытием (`messenger-integration`), свежесть `go generate`, `govulncheck`; курс — `npm test`, `typecheck`, `check:go`, `check:refs`, сборка; лабы — `check-labs`; образ — сборка, `trivy`, smoke-тест; workflow — `actionlint`. На push в `main` проверяется всё. Итоговая джоба `ci-ok` — единственная обязательная проверка в защите ветки: она красная, если любые ворота упали или отменены, и зелёная, если они прошли или пропущены фильтром.
- `release` — после зелёного `ci` на `main` и на тег `v*`: мультиархитектурный образ в GHCR с SBOM и подписанной аттестацией происхождения (provenance).
- `pages` — сборка курса под адресом GitHub Pages и публикация.

## Лабораторные

Практика модулей лежит в `labs/` — отдельный Go-модуль (`labs/go.mod`, Go 1.27.2; единственная внешняя зависимость — `golang.org/x/sync` для `errgroup` и `semaphore` в уроке 1.5.5). Каталог модуля 0b:

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

Лабораторная модуля 1.5 — `labs/1-5/`: маленькие программы без Docker. Часть 1.5a — `sched`, `chan`, `race`, `sync`, `patterns`, по одной группе на урок 1.5.1–1.5.5; часть 1.5b — `netpoll` (цена простаивающего соединения, урок 1.5.7), `alloc` (рост стека, escape-анализ, цена аллокации, 1.5.8), `gc` (`gctrace`, ограничитель CPU, Green Tea, 1.5.9), `pgo` и `fold` (PGO и свёртка профилей для флейм-графа, 1.5.10). `alloc/escape/dump` печатает данные песочницы escape-lab (`course/viz/data/escape-lab.json`), сценарий проверяет, что файл с ними совпадает. Каждая печатает версию Go, `GOMAXPROCS` и `NumCPU`; намеренные гонки и утечки собираются только с тегом `labdemo`, чтобы не попадать в `go vet` и `-race`. `labs/1-5/scenario.sh` запускает всё и проверяет ключевые строки, команды — в `labs/1-5/README.md`.

```bash
cd labs
go run ./1-5/sched/cost
bash 1-5/scenario.sh
```

Лабораторная модуля 2a — `labs/2a/`: одноразовый PostgreSQL 18.6 в Docker (`compose.yaml`, порт только на 127.0.0.1, данные на tmpfs), SQL-скрипты с намеренными ошибками — аномалии и нормальные формы (2.2), типы и коллации (2.3), ограничения и их SQLSTATE (2.4), планы запросов мессенджера на миллионе чатов (2.5), JSONB и GIN, мягкое удаление, аудит триггером (2.6), антипаттерны Карвина и RLS (2.7) — и Go-программа `keys` на `pgx` (цена `bigint`, UUIDv4 и UUIDv7 как первичного ключа). `labs/2a/scenario.sh` поднимает свою копию базы, прогоняет всё с проверками и убирает контейнер; полные выводы, на которые ссылаются уроки, — в `labs/2a/results/`, команды — в `labs/2a/README.md`.

```bash
cd labs
bash 2a/scenario.sh                                   # около 15 с с прогретым кешем; нужен Docker
docker compose -f 2a/compose.yaml up -d --wait        # база для своих опытов на 127.0.0.1:55432
docker compose -f 2a/compose.yaml exec db psql
docker compose -f 2a/compose.yaml down -v
```

Проверка — из `course/`: `npm run check:labs` (gofmt, `go vet`, `go test -race` по `labs/`, `docker compose config`, сценарии 1c, 1.5 и 2a).

Код лабораторных попадает в уроки только импортом региона: в Go-файле регион размечен `// #region имя` … `// #endregion имя`, в уроке — `<<< @/../labs/0b/cmd/tcpecho/main.go#server{go}`. Так урок всегда показывает тот код, который проверен тестами. Вывод команд в уроках — реальный, с пометкой «у вас числа будут другими, порядок — такой».

## Мессенджер

Сквозной проект курса лежит в `messenger/` — отдельный Go-модуль (`messenger/go.mod`, Go 1.27.2). Код раскладывается по контекстам (`internal/user`, `internal/chat`, `internal/auth`) и слоям `domain` ← `usecase` ← `adapters` ← `cmd`; правило зависимостей проверяет линтер `depguard` (настройка — `messenger/.golangci.yml`, решения — `messenger/docs/adr/`).

```bash
cd messenger
go run ./cmd/keygen -out jwt.pem          # один раз: закрытый ключ подписи JWT (Ed25519, права 0600)
# нужен PostgreSQL с применёнными миграциями (проще всего — compose, см. «В Docker»);
# secrets/local_url — файл со строкой postgres://… (каталог secrets/ в git не попадает):
# MESSENGER_DATABASE_URL_FILE=secrets/local_url go run ./cmd/messenger migrate up
export MESSENGER_JWT_KEY_FILE=jwt.pem MESSENGER_DATABASE_URL_FILE=secrets/local_url
go run ./cmd/messenger                                         # HTTP-сервер на 127.0.0.1:8080
MESSENGER_ADDR=:8080 go run ./cmd/messenger                    # слушать все интерфейсы — явное решение
MESSENGER_DEBUG_ADDR=127.0.0.1:6060 go run ./cmd/messenger     # + служебный порт pprof, только loopback
```

Без `MESSENGER_JWT_KEY_FILE` и `MESSENGER_DATABASE_URL_FILE` сервер не стартует; не стартует он и тогда, когда база не ответила за 10 с или схема в ней старше той, на которую рассчитан бинарь (подсказка — `messenger migrate up`). Пул соединений: `MESSENGER_DB_MAX_CONNS` — потолок соединений (целое от 1 до 100; по умолчанию 10, урок 2.9); `/readyz` отвечает 503 с `"checks":{"db":"fail"}`, если база не ответила на `Ping` за 1 с, а `/livez` базу не проверяет. Сроки задаются окружением: `MESSENGER_REQUEST_TIMEOUT` (по умолчанию 10 с), `MESSENGER_SHUTDOWN_TIMEOUT` (10 с) и `MESSENGER_DRAIN_DELAY` (5 с) — при старте проверяется, что дренаж, остановка HTTP и 2 с на фоновые компоненты вместе укладываются в 30 с до SIGKILL. Фоновая работа и нагрузка: `MESSENGER_IDEMPOTENCY_SWEEP_EVERY` — период очистки ключей идемпотентности (Go-длительность, например `30s`; по умолчанию 1 мин, больше нуля), `MESSENGER_HASH_CONCURRENCY` — сколько хешей паролей считать одновременно (целое от 1 до 256; по умолчанию `GOMAXPROCS`, но не больше 256), `MESSENGER_HASH_QUEUE` — сколько вызовов ждут места (целое от 0 до 1024; по умолчанию 4 × `MESSENGER_HASH_CONCURRENCY`; 0 — отказ сразу, если все места заняты; сверх очереди — 503 `overloaded` с `Retry-After`). Служебный порт (урок 1.5.10, ADR-0013): `MESSENGER_DEBUG_ADDR` — адрес слушателя с `net/http/pprof` (`/debug/pprof/…`, `?seconds` не больше 60) и снимком flight recorder (`GET /debug/flight`, файл для `go tool trace`); по умолчанию пусто — порт выключен, потому что профили раскрывают стеки и командную строку; при включённом порте включаются и профили mutex и block. Локально задавайте адрес на loopback — `MESSENGER_DEBUG_ADDR=127.0.0.1:6060`: форма `:6060` слушает все интерфейсы и открывает профили всей сети (в compose она нужна внутри контейнера, а наружу порт опубликован только на loopback хоста). `MESSENGER_FLIGHT_DIR` — существующий каталог, куда пишется снимок трассы последних секунд после запроса дольше `MESSENGER_FLIGHT_THRESHOLD` (по умолчанию 1 с, меньше окна рекордера 10 с), не чаще раза в минуту; пусто — не писать, требует `MESSENGER_DEBUG_ADDR`. Нечисловое или выходящее за пределы значение — ошибка старта с именем переменной. Второй SIGINT/SIGTERM во время остановки завершает процесс сразу с кодом 1. `keygen` не перезаписывает существующий файл: новый ключ сделал бы недействительными все выданные токены. Файлы `*.pem` в git не попадают (`messenger/.gitignore`). Клиент регистрируется (`POST /v1/users` с паролем), получает токен (`POST /v1/tokens`) и передаёт его в `Authorization: Bearer …`. Данные живут в PostgreSQL (модуль 2a) и переживают перезапуск; по умолчанию сервер слушает только loopback. Сквозной сценарий с `curl` — в уроке 1.14.

API описан контрактом `messenger/api/openapi.yaml` (contract-first, ADR-0006): типы запросов и ответов и интерфейсы обработчиков сгенерированы из него в пакеты `internal/*/adapters/httpapi/gen`. После правки контракта — `go generate ./...` из `messenger/`; сгенерированные файлы коммитятся, а `npm run check:messenger` падает, если они отстали от контракта.

### В Docker

```bash
cd messenger
go run ./cmd/keygen -out jwt.pem           # один раз: ключ подписи JWT
# один раз: пароль PostgreSQL и строка подключения с ним — файлами-секретами в messenger/secrets/
mkdir -p secrets
openssl rand -hex 24 > secrets/db_password
printf 'postgres://messenger:%s@db:5432/messenger?sslmode=disable\n' "$(cat secrets/db_password)" > secrets/db_url
chmod 0444 secrets/db_password secrets/db_url   # читают процессы контейнеров (UID 999 и 65532), см. ниже
docker compose up --build                  # http://127.0.0.1:8080
```

Образ — multi-stage сборка на distroless (`messenger/Dockerfile`, урок 1.16), процесс работает от UID 65532 с файловой системой только для чтения, без capabilities, с лимитами 256 МБ памяти и 1 CPU (`messenger/compose.yaml`, урок 1.17). Ключ подписи приходит секретом-файлом в `/run/secrets/jwt_key`, а не переменной окружения. На Docker Desktop файл `0600` читается процессом контейнера; на «голом» Docker Engine (Linux без Desktop) — нет, и сервер падает с `permission denied`: для учебного ключа `chmod 0444 jwt.pem`, почему — в уроке 1.17. Пробы: `/livez` (процесс жив) и `/readyz` (готов принимать трафик; по SIGTERM сразу 503, затем дренаж и остановка). `HEALTHCHECK` образа вызывает `messenger healthcheck`. Порты опубликованы только на loopback хоста: API — 8080, служебный (pprof, `/debug/flight`) — 6060; в compose служебный порт включён, а снимки по медленным запросам пишутся в tmpfs `/tmp` контейнера (потолок 32 МиБ: tmpfs — это память контейнера) — `docker compose cp` его не видит, поэтому на стенде снимок берут с `GET /debug/flight`. Нагрузка для опытов — из `messenger/`: `scripts/overload.sh [N]` (N одновременных регистраций, урок 1.5.6) и `scripts/read-load.sh [N] [секунд]` (чтение истории чата, урок 1.5.10); профиль снимают со служебного порта, например `curl -o cpu.pprof '127.0.0.1:6060/debug/pprof/profile?seconds=20'`, и смотрят `go tool pprof` из `messenger/`.

PostgreSQL 18.6 (урок 2.9) — сервис `db` в том же compose: данные — в томе `db-data` (переживают `docker compose down`, стираются `down -v`), порт наружу не публикуется — посмотреть данные можно через `docker compose exec db psql`. Схему создают миграции goose из `messenger/migrations` (встроены в бинарь): одноразовый сервис `migrate` того же образа выполняет `messenger migrate up` после того, как `db` стал healthy, а `messenger` стартует только после его выхода с кодом 0 (урок 2.8). Применить и посмотреть вручную: сначала `docker compose build` (у сервиса `migrate` нет своей сборки — он берёт образ `messenger:local`, и без сборки compose попытается скачать его из реестра), затем `docker compose run --rm migrate migrate up` и `docker compose run --rm migrate migrate status` (код выхода 3, если есть неприменённые миграции; на пустой базе `status` ничего не создаёт); без compose — `MESSENGER_DATABASE_URL_FILE=<файл со строкой postgres://…> go run ./cmd/messenger migrate status`. Пароль базы (`POSTGRES_PASSWORD_FILE`) и строка подключения (`MESSENGER_DATABASE_URL_FILE`) приходят секретами-файлами `secrets/db_password` и `secrets/db_url`, а не переменными окружения; каталог `messenger/secrets/` в git не попадает, а в образ — тем более (`.dockerignore` пропускает только код). Пароль — hex, чтобы его не нужно было экранировать в строке подключения; `sslmode=disable` — только потому, что база видна лишь во внутренней сети compose. Права `0444` — по той же причине, что у `jwt.pem` на Docker Engine: файл монтируется с владельцем хоста, а читают его пользователь `postgres` (UID 999) и мессенджер (UID 65532). Подкоманда `migrate` не печатает строку подключения и её пароль ни в каких ошибках.

Проверка — из `course/`: `npm run check:messenger` — `gofmt`, свежесть `go generate`, `go vet`, `golangci-lint run`, `go test -race` по `messenger/` и ссылки `<<< @/../messenger/…#регион` во всех уроках (нет файла, нет региона или строка с `<<<`, которую не разобрать, — ошибка). Нужен **golangci-lint v2** (`version: "2"` в конфиге; v1 его не прочитает), например `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0`; скрипт ищет его сначала в `~/go/bin`, потом в `PATH`. Интеграционные тесты (адаптеры и миграции на настоящем PostgreSQL) поднимают контейнер через testcontainers-go (`internal/platform/pgtest`): один контейнер на пакет, перед каждым тестом база восстанавливается из снимка. Без Docker они пропускаются с сообщением; `MESSENGER_REQUIRE_DOCKER=1` превращает пропуск в провал. `go test -short ./...` пропускает их, не трогая Docker, — так идёт CI-джоба `messenger-test`, а полный прогон с `MESSENGER_REQUIRE_DOCKER=1` — джоба `messenger-integration`. Сквозная проверка стека compose — `scripts/smoke.sh --compose` из `messenger/` (сценарий API, перезапуск с данными, порядок остановки HTTP → фон → пул); `scripts/smoke.sh <образ>` — то же для одного образа с одноразовым PostgreSQL рядом (`npm run check:images`, CI-джоба `image`).

Каждый урок модуля 1 заканчивается тегом `m01-lNN` (сейчас — от `m01-l01` до `m01-l20`), каждый урок модуля 1.5 — тегом `m01-5-lNN` (от `m01-5-l01` до `m01-5-l11`), каждый урок части 2a — тегом `m02-lNN` (от `m02-l01` до `m02-l09`) — состоянием кода после урока: `git checkout m01-l03` показывает код после урока 1.3, а `git -c core.quotePath=false diff --stat m01-l02 m01-l03 -- messenger` — что в нём изменилось. На каждом теге `go build ./...` и `go test -race ./...` проходят в `messenger/` и `labs/`; с `m02-l04` в `messenger/` есть интеграционные тесты, и для полного прогона на теге нужен Docker.

## Как добавить урок

1. Файл `course/modules/NN-slug/NN-lesson.md` (из пути вычисляется строка «Модуль N · Урок M» над заголовком) с frontmatter `title:` — он сам появится в меню.
   Папки модулей начинаются с двух цифр (`00-foundation`, `00b-network`, `01-go-service`, `01-5-go-concurrency`), остальные папки в меню не попадают. Порядок модулей задаёт числовое поле `order:` в `index.md` модуля (`0`, `0.5`, `1`, `1.5`…): имена вроде `02.5-` и `02a-` по алфавиту сортируются неверно. Без `order:` берётся число из первых двух цифр имени.
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
