# Модуль 1, часть 1c «Docker изнутри, CI и доставка» — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Уроки 1.15–1.20 и всё, что делает мессенджер продакшен-сервисом в контейнере:
- лаба «контейнер с нуля на Go» (namespaces, `pivot_root`, cgroup v2);
- образ мессенджера: multi-stage, distroless `static:nonroot`, кэш BuildKit;
- сервис в контейнере: `/livez` и `/readyz`, дренаж перед остановкой, таймаут запроса, healthcheck без shell, конфиг таймаутов, `compose.yaml` с жёсткими настройками безопасности;
- CI на GitHub Actions с агрегирующей обязательной проверкой, доставка в GHCR с provenance и SBOM, курс на GitHub Pages, Dependabot;
- симулятор собеседования по модулю 1;
- две песочницы и пять плееров.

**Architecture:**
- Раскладка и правила слоёв 1a/1b не меняются. Новое в мессенджере — технические пакеты `platform/health` и `middleware.Timeout`, служебные подкоманды бинаря (`version`, `healthcheck`) в `cmd/messenger`, параметры остановки в `httpserver.Run`.
- Пробы живут **вне** цепочки middleware и вне контракта: корневой `ServeMux` в composition root отдаёт `/livez` и `/readyz` сам, всё остальное — в прежнюю цепочку (ADR-0008).
- Лаба 1c — отдельная программа в общем модуле `labs`, запускается в privileged-контейнере с собственным cgroup namespace.
- CI — три workflow (`ci`, `release`, `pages`); публикации запускаются только после зелёного `ci` на `main` (`workflow_run`).
- Код растёт урок за уроком: задачи 4–11, тег `m01-lNN` на завершающем коммите урока. Визуализации — задачи 2–3, до уроков.

**Tech Stack:**
- курс: VitePress 1.6.4, Vue 3.5.43, TypeScript 5.9.3, Vitest 3.2.7, Node 26 (у автора 26.4.0), `yaml` 2.9.1;
- мессенджер и лабы: Go 1.26 (у автора 1.26.5; в образах и CI — 1.26.9), `golangci-lint` 2.12.2;
- Docker Engine 29.9, Buildx 0.36, Compose v5.5.1 (у автора — Docker Desktop для Linux: контейнеры работают в его VM);
- инструменты, закреплённые версии (не моложе двух недель на 2026-10-15, проверены спайком 2026-10-15):

| Что | Версия | Закрепление |
|---|---|---|
| `actions/checkout` | v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` |
| `actions/setup-go` | v7.0.0 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` |
| `actions/setup-node` | v7.0.0 | `820762786026740c76f36085b0efc47a31fe5020` |
| `actions/upload-artifact` | v7.0.1 | `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a` |
| `dorny/paths-filter` | v4.0.3 | `ceb8a2b8f2d89434be7ff52d3de7ec3738c5cc9d` |
| `golangci/golangci-lint-action` | v9.3.0 | `ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a` |
| `docker/setup-buildx-action` | v4.4.1 | `f87e5991a6d7451dcb8d9637bfbc97413f497069` |
| `docker/build-push-action` | v7.4.0 | `c3c9e263c25d99ce0380d002d59b67737d91b0dc` |
| `docker/login-action` | v4.6.0 | `dbcb813823bdd20940b903addbd779551569679f` |
| `docker/metadata-action` | v6.2.0 | `dc802804100637a589fabce1cb79ff13a1411302` |
| `aquasecurity/trivy-action` | v0.36.0 | `ed142fd0673e97e23eac54620cfb913e5ce36c25` |
| `actions/attest-build-provenance` | v4.2.2 | `4d101475d8b20a2381f78447822ac1eab6504dd8` |
| `actions/configure-pages` | v6.0.0 | `45bfe0192ca1faeb007ade9deae92b16b8254a0d` |
| `actions/upload-pages-artifact` | v5.0.0 | `fc324d3547104276b827a68afc52ff2a11cc49c9` |
| `actions/deploy-pages` | v5.0.1 | `368f82528645a54fb793d4d04e342629a3f51346` |
| trivy (CLI и образ `aquasec/trivy`) | 0.74.0 | тег |
| actionlint | v1.7.12 | `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` |
| govulncheck | v1.8.0 | `go run golang.org/x/vuln/cmd/govulncheck@v1.8.0` (v1.9.0 вышла 2026-10-11 — моложе двух недель) |
| `docker/dockerfile` (синтаксис) | 1.19 | `sha256:b6afd42430b15f2d2a4c5a02b919e98a525b785b1aaff16747d2f623364e39b6` |
| `golang:1.26.9-trixie` | — | `sha256:f06b7122f888680048afb247c4e5f49c0069eb6c4c1ccae0f3f4834527a8d22c` |
| `gcr.io/distroless/static-debian13:nonroot` | — | `sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3` |
| `busybox:1.37` | — | `sha256:bdf57e528e45e4433820e045b29b4597825a1c9e38353532d90a01445013f82e` |
| `alpine:3.23` | — | `sha256:85fe1e81d6758c208f3e1eed4338a1997e19d4be002d4dd32d3100c9a8c010a0` |

**Spec:**
- `docs/superpowers/specs/2026-10-15-module-1c-docker-ci-design.md` — главный.
- `docs/superpowers/specs/2026-10-14-module-1b-rest-api-design.md` и `docs/superpowers/specs/2026-10-13-module-1a-service-architecture-design.md` — правила, которые продолжают действовать.
- `docs/superpowers/specs/2026-10-09-course-platform-design.md` — §1, §2.2–2.5.

**Отступления от стандарта плана (как в 1a и 1b):**
- **Пакеты Go (`platform/health`, `middleware.Timeout`, `httpserver`, конфиг, healthcheck, лаба `minicontainer`), shell-скрипты, Dockerfile, compose-файлы, workflow, симуляции песочниц, схема и логика симулятора интервью** даны полным кодом.
- **Тексты уроков, рендереры, сцены, плееры, компонент `InterviewSim`, банк вопросов, ADR** даны **контрактом**: поведение, обязательные свойства, имена. Ревьюер проверяет контракт.

**Проверено спайком 2026-10-15 (не перепроверять, но не противоречить):**
- `docker compose` (v5.5.1) **не поддерживает** `uid`, `gid`, `mode` у секретов из файла: предупреждает «secrets `uid`, `gid` and `mode` are not supported, they will be ignored». На Docker Desktop файл 0600 владельца хоста внутри контейнера виден как `-rw------- 65532 65532` и читается процессом с UID 65532 (общий доступ к файлам VM подменяет владельца). На «голом» Docker Engine (раннеры GitHub, Linux без Desktop) bind mount сохраняет владельца хоста — UID 65532 файл 0600 **не прочитает**. Отсюда: в CI одноразовый ключ получает права 0444; урок 1.17 объясняет разницу и как это делают в Kubernetes (`Secret` с `defaultMode` и `fsGroup`).
- Docker Desktop не видит `/tmp` хоста (bind mount падает «bind source path does not exist»): всё, что монтируется в контейнер, лежит в каталоге проекта.
- Docker Hub у автора периодически отвечает таймаутом (`context deadline exceeded`, `TLS handshake timeout`): при сбое загрузки образа — повторить; это не ошибка плана.
- Корень `/sys/fs/cgroup` у автора — `cgroup2fs`; Docker: cgroup v2, драйвер `cgroupfs`.
- `golangci-lint` 2.12.2 лежит в `~/go/bin`; `actionlint`, `trivy`, `govulncheck`, `gh` локально не установлены — запускаются через `go run …@версия` и образ `aquasec/trivy:0.74.0`.
- `problem.Respond` уже переводит `context.DeadlineExceeded` в 503 `timeout` (код и строка реестра есть с 1b); `httpserver.New` уже выставляет `ReadHeaderTimeout` 5 с, `ReadTimeout` 10 с, `WriteTimeout` 10 с, `IdleTimeout` 60 с; `main.go` уже ловит SIGTERM через `signal.NotifyContext`.
- `check-labs.ts` сценарии 0b **не** запускает (только `gofmt`, `vet`, `test -race`, `compose config`); сценарий 1c — первый, который он запускает.

**Решения плана, уточняющие спеку (рулинги до исполнения):**
1. **Go в CI — `go-version: "1.26.x"`, а не `go-version-file`.** В `go.mod` написано `go 1.26.0`; `setup-go` по нему поставил бы 1.26.0, и `govulncheck` справедливо нашёл бы уязвимости стандартной библиотеки, закрытые в патчах. Ветка `1.26.x` берёт последний патч. Урок 1.18 объясняет почему. Спека §5.2 говорит «из `go.mod`» — отступление осознанное.
2. **Rootfs лабы — `COPY --from=busybox:1.37@sha256:…`**, а не скрипт `docker export`. Так закрепление по digest сохраняется, лишний скрипт `rootfs.sh` не нужен, а сама лаба становится примером multi-stage. Для эксперимента с памятью в rootfs кладётся статический Go-бинарь `hog` (в busybox нечем занять память).
3. **Сборка образов — отдельный скрипт `npm run check:images`**, а не часть `check:messenger`. Сборка четырёх образов и `trivy` — минуты и сеть; `check:messenger` остаётся быстрым. В CI те же проверки делает джоба `image`.
4. **Ссылки `<<<` проверяет отдельный скрипт `npm run check:refs`** (функция выносится из `check-messenger.ts`, тот её импортирует). Джобе `course` в CI не нужен весь `check:messenger` — линт и тесты мессенджера уже делают его джобы.
5. **`check:workflows`** — новый npm-скрипт: `actionlint` по `.github/workflows`.
6. **Таймауты `http.Server` 1a сохраняются** (`ReadTimeout` 10 с, `IdleTimeout` 60 с — спека разрешила «проверить в плане»); меняется только `WriteTimeout` = таймаут запроса + 5 с.
7. **Плеер `request-pipeline` урока 1.8 не меняется:** он показывает цепочку на момент урока 1.8. Урок 1.8 получает одну строку-ссылку «в уроке 1.17 в цепочку встанет `Timeout`».
8. **Строка в журнале при остановке — существующая «сервер остановлен»** (спека §11 пишет «shutdown complete»; журнал мессенджера — по-русски).
9. **Метки OCI `source`, `revision`, `version`, `created`** ставит `docker/metadata-action` в CI; в Dockerfile — только постоянные `title` и `description`.
10. **Урок 1.19 до первого пуша** показывает вывод `gh attestation verify` и адрес Pages как «пример вывода» с пометкой; задача 13 (после разрешённого пуша) заменяет его реальным выводом. Это единственное исключение из правила «вывод — реальный».

## Global Constraints

- Команды npm — из `/home/idel/DevGo/system-desine/course`; Go-команды мессенджера — из `/home/idel/DevGo/system-desine/messenger`; лаб — из `/home/idel/DevGo/system-desine/labs`. `golangci-lint` — в `~/go/bin` (`PATH=$HOME/go/bin:$PATH`). Версии — точные, из таблицы выше.
- Работа идёт в ветке `feat/module-1c` (уже создана, в ней коммит спеки). Слияние в `main` и любой `git push` — только с явного согласия пользователя. Создание репозитория на GitHub и `gh auth login` выполняет пользователь.
- Язык контента — русский; английский оригинал термина — в скобках при первом упоминании. Каждый термин при первом появлении в уроке — `<T id="…">` (id в двойных кавычках; неизвестный id — ошибка сборки).
- Тексты уроков — по основной спецификации:
  - §1 «Голос автора»: доктор наук, объясняющий просто; упрощение без искажения, упрощения помечены `<Callout type="simplified">`;
  - §2.2 — все 10 правил, включая самодостаточность;
  - §2.3 — у каждой визуализации полноэкранный режим;
  - §2.4 — `<Part>` у каждого `##`; `<N>` для 3–8 ключевых чисел (только результат формулы или измерения); `<Deep>` 150–500 слов (прозой, без кода).
- Структура урока — как у 1.14: «Проблема» → теория (несколько `##`) → «Визуализация» (если есть) → «Код» → «Что изменилось в мессенджере» (у уроков, меняющих код) → «Типичные ошибки» → «Компромисс» → «Самопроверка» (5–8 `<SelfCheck>`) → «Источники».
- Чистые TS-модули не импортируют `vue` и не трогают DOM. В компонентах DOM — только в `onMounted`, обработчиках и `watch` без `immediate`; слушатели снимаются в `onBeforeUnmount`.
- Цвета только через CSS-переменные (`--viz-*`, `--vp-c-*`, `--course-*`, `--part-*`), обе темы, контраст текста ≥ 4,5:1, `prefers-reduced-motion`, 375 px без горизонтальной прокрутки, каждая сцена в `VizFrame`.
- Go-код — стандарт пользователя (`~/.claude/CLAUDE.md`) и §2.5:
  - зависимости только внутрь; порт объявляется у потребителя; узкие интерфейсы;
  - конструкторы `NewX(deps…) (*X, error)` проверяют `nil` и пустые значения;
  - `context.Context` — первый аргумент всего, что ходит вовне;
  - ошибки оборачиваются `fmt.Errorf("…: %w", err)`, сообщения — по-русски, со строчной буквы, без точки;
  - у каждой горутины владелец и способ остановки; таймеры останавливаются;
  - `gofmt`, `go vet`, `golangci-lint run`, `go test -race` — чистые;
  - комментарии объясняют «почему»; все лимиты и сроки — именованные константы.
- Shell-скрипты: `#!/bin/sh` + `set -eu` (POSIX) или `#!/bin/bash` + `set -euo pipefail`, переменные в кавычках; проходят `shellcheck`, если он установлен (`actionlint` проверяет `run:`-блоки им же).
- Workflow: каждое действие закреплено полным SHA с комментарием `# vX.Y.Z`; `permissions` на уровне workflow — минимум, остальное выдаётся джобе; у каждой джобы `timeout-minutes`; `actions/checkout` всегда с `persist-credentials: false`; данные событий (`github.event.*`, `needs.*`) в `run:` — только через `env:`, не подстановкой `${{ }}` в текст скрипта.
- Код и конфиги в уроках — только `<<< @/../<путь>#<регион>{lang}`. Регионы: Go — `// #region имя` … `// #endregion имя`; YAML, Dockerfile, shell — `# region имя` … `# endregion имя`. Иллюстративные Go-фрагменты — с `<!-- go-check -->` и проходят `npm run check:go`.
- Тег `m01-lNN` ставится исполнителем задачи урока на её последний коммит; в раундах исправлений — `git tag -f`. На каждом теге `go build ./...` и `go test -race ./...` в `messenger/` и `labs/` проходят.
- Вывод команд в уроках — реальный, полученный прогоном исполнителя (исключение — рулинг 10).
- Коммиты на русском, заканчиваются строкой `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Факты — сверять с первоисточником (man-страницы `namespaces(7)`, `cgroups(7)`, `pivot_root(2)`, `clone(2)`; `Documentation/admin-guide/cgroup-v2.rst`; OCI image и runtime spec; документация Docker, BuildKit, Compose, GitHub Actions, GHCR, Sigstore, SLSA v1.0; исходники Go 1.26); спорное — помечать в тексте со ссылкой.

## Review Focus

1. **SIGTERM, когда идёт долгий запрос, и новые запросы во время дренажа.** Ожидается: `/readyz` сразу 503; новые запросы в течение `DrainDelay` ещё обслуживаются (балансировщик о неготовности ещё не знает); начатый запрос завершается до `Timeout`; процесс выходит с кодом 0. Покрывают `TestRunDrainsBeforeShutdown` и `TestRunServesDuringDrainDelay` (задача 7).
2. **Обработчик, не уважающий `ctx`, и клиент, ушедший раньше срока.** Ожидается: `Timeout` не пишет второй ответ поверх начатого, не вызывает `onTimeout`, если обработчик ответил сам (пусть и поздно), и молчит, если отменил клиент. Покрывают `TestTimeoutLateWriteKeepsHandlerResponse` и `TestTimeoutClientGoneNoResponse` (задача 7).
3. **Разные формы `MESSENGER_ADDR` для healthcheck:** `:8080`, `0.0.0.0:8080`, `[::]:8080`, `127.0.0.1:9090`, `localhost:8080`, мусор. Ожидается: проба идёт на loopback того же порта; мусор — понятная ошибка и код 1. Покрывает табличный `TestProbeURL` (задача 7).
4. **`minicontainer` не под root, без контроллеров cgroup, ребёнок убит OOM.** Ожидается: понятная ошибка до создания чего-либо; cgroup удаляется и после OOM-kill; в `/sys/fs/cgroup` не остаётся `minicontainer-*`. Покрывают `TestCheckControllers`, `TestCreateCgroupWritesLimits` и последняя проверка `scenario.sh` (задача 4).
5. **Курс под подпутём Pages (`/<repo>/`).** Ожидается: favicon, ассеты, ссылки из Vue-компонентов и глоссария работают; ни одного абсолютного `href="/…"`/`src="/…"` без базового пути. Покрывает `check:base` (задача 10).

---

## Структура файлов

```
.github/
├── workflows/
│   ├── ci.yml                         # задача 9
│   ├── release.yml                    # задача 10
│   └── pages.yml                      # задача 10
└── dependabot.yml                     # задача 10
labs/1c/                               # задача 4
├── README.md
├── Dockerfile
├── compose.yaml
├── entrypoint.sh
├── scenario.sh
└── cmd/
    ├── minicontainer/{main.go,cgroup.go,cgroup_test.go,flags.go,flags_test.go}
    └── hog/main.go
messenger/
├── Dockerfile                         # задача 6 (HEALTHCHECK — задача 7)
├── .dockerignore                      # задача 6
├── compose.yaml                       # задача 8
├── scripts/smoke.sh                   # задача 7
├── cmd/messenger/
│   ├── main.go                        # задачи 6, 7
│   ├── cli.go, cli_test.go            # задача 6 (version), 7 (healthcheck)
│   ├── healthcheck.go, healthcheck_test.go  # задача 7
│   ├── app.go, app_test.go            # задача 7
├── internal/platform/
│   ├── health/{doc.go,health.go,health_test.go}   # задача 7
│   ├── middleware/{timeout.go,timeout_test.go}    # задача 7
│   └── httpserver/{server.go,server_test.go}      # задача 7
├── .golangci.yml                      # задача 7
└── docs/adr/0007…0010                 # задачи 6, 8, 9, 10
course/
├── .nvmrc                             # задача 1
├── package.json                       # задачи 1, 6, 9, 10
├── scripts/{check-refs.ts,check-messenger.ts,check-labs.ts,check-images.ts,check-base.ts}
├── .vitepress/config.ts               # задача 10
├── glossary/terms.yaml                # задача 1
├── interview/{schema.ts,schema.test.ts,session.ts,session.test.ts,bank.test.ts,interview.data.ts,module-01.yaml,InterviewSim.vue}  # задача 11
├── viz/simulations/{layer-cache,supply-chain,container-start,cgroup-limits,graceful-shutdown,gomemlimit,ci-graph}.ts (+ .test.ts)
├── viz/renderers/{LayerCache,SupplyChain,ContainerStart,CgroupLimits,GracefulShutdown,GoMemLimit,CiGraph}View.vue
├── viz/scenes/{LayerCacheSandbox,SupplyChainSandbox,ContainerStart,CgroupLimits,GracefulShutdown,GoMemLimit,CiGraph}.vue
└── modules/01-go-service/
    ├── 15-container-internals.md … 20-interview.md
    ├── examples/dockerfiles/{naive,alpine,scratch}.Dockerfile   # задача 6
    └── 05-context.md, 07-http-adapter.md, 08-resources.md, index.md  # задачи 8, 12
```

---

### Task 1: Глоссарий, разделение проверок курса, версия Node

**Files:**
- Modify: `course/glossary/terms.yaml`
- Create: `course/scripts/check-refs.ts`, `course/.nvmrc`
- Modify: `course/scripts/check-messenger.ts`, `course/package.json`

**Interfaces:**
- Produces: id терминов глоссария (ниже) — их используют подписи визуализаций (задачи 2–3) и уроки (задачи 5–11); `npm run check:refs`; экспорт `checkSnippetRefs(): number` из `scripts/check-refs.ts`.

- [ ] **Step 1: Термины глоссария**

Добавить в `course/glossary/terms.yaml` записи по схеме `glossary/schema.ts` (поля `term`, `en`, `short`, `long`, при необходимости `links`; посмотреть 3–4 существующие записи 1b — `idempotency-key`, `jwt`, `middleware` — и писать так же: `short` — одно предложение, `long` — 2–5 предложений с примером из мессенджера). Ни одного из этих id в файле пока нет (проверено 2026-10-15):

| id | term |
|---|---|
| `container` | Контейнер |
| `namespace` | Пространство имён (namespace) |
| `cgroup` | Контрольная группа (cgroup) |
| `rootfs` | Корневая файловая система контейнера (rootfs) |
| `pivot-root` | pivot_root |
| `overlayfs` | OverlayFS |
| `oci` | OCI (Open Container Initiative) |
| `container-runtime` | Среда выполнения контейнеров (container runtime) |
| `privileged-container` | Привилегированный контейнер |
| `oom-kill` | OOM-kill |
| `cpu-throttling` | Троттлинг CPU |
| `container-image` | Образ контейнера |
| `image-layer` | Слой образа |
| `build-cache` | Кэш сборки |
| `multi-stage-build` | Многоэтапная сборка (multi-stage build) |
| `distroless` | Distroless-образ |
| `static-binary` | Статический бинарь |
| `image-digest` | Digest образа |
| `image-registry` | Реестр образов |
| `pid-1` | PID 1 |
| `graceful-shutdown` | Корректное завершение (graceful shutdown) |
| `liveness-probe` | Проба живости (liveness probe) |
| `readiness-probe` | Проба готовности (readiness probe) |
| `healthcheck` | Healthcheck |
| `twelve-factor` | Двенадцать факторов (The Twelve-Factor App) |
| `request-timeout` | Таймаут запроса |
| `gomemlimit` | GOMEMLIMIT |
| `gomaxprocs` | GOMAXPROCS |
| `ci` | Непрерывная интеграция (CI) |
| `cd` | Непрерывная доставка (CD) |
| `pipeline` | Конвейер CI/CD (pipeline) |
| `ci-job` | Джоба CI (job) |
| `runner` | Раннер (runner) |
| `required-check` | Обязательная проверка (required status check) |
| `monorepo` | Монорепозиторий |
| `least-privilege` | Принцип минимальных привилегий |
| `action-pinning` | Закрепление action по SHA |
| `supply-chain-attack` | Атака на цепочку поставок |
| `sbom` | SBOM (Software Bill of Materials) |
| `provenance` | Происхождение сборки (provenance) |
| `slsa` | SLSA |
| `attestation` | Аттестация (attestation) |
| `dependency-update` | Автообновление зависимостей |
| `technical-interview` | Техническое собеседование |

Факты в `short`/`long` — точные (например, `gomaxprocs`: «с Go 1.25 рантайм по умолчанию учитывает лимит CPU cgroup (`cpu.max`)»; `gomemlimit`: «мягкий лимит памяти для GC, введён в Go 1.19; сам из cgroup не выставляется»; `pivot-root`: «в отличие от `chroot`, меняет корень всего mount namespace, и старый корень можно отмонтировать»).

- [ ] **Step 2: Проверить глоссарий**

Run: `npx vitest run glossary`
Expected: PASS (схема и уникальность id).

- [ ] **Step 3: Вынести проверку ссылок `<<<` в `scripts/check-refs.ts`**

Перенести из `check-messenger.ts` функции `markdownFiles` и `checkSnippetRefs` (без изменений поведения) в новый файл, экспортировать `checkSnippetRefs`, а при запуске файла напрямую — выполнить проверку и напечатать итог:

```ts
import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { hasRegion, scanSnippetRefs } from './snippet-regions.ts'

const COURSE_DIR = fileURLToPath(new URL('..', import.meta.url))
const MODULES_DIR = join(COURSE_DIR, 'modules')

function markdownFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) return markdownFiles(p)
    return e.name.endsWith('.md') ? [p] : []
  })
}

// Ссылки `<<< @/…` в уроках не проверяет сборка VitePress: пропавший регион
// молча даёт пустой блок кода. Поэтому проверяем их сами.
export function checkSnippetRefs(): number {
  const problems: string[] = []
  let total = 0
  for (const file of markdownFiles(MODULES_DIR)) {
    const scan = scanSnippetRefs(readFileSync(file, 'utf8'))
    for (const bad of scan.unparseable) problems.push(`${relative(COURSE_DIR, file)}:${bad.line}: unparseable ref «${bad.text}»`)
    for (const ref of scan.refs) {
      total++
      const target = resolve(COURSE_DIR, ref.path)
      const where = `${relative(COURSE_DIR, file)}:${ref.line} → ${ref.path}${ref.region === null ? '' : `#${ref.region}`}`
      if (!existsSync(target)) problems.push(`${where}: нет файла`)
      else if (ref.region !== null && !hasRegion(readFileSync(target, 'utf8'), ref.region)) problems.push(`${where}: нет региона`)
    }
  }
  if (problems.length > 0) {
    console.error(`Битые ссылки на код:\n${problems.join('\n')}`)
    process.exit(1)
  }
  return total
}

if (import.meta.url === `file://${process.argv[1]}`) {
  console.log(`Ссылки <<< на код — без ошибок (ссылок: ${checkSnippetRefs()}).`)
}
```

В `check-messenger.ts`: удалить перенесённые функции, импортировать `checkSnippetRefs` из `./check-refs.ts`.

- [ ] **Step 4: Исправить `generatedFiles` (бэклог 1b)**

В `check-messenger.ts` путь проверяется на сегмент `gen` **относительно `messenger/`**, а не абсолютный: репозиторий, лежащий в каталоге `gen`, раздул бы счётчик. Заменить условие на
`relative(MESSENGER, p).split(sep).includes('gen')` (импортировать `sep` из `node:path`). Над `checkGenerated` добавить комментарий «почему»: проверка сравнивает рабочее дерево до и после `go generate`, а строгой (по git, на чистом checkout) её делает джоба `messenger-generate` в CI.

- [ ] **Step 5: `.nvmrc` и скрипт**

`course/.nvmrc` — одна строка `26`. В `package.json` → `scripts` добавить `"check:refs": "node scripts/check-refs.ts"`.

- [ ] **Step 6: Прогнать**

Run: `npm run check:refs && npm run check:messenger && npm test`
Expected: оба скрипта печатают итог без ошибок; число ссылок в `check:refs` совпадает с числом в `check:messenger`; тесты зелёные.

- [ ] **Step 7: Commit**

```bash
git add course/glossary/terms.yaml course/scripts/check-refs.ts course/scripts/check-messenger.ts course/package.json course/.nvmrc
git commit -m "chore(course): глоссарий 1c, check:refs, путь gen относительно messenger, .nvmrc

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Песочницы `layer-cache` и `supply-chain`

**Files:**
- Create: `course/viz/simulations/layer-cache.ts`, `course/viz/simulations/layer-cache.test.ts`
- Create: `course/viz/simulations/supply-chain.ts`, `course/viz/simulations/supply-chain.test.ts`
- Create: `course/viz/renderers/LayerCacheView.vue`, `course/viz/renderers/SupplyChainView.vue`
- Create: `course/viz/scenes/LayerCacheSandbox.vue`, `course/viz/scenes/SupplyChainSandbox.vue`

**Interfaces:**
- Consumes: `Simulation`, `Step` (`viz/core/types.ts`), `term()` (`viz/core/caption.ts`), `VizFrame`, `Player`; id глоссария из задачи 1.
- Produces: сцены `LayerCacheSandbox` (урок 1.16) и `SupplyChainSandbox` (урок 1.18).

- [ ] **Step 1: Тесты `layer-cache`**

```ts
import { describe, expect, it } from 'vitest'
import { layerCacheSimulation as sim, type LayerAction } from './layer-cache'

const run = (...actions: LayerAction[]) => actions.reduce(sim.apply, sim.initial())
const statuses = (s: ReturnType<typeof run>) => s.lastBuild.map((l) => l.status)

describe('layer-cache', () => {
  it('первая сборка — всё собирается заново', () => {
    const s = run({ type: 'build' })
    expect(statuses(s).every((x) => x === 'built')).toBe(true)
  })
  it('без изменений повторная сборка берёт все слои из кэша', () => {
    const s = run({ type: 'build' }, { type: 'build' })
    expect(statuses(s).every((x) => x === 'cached')).toBe(true)
  })
  it('правильный порядок: правка исходника не трогает слой зависимостей', () => {
    const s = run({ type: 'build' }, { type: 'edit', file: 'source' }, { type: 'build' })
    expect(s.lastBuild.find((l) => l.id === 'download')?.status).toBe('cached')
    expect(s.lastBuild.find((l) => l.id === 'build')?.status).toBe('built')
  })
  it('наивный порядок: правка исходника заново качает зависимости', () => {
    const s = run({ type: 'order', order: 'naive' }, { type: 'build' }, { type: 'edit', file: 'source' }, { type: 'build' })
    expect(s.lastBuild.find((l) => l.id === 'download')?.status).toBe('built')
  })
  it('правка go.mod инвалидирует зависимости и всё после них в стадии сборки', () => {
    const s = run({ type: 'build' }, { type: 'edit', file: 'gomod' }, { type: 'build' })
    const stage = s.lastBuild.filter((l) => l.stage === 'build')
    const i = stage.findIndex((l) => l.id === 'copy-mod')
    expect(stage.slice(0, i).every((l) => l.status === 'cached')).toBe(true)
    expect(stage.slice(i).every((l) => l.status === 'built')).toBe(true)
    // Новый бинарь — новый слой COPY --from, а FROM финальной стадии остаётся в кэше.
    expect(s.lastBuild.find((l) => l.id === 'runtime')?.status).toBe('cached')
    expect(s.lastBuild.find((l) => l.id === 'copy-bin')?.status).toBe('built')
  })
  it('README без .dockerignore инвалидирует COPY . ., с ним — нет', () => {
    const without = run({ type: 'dockerignore', on: false }, { type: 'build' }, { type: 'edit', file: 'readme' }, { type: 'build' })
    expect(without.lastBuild.find((l) => l.id === 'copy-src')?.status).toBe('built')
    const withIgnore = run({ type: 'build' }, { type: 'edit', file: 'readme' }, { type: 'build' })
    expect(withIgnore.lastBuild.every((l) => l.status === 'cached')).toBe(true)
  })
  it('cache mount ускоряет пересобранный шаг сборки, но не делает его кэшированным', () => {
    const on = run({ type: 'build' }, { type: 'edit', file: 'source' }, { type: 'build' })
    const off = run({ type: 'cacheMount', on: false }, { type: 'build' }, { type: 'edit', file: 'source' }, { type: 'build' })
    const t = (s: typeof on) => s.lastBuild.find((l) => l.id === 'build')?.seconds ?? 0
    expect(t(on)).toBeLessThan(t(off))
  })
  it('итоговый образ — только слои финальной стадии', () => {
    const s = run({ type: 'build' })
    expect(s.imageMB).toBeLessThan(30)
    expect(run({ type: 'stage', multi: false }, { type: 'build' }).imageMB).toBeGreaterThan(800)
  })
  it('каждый шаг подписан, подписи с терминами', () => {
    const steps = sim.steps(run({ type: 'build' }))
    expect(steps).toHaveLength(1)
    expect(JSON.stringify(steps[0]?.caption)).toContain('build-cache')
  })
})
```

- [ ] **Step 2: Run** `npx vitest run viz/simulations/layer-cache` — Expected: FAIL (модуля нет).

- [ ] **Step 3: Реализация `layer-cache.ts`**

```ts
import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

// Модель кэша BuildKit, достаточная для урока: ключ слоя = инструкция + ключ
// предыдущего слоя + (для COPY) содержимое скопированных файлов. Слой берётся из кэша,
// только если совпал ключ его и всех слоёв до него.

export type FileId = 'gomod' | 'source' | 'readme'
export type LayerId = 'base' | 'workdir' | 'copy-mod' | 'download' | 'copy-src' | 'build' | 'runtime' | 'copy-bin'
export type Order = 'correct' | 'naive'
export type LayerStatus = 'built' | 'cached'

export interface Layer {
  id: LayerId
  instruction: string
  stage: 'build' | 'runtime'
  status: LayerStatus
  seconds: number
}

export interface LayerState {
  order: Order
  dockerignore: boolean
  cacheMount: boolean
  multiStage: boolean
  versions: Readonly<Record<FileId, number>> // номер правки каждого файла
  cache: ReadonlySet<string> // ключи слоёв, уже лежащих в кэше
  lastBuild: readonly Layer[]
  imageMB: number
  builds: number
  last: LayerAction['type'] | 'start'
}

export type LayerAction =
  | { type: 'build' }
  | { type: 'edit'; file: FileId }
  | { type: 'order'; order: Order }
  | { type: 'dockerignore'; on: boolean }
  | { type: 'cacheMount'; on: boolean }
  | { type: 'stage'; multi: boolean }
  | { type: 'reset' }

interface Instr {
  id: LayerId
  text: string
  stage: 'build' | 'runtime'
  copies: readonly FileId[] // какие файлы входят в ключ слоя
  seconds: number // время сборки шага без кэша
  mb: number // вклад в размер, если слой попадёт в итоговый образ
}

// Время и размеры — порядок величин для Go-сервиса средних размеров, не замер.
const BASE: Instr = { id: 'base', text: 'FROM golang:1.26 AS build', stage: 'build', copies: [], seconds: 20, mb: 850 }
const WORKDIR: Instr = { id: 'workdir', text: 'WORKDIR /src', stage: 'build', copies: [], seconds: 0.1, mb: 0 }
const COPY_MOD: Instr = { id: 'copy-mod', text: 'COPY go.mod go.sum ./', stage: 'build', copies: ['gomod'], seconds: 0.1, mb: 0 }
const DOWNLOAD: Instr = { id: 'download', text: 'RUN go mod download', stage: 'build', copies: [], seconds: 25, mb: 120 }
const BUILD: Instr = { id: 'build', text: 'RUN go build -o /out/messenger ./cmd/messenger', stage: 'build', copies: [], seconds: 40, mb: 300 }
const RUNTIME: Instr = { id: 'runtime', text: 'FROM gcr.io/distroless/static-debian13:nonroot', stage: 'runtime', copies: [], seconds: 3, mb: 2 }
const COPY_BIN: Instr = { id: 'copy-bin', text: 'COPY --from=build /out/messenger /messenger', stage: 'runtime', copies: [], seconds: 0.2, mb: 15 }

const copySrc = (ignore: boolean): Instr => ({
  id: 'copy-src',
  text: 'COPY . .',
  stage: 'build',
  copies: ignore ? ['gomod', 'source'] : ['gomod', 'source', 'readme'],
  seconds: 0.5,
  mb: 0,
})

function program(s: Pick<LayerState, 'order' | 'dockerignore' | 'multiStage'>): Instr[] {
  const src = copySrc(s.dockerignore)
  const build = s.order === 'correct' ? [BASE, WORKDIR, COPY_MOD, DOWNLOAD, src, BUILD] : [BASE, WORKDIR, src, DOWNLOAD, BUILD]
  return s.multiStage ? [...build, RUNTIME, COPY_BIN] : build
}

// Пересобранный go build с cache mount переиспользует кэш компилятора: пакеты, которые
// не менялись, не компилируются заново. Слой всё равно новый — меняется его результат.
const CACHE_MOUNT_SPEEDUP = 0.15

function build(s: LayerState): LayerState {
  const cache = new Set(s.cache)
  let parent = ''
  const layers: Layer[] = []
  for (const ins of program(s)) {
    // Новая стадия начинается со своего FROM: её ключ не зависит от предыдущей стадии.
    if (ins.id === 'runtime') parent = ''
    const copied = ins.copies.map((f) => `${f}@${s.versions[f]}`).join(',')
    // COPY --from сравнивает содержимое копируемого файла. Бинарь определяют go.mod и
    // исходники (сборка с -trimpath воспроизводима), а не то, пересобирался ли слой.
    const bin = ins.id === 'copy-bin' ? `|bin@${s.versions.gomod}.${s.versions.source}` : ''
    // Ключ включает ключ родителя: промах в любом слое выше даёт промах во всех ниже.
    const key = `${parent}|${ins.text}|${copied}${bin}`
    const hit = cache.has(key)
    cache.add(key)
    const seconds = hit ? 0 : ins.id === 'build' && s.cacheMount && s.builds > 0 ? ins.seconds * CACHE_MOUNT_SPEEDUP : ins.seconds
    layers.push({ id: ins.id, instruction: ins.text, stage: ins.stage, status: hit ? 'cached' : 'built', seconds })
    parent = key
  }
  const finalStage = s.multiStage ? program(s).filter((i) => i.stage === 'runtime') : program(s)
  const imageMB = Math.round(finalStage.reduce((sum, i) => sum + i.mb, 0))
  return { ...s, cache, lastBuild: layers, imageMB, builds: s.builds + 1, last: 'build' }
}

const start = (): LayerState => ({
  order: 'correct',
  dockerignore: true,
  cacheMount: true,
  multiStage: true,
  versions: { gomod: 0, source: 0, readme: 0 },
  cache: new Set(),
  lastBuild: [],
  imageMB: 0,
  builds: 0,
  last: 'start',
})

export const layerCacheSimulation: Simulation<LayerState, LayerAction> = {
  initial: start,
  apply: (s, a) => {
    switch (a.type) {
      case 'reset':
        return start()
      case 'build':
        return build(s)
      case 'edit':
        return { ...s, versions: { ...s.versions, [a.file]: s.versions[a.file] + 1 }, last: 'edit' }
      case 'order':
        return { ...s, order: a.order, last: 'order' }
      case 'dockerignore':
        return { ...s, dockerignore: a.on, last: 'dockerignore' }
      case 'cacheMount':
        return { ...s, cacheMount: a.on, last: 'cacheMount' }
      case 'stage':
        return { ...s, multiStage: a.multi, last: 'stage' }
    }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<LayerState>],
}

function captionFor(s: LayerState): Caption {
  if (s.lastBuild.length === 0)
    return [{ text: 'Соберите образ, поменяйте файл и соберите ещё раз: ' }, term('кэш сборки', 'build-cache'), { text: ' переиспользует слой, только если не изменились он сам и все слои до него.' }]
  const rebuilt = s.lastBuild.filter((l) => l.status === 'built')
  const total = s.lastBuild.reduce((sum, l) => sum + l.seconds, 0)
  if (rebuilt.length === 0) return [{ text: `Все слои взяты из ` }, term('кэша сборки', 'build-cache'), { text: `. Сборка заняла ~0 с, образ ${s.imageMB} МБ.` }]
  const first = rebuilt[0] as Layer
  return [
    { text: `Первый пересобранный слой — «${first.instruction}»; после него пересобирается всё. ` },
    { text: `Сборка ~${Math.round(total)} с, ` },
    term('образ', 'container-image'),
    { text: ` ${s.imageMB} МБ.` },
  ]
}
```

- [ ] **Step 4: Run** `npx vitest run viz/simulations/layer-cache` — Expected: PASS. Если какой-то тест расходится с моделью, править **модель**, а не ослаблять тест; ключевые утверждения тестов — из спеки §7.1.

- [ ] **Step 5: Тесты `supply-chain`**

```ts
import { describe, expect, it } from 'vitest'
import { supplyChainSimulation as sim, type ChainAction } from './supply-chain'

const run = (...actions: ChainAction[]) => actions.reduce(sim.apply, sim.initial())

describe('supply-chain', () => {
  it('без атаки выполняется исходный код всех actions', () => {
    const s = run({ type: 'run' })
    expect(s.result?.executed.every((e) => !e.malicious)).toBe(true)
    expect(s.result?.leaked).toEqual([])
  })
  it('закрепление по тегу: подменённый тег выполняет код злоумышленника', () => {
    const s = run({ type: 'pin', mode: 'tag' }, { type: 'attack' }, { type: 'run' })
    expect(s.result?.executed.some((e) => e.malicious)).toBe(true)
  })
  it('закрепление по SHA: подмена тега не влияет', () => {
    const s = run({ type: 'pin', mode: 'sha' }, { type: 'attack' }, { type: 'run' })
    expect(s.result?.executed.every((e) => !e.malicious)).toBe(true)
    expect(s.result?.leaked).toEqual([])
  })
  it('тег + write-all + секрет в env: утекают токен с правом записи и секрет', () => {
    const s = run({ type: 'pin', mode: 'tag' }, { type: 'permissions', mode: 'write-all' }, { type: 'secret', inEnv: true }, { type: 'attack' }, { type: 'run' })
    expect(s.result?.leaked).toEqual(expect.arrayContaining(['GITHUB_TOKEN: contents:write', 'DEPLOY_KEY']))
  })
  it('тег + минимальные права: утекает только токен на чтение', () => {
    const s = run({ type: 'pin', mode: 'tag' }, { type: 'permissions', mode: 'minimal' }, { type: 'secret', inEnv: false }, { type: 'attack' }, { type: 'run' })
    expect(s.result?.leaked).toEqual(['GITHUB_TOKEN: contents:read'])
  })
  it('подпись называет, что спасло или что утекло', () => {
    const s = run({ type: 'pin', mode: 'sha' }, { type: 'attack' }, { type: 'run' })
    expect(JSON.stringify(sim.steps(s)[0]?.caption)).toContain('action-pinning')
  })
})
```

- [ ] **Step 6: Run** `npx vitest run viz/simulations/supply-chain` — Expected: FAIL.

- [ ] **Step 7: Реализация `supply-chain.ts`**

```ts
import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

// Модель атаки марта 2025 года на tj-actions/changed-files: злоумышленник перевесил
// теги действия на свой коммит, и все workflow, закреплённые по тегу, выполнили его код.
// Код печатал секреты раннера в журнал сборки.

export type PinMode = 'tag' | 'sha'
export type PermMode = 'write-all' | 'minimal'

export interface ActionRef {
  name: string
  ref: string // что написано в uses:
}

export interface Executed {
  name: string
  malicious: boolean
}

export interface RunResult {
  executed: readonly Executed[]
  leaked: readonly string[]
}

export interface ChainState {
  pin: PinMode
  permissions: PermMode
  secretInEnv: boolean
  attacked: boolean
  actions: readonly ActionRef[]
  result: RunResult | null
  last: ChainAction['type'] | 'start'
}

export type ChainAction =
  | { type: 'pin'; mode: PinMode }
  | { type: 'permissions'; mode: PermMode }
  | { type: 'secret'; inEnv: boolean }
  | { type: 'attack' }
  | { type: 'run' }
  | { type: 'reset' }

const VICTIM = 'changed-files'

function refs(pin: PinMode): ActionRef[] {
  return pin === 'tag'
    ? [
        { name: 'checkout', ref: 'actions/checkout@v7' },
        { name: VICTIM, ref: 'example/changed-files@v46' },
        { name: 'setup-go', ref: 'actions/setup-go@v7' },
      ]
    : [
        { name: 'checkout', ref: 'actions/checkout@3d3c42e5… # v7.0.1' },
        { name: VICTIM, ref: 'example/changed-files@a1b2c3d4… # v46.0.1' },
        { name: 'setup-go', ref: 'actions/setup-go@b7ad1dad… # v7.0.0' },
      ]
}

function execute(s: ChainState): RunResult {
  // Тег — изменяемая ссылка: после подмены он указывает на коммит злоумышленника.
  // SHA — неизменяемая: тот же коммит, что был при ревью.
  const executed = s.actions.map((a) => ({ name: a.name, malicious: s.attacked && s.pin === 'tag' && a.name === VICTIM }))
  if (!executed.some((e) => e.malicious)) return { executed, leaked: [] }
  const leaked = [`GITHUB_TOKEN: ${s.permissions === 'write-all' ? 'contents:write' : 'contents:read'}`]
  if (s.secretInEnv) leaked.push('DEPLOY_KEY')
  return { executed, leaked }
}

const start = (): ChainState => ({ pin: 'tag', permissions: 'write-all', secretInEnv: true, attacked: false, actions: refs('tag'), result: null, last: 'start' })

export const supplyChainSimulation: Simulation<ChainState, ChainAction> = {
  initial: start,
  apply: (s, a) => {
    switch (a.type) {
      case 'reset':
        return start()
      case 'pin':
        return { ...s, pin: a.mode, actions: refs(a.mode), result: null, last: 'pin' }
      case 'permissions':
        return { ...s, permissions: a.mode, result: null, last: 'permissions' }
      case 'secret':
        return { ...s, secretInEnv: a.inEnv, result: null, last: 'secret' }
      case 'attack':
        return { ...s, attacked: true, result: null, last: 'attack' }
      case 'run':
        return { ...s, result: execute(s), last: 'run' }
    }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<ChainState>],
}

function captionFor(s: ChainState): Caption {
  if (s.result === null)
    return [{ text: 'Настройте workflow, подмените тег одного action и запустите. Это модель реальной ' }, term('атаки на цепочку поставок', 'supply-chain-attack'), { text: ' 2025 года.' }]
  if (s.result.leaked.length === 0 && s.attacked)
    return [term('Закрепление по SHA', 'action-pinning'), { text: ': подменённый тег не изменил коммит, который выполнился. Ничего не утекло.' }]
  if (s.result.leaked.length === 0) return 'Атаки не было: выполнился исходный код всех actions.'
  return [
    { text: `Выполнился код злоумышленника. Утекло: ${s.result.leaked.join(', ')}. ` },
    term('Минимальные права', 'least-privilege'),
    { text: ' сужают ущерб, но не предотвращают атаку.' },
  ]
}
```

- [ ] **Step 8: Run** `npx vitest run viz/simulations/supply-chain` — Expected: PASS.

- [ ] **Step 9: Рендереры и сцены (контракт)**

`LayerCacheView.vue` (`props: { state: LayerState }`):
- две колонки «стадия сборки» и «итоговый образ» (на 375 px — одна под другой); каждый слой — строка с инструкцией моноширинным шрифтом, плашкой статуса (`CACHED` — `--viz-ok`, `BUILT` — `--viz-warn`) и временем;
- над слоями — «время сборки ~N с» и «размер образа N МБ» крупно;
- справа (или ниже) — три файла (`go.mod`, `main.go`, `README.md`) с номером правки; файл, изменённый с прошлой сборки, подсвечен;
- статус передаётся не только цветом (текст плашки).

`LayerCacheSandbox.vue` — как `IdempotencySandbox.vue`: `VizFrame label="Песочница: кэш слоёв Docker"`, вступление одной фразой, группы кнопок:
- «Собрать»; «Изменить main.go», «Изменить go.mod», «Изменить README»;
- переключатели (role="radio"/aria-checked, как в образце): порядок «COPY go.mod раньше / COPY . . сразу», «.dockerignore вкл/выкл», «cache mount вкл/выкл», «multi-stage вкл/выкл»;
- «Сброс»; ниже `Player` с `LayerCacheView`.

`SupplyChainView.vue` (`props: { state: ChainState }`):
- блок «workflow»: три строки `uses:` (подменённая при `attacked && pin==='tag'` — с меткой «тег перевешен»), строка `permissions:`, строка `env: DEPLOY_KEY` (если `secretInEnv`);
- блок «что выполнилось»: три плашки, вредоносная — `--viz-bad` с текстом «код злоумышленника»;
- блок «что утекло»: список или «ничего».

`SupplyChainSandbox.vue`: `VizFrame label="Песочница: атака на цепочку поставок"`, переключатели «закрепление: тег / SHA», «permissions: write-all / минимальные», «секрет в env: да / нет», кнопки «Подменить тег», «Запустить workflow», «Сброс».

Оба рендерера — палитра `--viz-*`, обе темы, контраст ≥ 4,5:1, без горизонтальной прокрутки на 375 px, `prefers-reduced-motion` (переходы отключаются).

- [ ] **Step 10: Проверить**

Run: `npm test && npm run typecheck`
Expected: PASS.

- [ ] **Step 11: Commit**

```bash
git add course/viz
git commit -m "feat(viz): песочницы layer-cache и supply-chain

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Плееры `container-start`, `cgroup-limits`, `graceful-shutdown`, `gomemlimit`, `ci-graph`

**Files:**
- Create: `course/viz/simulations/{container-start,cgroup-limits,graceful-shutdown,gomemlimit,ci-graph}.ts` и `.test.ts` к каждому
- Create: `course/viz/renderers/{ContainerStart,CgroupLimits,GracefulShutdown,GoMemLimit,CiGraph}View.vue`
- Create: `course/viz/scenes/{ContainerStart,CgroupLimits,GracefulShutdown,GoMemLimit,CiGraph}.vue`

**Interfaces:**
- Consumes: `Scenario`, `Step`, `term()`, `Player` (в том числе вариант со сценариями-вкладками, как в `request-pipeline` 1b — посмотреть `RequestPipeline.vue` и `request-pipeline.ts`), `VizFrame`; id глоссария задачи 1.
- Produces: сцены для уроков 1.15 (`ContainerStart`, `CgroupLimits`), 1.17 (`GracefulShutdown`, `GoMemLimit`), 1.18 (`CiGraph`).

Каждый модуль экспортирует функцию(и) сценария `xxxScenario(variant?): readonly Step<XxxFrame>[]` — чистые, детерминированные. Ниже для каждого — **контракт кадров** и **обязательные тесты** (их писать дословно по смыслу; формулировки `it` — по-русски).

- [ ] **Step 1: `container-start` (1.15)**

Кадр: `{ phase, host: ProcNode[], container: ProcNode[] | null, mounts: { root: string, proc: boolean, oldroot: boolean } , hostname: { host: string, container: string | null } }`, где `ProcNode = { pid: number, nsPid?: number, cmd: string }`.

Фазы по порядку (по кадру на фазу): `run` (на хосте `minicontainer run`), `clone` (новые namespaces: UTS, PID, mount, net, IPC — подсветить список флагов), `child` (`/proc/self/exe child` — на хосте PID, например 4242, в контейнере PID 1), `hostname`, `private-mounts`, `pivot-root` (корень = rootfs, старый корень в `/.oldroot`), `umount-oldroot` (старый корень отцеплен), `mount-proc`, `exec` (PID 1 теперь `sh`; на хосте тот же PID 4242 показывает `sh`).

Тесты:
- фазы идут в указанном порядке, кадров 9;
- после `child` у процесса два PID: хостовый ≠ 1 и `nsPid === 1`;
- `mounts.oldroot` истинно только между `pivot-root` и `umount-oldroot`;
- `hostname.container` появляется на фазе `hostname`, `hostname.host` не меняется ни в одном кадре;
- в подписях встречаются термины `namespace`, `pivot-root`, `rootfs`, `pid-1`.

Рендерер: два дерева (хост слева, контейнер справа; на 375 px — одно под другим), под ними — строка монтирования и hostname; текущая фаза подсвечена в ленте фаз сверху.

- [ ] **Step 2: `cgroup-limits` (1.15)** — два варианта: `memory` и `cpu`.

`memory`: кадр `{ usageMiB, maxMiB: 64, reclaimed: boolean, oomKilled: boolean, events: { oom: number, oom_kill: number } }`. Кадры: рост 16 → 32 → 48 → 60 МиБ; на ~60 — ядро пытается освободить кэш страниц (`reclaimed`); 64 — `oom` 1; процесс убит: `oomKilled`, `oom_kill: 1`, код выхода 137.
`cpu`: кадр `{ quotaUs: 50000, periodUs: 100000, periods: { used: number, throttled: boolean }[] }`. Процесс хочет 100 % одного ядра: в каждом периоде 100 мс использует 50 мс и ждёт остаток (`throttled: true`); 4–5 кадров, в последнем — `nr_throttled` и `throttled_usec` из `cpu.stat`.

Тесты:
- `memory`: `usageMiB` не убывает до OOM; `oom_kill` = 1 только в последнем кадре; ни в одном кадре `usageMiB > maxMiB`;
- `cpu`: в каждом периоде `used ≤ quotaUs`; при спросе 100 % все периоды `throttled`; итоговая доля CPU = 50 %;
- подписи содержат термины `cgroup`, `oom-kill` (memory) и `cpu-throttling` (cpu).

- [ ] **Step 3: `graceful-shutdown` (1.17)** — варианты `exec-form` и `wrapper-script`.

Кадр: `{ t: number /* секунды от SIGTERM */, process: 'running' | 'draining' | 'shutting-down' | 'exited' | 'killed', readyz: 200 | 503 | null, inLb: boolean, inflight: number, newRequests: 'served' | 'refused' | 'none', signalAt: 'pid1-app' | 'pid1-sh' }`.

`exec-form` (значения как в мессенджере: `DRAIN_DELAY` 5 с, `SHUTDOWN_TIMEOUT` 10 с, grace period 30 с, проба раз в 1 с с порогом 1):
1. t=0 SIGTERM пришёл приложению (PID 1); `readyz` 503, `process: draining`, `inLb: true`, новые запросы обслуживаются;
2. t=1 балансировщик увидел 503 → `inLb: false`;
3. t=5 конец дренажа → `shutting-down`: новые соединения не принимаются, `inflight` уменьшается;
4. t≈6 последний запрос ответил → `exited`, код 0.

`wrapper-script` (`ENTRYPOINT ["/start.sh"]`, где скрипт запускает `messenger` **без** `exec` → PID 1 — `sh`, приложение — его ребёнок):
1. t=0 SIGTERM пришёл `sh`; `sh` как PID 1 без своего обработчика его игнорирует и детям не пересылает → приложение ничего не знает, `readyz` 200;
2. t=10 (`docker stop` по умолчанию ждёт 10 с) → SIGKILL, `process: killed`, `inflight` обрывается (клиенты получают сброс соединения).

Тесты:
- `exec-form`: `readyz === 503` с первого кадра; `inLb` становится `false` раньше, чем `process === 'shutting-down'`; последний кадр `exited`; `t` последнего < 30;
- `wrapper-script`: `readyz` остаётся 200 до `killed`; последний кадр `killed` при `t === 10`;
- подписи содержат `graceful-shutdown`, `readiness-probe`, `pid-1`.

- [ ] **Step 4: `gomemlimit` (1.17)** — варианты `without` и `with`.

Кадр: `{ t, heapLiveMiB, heapGoalMiB, rssMiB, limitMiB: 256, gcRuns, killed }`. Модель: живая куча растёт до 120 МиБ и держится; без `GOMEMLIMIT` цель GC = живая × 2 (GOGC=100), RSS идёт к цели плюс накладные ~10 % → на пике выделений превышает 256 → `killed`. С `GOMEMLIMIT=230MiB` цель ограничена 230, GC запускается чаще (`gcRuns` растёт быстрее), RSS ≤ 256, процесс жив.

Тесты:
- `without`: в каком-то кадре `heapGoalMiB > limitMiB`, последний кадр `killed`;
- `with`: во всех кадрах `heapGoalMiB ≤ 230` и `rssMiB ≤ limitMiB`, последний кадр не `killed`; итоговое `gcRuns` больше, чем у `without` на том же отрезке времени;
- подписи содержат `gomemlimit`, `oom-kill`.

Числа — модель, в подписи один раз сказано, что это упрощение (`Callout` не нужен — достаточно слов «упрощённая модель» в первой подписи).

- [ ] **Step 5: `ci-graph` (1.18)** — варианты `aggregate` и `on-paths`.

Факт GitHub (Docs, «Troubleshooting required status checks»): джоба, пропущенная по условию `if:`, отчитывается как **Success**; а workflow, пропущенный целиком фильтром `on.<event>.paths`/`branches` или `[skip ci]`, не создаёт проверок вовсе — обязательные проверки остаются **Pending** и блокируют PR.

Кадр: `{ jobs: { id: string, status: 'pending' | 'running' | 'success' | 'failure' | 'skipped' }[], required: string[], pr: 'waiting' | 'mergeable' | 'blocked' }`. Джобы как в `ci.yml` задачи 9: `changes`, `messenger-lint`, `messenger-test`, `messenger-generate`, `messenger-vuln`, `course`, `labs`, `image`, `workflows`, `ci-ok`. Сценарий: PR меняет только урок (`course/**`).

`aggregate` (обязательная — `ci-ok`): `changes` → все `messenger-*`, `labs`, `image`, `workflows` — `skipped`, `course` — `running` → `success` → `ci-ok` `success` → `pr: mergeable`.
`on-paths` (фильтр на уровне workflow: `on.pull_request.paths: ['messenger/**']`, обязательные — джобы мессенджера): PR меняет только урок → workflow не запускается вовсе, джобы остаются `pending` во всех кадрах, последний кадр `pr: waiting` (подпись: «Expected — Waiting for status to be reported», как пишет GitHub). Поэтому фильтр — на уровне джоб (`paths-filter` + `if:`), а не `on.paths`.

Тесты:
- `aggregate`: последний кадр `mergeable`; `required` = `['ci-ok']`; ни одна `messenger-*` не `running` ни в одном кадре;
- `on-paths`: ни одна джоба не выходит из `pending`; последний кадр `waiting`, а не `mergeable`;
- `ci-ok` в `aggregate` становится `success` только после того, как все остальные — `success` или `skipped`;
- подписи содержат `required-check`, `ci-job`, `monorepo`.

- [ ] **Step 6: Рендереры и сцены (общий контракт)**

- Каждая сцена: `VizFrame` с осмысленным `label`, `Player` со сценарием; для сценариев с вариантами — переключатель вариантов (как в `RequestPipeline.vue`), переключение сбрасывает плеер на первый кадр.
- `GracefulShutdownView`: шкала времени 0–30 с с отметками SIGTERM, «LB убрал», конец дренажа, выход/SIGKILL; ниже — состояние процесса, `/readyz`, счётчик начатых запросов.
- `GoMemLimitView`: SVG-график (живая куча, цель GC, RSS, линия лимита), ось времени; в обоих вариантах масштаб одинаковый; на 375 px график масштабируется по ширине (`viewBox`, без фиксированной ширины).
- `CiGraphView`: граф джоб слоями (`changes` → ворота → `ci-ok`), статус — цветом **и** текстом/значком; плашка состояния PR.
- `ContainerStartView`, `CgroupLimitsView` — по контрактам шагов 1–2.

- [ ] **Step 7: Проверить**

Run: `npm test && npm run typecheck`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add course/viz
git commit -m "feat(viz): плееры container-start, cgroup-limits, graceful-shutdown, gomemlimit, ci-graph

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
### Task 4: Лаба «контейнер с нуля» (`labs/1c`)

**Files:**
- Create: `labs/1c/cmd/minicontainer/{main.go,flags.go,flags_test.go,cgroup.go,cgroup_test.go}`, `labs/1c/cmd/hog/main.go`
- Create: `labs/1c/{Dockerfile,compose.yaml,entrypoint.sh,scenario.sh,README.md}`
- Modify: `course/scripts/check-labs.ts`

**Interfaces:**
- Produces: `minicontainer run [--mem 64M] [--pids 32] [--cpu 50%] [--hostname box] <rootfs> <команда> [аргументы…]`; итоговая строка в stderr `minicontainer: итог: oom_kill=<n> pids_max=<n>`; образ `lab1c:local`; rootfs в образе — `/rootfs`; сценарий `/lab/scenario.sh`. Урок 1.15 (задача 5) цитирует регионы `clone`, `child`, `cgroup-limits` и `entrypoint`.

- [ ] **Step 1: Тесты разбора флагов**

`labs/1c/cmd/minicontainer/flags_test.go`:

```go
//go:build linux

package main

import (
	"slices"
	"testing"
)

func TestParseBytes(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"64M", 64 << 20, false},
		{"512K", 512 << 10, false},
		{"1G", 1 << 30, false},
		{"1048576", 1 << 20, false},
		{"64m", 64 << 20, false},
		{"", 0, true},
		{"0", 0, true},
		{"-5M", 0, true},
		{"12X", 0, true},
		{"M", 0, true},
		{"100", 0, true}, // меньше минимума: ядро округлит до страницы, а Go-рантайм не стартует
	}
	for _, c := range cases {
		got, err := parseBytes(c.in)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("parseBytes(%q) = %d, %v; ждали %d, ошибка=%v", c.in, got, err, c.want, c.wantErr)
		}
	}
}

func TestParseCPU(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"50%", "50000 100000", false},
		{"150%", "150000 100000", false},
		{"1%", "1000 100000", false},
		{"0%", "", true},
		{"50", "", true},
		{"abc%", "", true},
		{"100001%", "", true},
	}
	for _, c := range cases {
		got, err := parseCPU(c.in)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("parseCPU(%q) = %q, %v; ждали %q, ошибка=%v", c.in, got, err, c.want, c.wantErr)
		}
	}
}

func TestParseRun(t *testing.T) {
	cfg, err := parseRun([]string{"--mem", "32M", "--pids", "16", "--cpu", "25%", "--hostname", "lab-box", "/rootfs", "sh", "-c", "echo hi"})
	if err != nil {
		t.Fatal(err)
	}
	want := runConfig{
		Hostname: "lab-box",
		Rootfs:   "/rootfs",
		Argv:     []string{"sh", "-c", "echo hi"},
		Limits:   limits{MemoryMax: "33554432", PidsMax: "16", CPUMax: "25000 100000"},
	}
	if cfg.Hostname != want.Hostname || cfg.Rootfs != want.Rootfs || !slices.Equal(cfg.Argv, want.Argv) || cfg.Limits != want.Limits {
		t.Fatalf("cfg = %+v, ждали %+v", cfg, want)
	}

	defaults, err := parseRun([]string{"/rootfs", "sh"})
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Limits != (limits{MemoryMax: "67108864", PidsMax: "32", CPUMax: "50000 100000"}) || defaults.Hostname != "box" {
		t.Fatalf("значения по умолчанию: %+v", defaults)
	}

	for _, bad := range [][]string{
		{},
		{"/rootfs"},
		{"--pids", "0", "/rootfs", "sh"},
		{"--hostname", "", "/rootfs", "sh"},
		{"--mem", "много", "/rootfs", "sh"},
		{"--нет-такого", "/rootfs", "sh"},
	} {
		if _, err := parseRun(bad); err == nil {
			t.Errorf("parseRun(%q): ждали ошибку", bad)
		}
	}
}
```

- [ ] **Step 2: Run** `cd labs && go test ./1c/...` — Expected: FAIL (нет функций).

- [ ] **Step 3: `flags.go`**

```go
//go:build linux

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Лимиты — строки ровно в том виде, в каком их ждут файлы cgroup v2.
type limits struct {
	MemoryMax string // memory.max: байты
	PidsMax   string // pids.max: число задач (процессов и потоков)
	CPUMax    string // cpu.max: "<квота мкс> <период мкс>"
}

type runConfig struct {
	Hostname string
	Rootfs   string
	Argv     []string
	Limits   limits
}

const (
	// cpuPeriodUS — период планировщика CFS по умолчанию в ядре (100 мс): квота
	// «50%» — это 50 мс работы в каждые 100 мс.
	cpuPeriodUS = 100_000
	// maxCPUPercent — 1000 ядер; больше — почти наверняка опечатка.
	maxCPUPercent = 100_000
	// minMemory — меньше 4 МиБ не стартует даже рантайм Go, а ядро всё равно
	// округляет лимит до страницы.
	minMemory = 4 << 20
	// maxHostname — HOST_NAME_MAX в Linux.
	maxHostname = 64
)

var errUsage = errors.New("нужно: run [--mem 64M] [--pids 32] [--cpu 50%] [--hostname box] <rootfs> <команда> [аргументы…]")

func parseRun(args []string) (runConfig, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	mem := fs.String("mem", "64M", "лимит памяти: 512K, 64M, 1G")
	pids := fs.Int("pids", 32, "лимит числа процессов и потоков")
	cpu := fs.String("cpu", "50%", "доля одного ядра: 50%, 150%")
	host := fs.String("hostname", "box", "имя хоста в контейнере")
	if err := fs.Parse(args); err != nil {
		return runConfig{}, fmt.Errorf("флаги: %w", err)
	}
	rest := fs.Args()
	if len(rest) < 2 {
		return runConfig{}, errUsage
	}
	memBytes, err := parseBytes(*mem)
	if err != nil {
		return runConfig{}, fmt.Errorf("--mem: %w", err)
	}
	cpuMax, err := parseCPU(*cpu)
	if err != nil {
		return runConfig{}, fmt.Errorf("--cpu: %w", err)
	}
	if *pids < 1 {
		return runConfig{}, fmt.Errorf("--pids: нужно не меньше 1, получено %d", *pids)
	}
	if *host == "" || len(*host) > maxHostname {
		return runConfig{}, fmt.Errorf("--hostname: длина от 1 до %d байт", maxHostname)
	}
	return runConfig{
		Hostname: *host,
		Rootfs:   rest[0],
		Argv:     rest[1:],
		Limits:   limits{MemoryMax: strconv.FormatInt(memBytes, 10), PidsMax: strconv.Itoa(*pids), CPUMax: cpuMax},
	}, nil
}

// parseBytes понимает двоичные суффиксы K, M, G (как docker run --memory).
func parseBytes(s string) (int64, error) {
	mult := int64(1)
	num := strings.ToUpper(s)
	switch {
	case strings.HasSuffix(num, "K"):
		mult, num = 1<<10, strings.TrimSuffix(num, "K")
	case strings.HasSuffix(num, "M"):
		mult, num = 1<<20, strings.TrimSuffix(num, "M")
	case strings.HasSuffix(num, "G"):
		mult, num = 1<<30, strings.TrimSuffix(num, "G")
	}
	n, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q — не размер: %w", s, err)
	}
	if n <= 0 || n > (1<<62)/mult {
		return 0, fmt.Errorf("%q — вне диапазона", s)
	}
	if n*mult < minMemory {
		return 0, fmt.Errorf("%q — меньше минимума %d МиБ", s, minMemory>>20)
	}
	return n * mult, nil
}

func parseCPU(s string) (string, error) {
	pct, ok := strings.CutSuffix(s, "%")
	if !ok {
		return "", fmt.Errorf("%q — нужен процент, например 50%%", s)
	}
	n, err := strconv.Atoi(pct)
	if err != nil {
		return "", fmt.Errorf("%q — не число: %w", s, err)
	}
	if n < 1 || n > maxCPUPercent {
		return "", fmt.Errorf("%q — нужно от 1%% до %d%%", s, maxCPUPercent)
	}
	return fmt.Sprintf("%d %d", n*cpuPeriodUS/100, cpuPeriodUS), nil
}
```

- [ ] **Step 4: Тесты cgroup**

`labs/1c/cmd/minicontainer/cgroup_test.go` (на обычных файлах во временном каталоге — у файлов cgroup та же форма «ключ значение»):

```go
//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckControllers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cgroup.subtree_control"), "cpu memory pids\n")
	if err := checkControllers(root); err != nil {
		t.Fatalf("все контроллеры включены: %v", err)
	}
	writeFile(t, filepath.Join(root, "cgroup.subtree_control"), "cpu io\n")
	err := checkControllers(root)
	if err == nil || !strings.Contains(err.Error(), "memory") {
		t.Fatalf("без memory: ошибка %v, ждали упоминание memory", err)
	}
	if err := checkControllers(filepath.Join(root, "нет")); err == nil {
		t.Fatal("нет cgroup v2: ждали ошибку")
	}
}

func TestCreateCgroupWritesLimits(t *testing.T) {
	root := t.TempDir()
	l := limits{MemoryMax: "33554432", PidsMax: "16", CPUMax: "25000 100000"}
	cg, err := createCgroup(root, "minicontainer-1", l)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cg.path) })
	for file, want := range map[string]string{"memory.max": l.MemoryMax, "memory.swap.max": "0", "pids.max": l.PidsMax, "cpu.max": l.CPUMax} {
		b, err := os.ReadFile(filepath.Join(cg.path, file))
		if err != nil || string(b) != want {
			t.Errorf("%s = %q, %v; ждали %q", file, b, err, want)
		}
	}
	if _, err := createCgroup(root, "minicontainer-1", l); err == nil {
		t.Error("каталог уже есть: ждали ошибку, а не чужие лимиты")
	}
}

func TestEvents(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "memory.events"), "low 0\nhigh 0\nmax 3\noom 1\noom_kill 1\noom_group_kill 0\n")
	cg := &cgroup{path: dir}
	ev, err := cg.events("memory.events")
	if err != nil || ev["oom_kill"] != 1 || ev["max"] != 3 {
		t.Fatalf("events = %v, %v", ev, err)
	}
	writeFile(t, filepath.Join(dir, "pids.events"), "max не-число\n")
	if _, err := cg.events("pids.events"); err == nil {
		t.Fatal("мусор в файле: ждали ошибку")
	}
}
```

- [ ] **Step 5: Run** `cd labs && go test ./1c/...` — Expected: FAIL.

- [ ] **Step 6: `cgroup.go`**

```go
//go:build linux

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const cgroupRoot = "/sys/fs/cgroup"

// requiredControllers — без них лимиты из флагов некуда записать: файлы memory.max,
// pids.max и cpu.max появляются в дочерней группе, только если контроллер включён
// в cgroup.subtree_control родителя.
var requiredControllers = []string{"cpu", "memory", "pids"}

type cgroup struct{ path string }

func checkControllers(root string) error {
	b, err := os.ReadFile(filepath.Join(root, "cgroup.subtree_control"))
	if err != nil {
		return fmt.Errorf("нет cgroup v2 в %s: %w", root, err)
	}
	enabled := strings.Fields(string(b))
	for _, c := range requiredControllers {
		if !slices.Contains(enabled, c) {
			return fmt.Errorf("контроллер %s не включён в %s/cgroup.subtree_control — запускайте лабу через docker compose: entrypoint.sh включает контроллеры", c, root)
		}
	}
	return nil
}

// #region cgroup-limits
// createCgroup создаёт группу и пишет лимиты до того, как в неё попадёт процесс:
// иначе первые миллисекунды он работал бы без ограничений.
func createCgroup(root, name string, l limits) (*cgroup, error) {
	path := filepath.Join(root, name)
	// Mkdir, а не MkdirAll: существующая группа — чужая или оставшаяся от упавшего
	// запуска, и молча писать в неё свои лимиты нельзя.
	if err := os.Mkdir(path, 0o755); err != nil {
		return nil, fmt.Errorf("создать cgroup: %w", err)
	}
	cg := &cgroup{path: path}
	settings := []struct{ file, value string }{
		{"memory.max", l.MemoryMax},
		// Без swap лимит памяти — жёсткий: процесс получит OOM-kill, а не уйдёт в своп.
		{"memory.swap.max", "0"},
		{"pids.max", l.PidsMax},
		{"cpu.max", l.CPUMax},
	}
	for _, s := range settings {
		err := os.WriteFile(filepath.Join(path, s.file), []byte(s.value), 0o644)
		// memory.swap.max нет, если ядро собрано без учёта swap: тогда и swap не грозит.
		if errors.Is(err, fs.ErrNotExist) && s.file == "memory.swap.max" {
			continue
		}
		if err != nil {
			return nil, errors.Join(fmt.Errorf("%s: %w", s.file, err), cg.remove())
		}
	}
	return cg, nil
}

// #endregion cgroup-limits

// open даёт дескриптор каталога группы для clone3(CLONE_INTO_CGROUP): ребёнок
// рождается сразу внутри группы.
func (c *cgroup) open() (*os.File, error) {
	f, err := os.Open(c.path)
	if err != nil {
		return nil, fmt.Errorf("открыть cgroup: %w", err)
	}
	return f, nil
}

// events читает файл вида «ключ значение» (memory.events, pids.events, cpu.stat).
func (c *cgroup) events(file string) (map[string]int64, error) {
	b, err := os.ReadFile(filepath.Join(c.path, file))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	out := map[string]int64{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		k, v, ok := strings.Cut(line, " ")
		if !ok {
			return nil, fmt.Errorf("%s: строка %q", file, line)
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: %q: %w", file, line, err)
		}
		out[k] = n
	}
	return out, nil
}

// removeAttempts × removeBackoff — сколько ждём, пока ядро отпустит группу: после
// выхода процесса она бывает «занята» ещё доли миллисекунды.
const (
	removeAttempts = 20
	removeBackoff  = 10 * time.Millisecond
)

// remove удаляет группу. В cgroupfs rmdir удаляет каталог вместе с файлами
// интерфейса, если в группе нет процессов.
func (c *cgroup) remove() error {
	var err error
	for range removeAttempts {
		if err = os.Remove(c.path); err == nil || errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		time.Sleep(removeBackoff)
	}
	return fmt.Errorf("удалить cgroup %s: %w", c.path, err)
}
```

- [ ] **Step 7: Run** `cd labs && go test ./1c/...` — Expected: PASS.

- [ ] **Step 8: `main.go` (run и child)**

```go
//go:build linux

// Command minicontainer — учебный контейнер на пару сотен строк: те же namespaces,
// pivot_root и cgroup v2, что использует runc, без всего остального (урок 1.15).
// Работает только от root и только внутри окружения лабы (docker compose).
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "minicontainer:", errUsage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		code, err := run(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "minicontainer:", err)
		}
		os.Exit(code)
	case "child":
		// child возвращается, только если не смог стать командой пользователя.
		err := child(os.Args[2:])
		fmt.Fprintln(os.Stderr, "minicontainer child:", err)
		os.Exit(1)
	default:
		fmt.Fprintln(os.Stderr, "minicontainer:", errUsage)
		os.Exit(2)
	}
}

// #region clone
// run — родитель: создаёт cgroup, запускает себя же в новых namespaces и ждёт.
// Двухступенчатый запуск — как у runc: часть настройки (hostname, монтирование)
// можно сделать только изнутри новых namespaces, то есть в уже запущенном ребёнке.
func run(args []string) (int, error) {
	cfg, err := parseRun(args)
	if err != nil {
		return 2, err
	}
	if os.Geteuid() != 0 {
		return 1, errors.New("нужен root: запускайте внутри контейнера лабы (docker compose run lab)")
	}
	if err := checkControllers(cgroupRoot); err != nil {
		return 1, err
	}
	rootfs, err := filepath.Abs(cfg.Rootfs)
	if err != nil {
		return 1, fmt.Errorf("rootfs: %w", err)
	}
	cg, err := createCgroup(cgroupRoot, fmt.Sprintf("minicontainer-%d", os.Getpid()), cfg.Limits)
	if err != nil {
		return 1, err
	}
	defer func() {
		if err := cg.remove(); err != nil {
			fmt.Fprintln(os.Stderr, "minicontainer:", err)
		}
	}()
	dir, err := cg.open()
	if err != nil {
		return 1, err
	}
	defer dir.Close()

	cmd := exec.Command("/proc/self/exe", append([]string{"child", cfg.Hostname, rootfs}, cfg.Argv...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Пять новых namespaces: имя хоста, номера процессов, точки монтирования,
		// сеть и System V IPC. User namespace не создаём — лаба работает от root.
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
		// clone3(CLONE_INTO_CGROUP): ребёнок под лимитами с первой инструкции.
		UseCgroupFD: true,
		CgroupFD:    int(dir.Fd()),
		// Родитель умер — ребёнок не должен пережить его и держать cgroup.
		Pdeathsig: syscall.SIGKILL,
	}
	runErr := cmd.Run()
	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		return 1, fmt.Errorf("запуск: %w", runErr)
	}
	report(cg)
	return exitCode(cmd.ProcessState), nil
}

// #endregion clone

// exitCode — как в shell: убитый сигналом N процесс даёт 128+N (OOM-kill → 137).
func exitCode(ps *os.ProcessState) int {
	ws, ok := ps.Sys().(syscall.WaitStatus)
	if ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ps.ExitCode()
}

// report печатает, сработали ли лимиты: это читает scenario.sh.
func report(cg *cgroup) {
	mem, merr := cg.events("memory.events")
	pids, perr := cg.events("pids.events")
	if err := errors.Join(merr, perr); err != nil {
		fmt.Fprintln(os.Stderr, "minicontainer: события cgroup:", err)
		return
	}
	fmt.Fprintf(os.Stderr, "minicontainer: итог: oom_kill=%d pids_max=%d\n", mem["oom_kill"], pids["max"])
}

// #region child
// child выполняется уже в новых namespaces: он PID 1 своего PID namespace.
func child(args []string) error {
	if len(args) < 3 {
		return errors.New("нужно: child <hostname> <rootfs> <команда> [аргументы…]")
	}
	host, rootfs, argv := args[0], args[1], args[2:]
	if err := syscall.Sethostname([]byte(host)); err != nil {
		return fmt.Errorf("sethostname: %w", err)
	}
	// Точки монтирования окружения обычно «shared»: без MS_PRIVATE наши mount и umount
	// распространились бы обратно в родительский mount namespace.
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("mount --make-rprivate /: %w", err)
	}
	// pivot_root требует, чтобы новый корень был точкой монтирования, — bind на себя.
	if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind %s: %w", rootfs, err)
	}
	oldroot := filepath.Join(rootfs, ".oldroot")
	if err := os.MkdirAll(oldroot, 0o700); err != nil {
		return fmt.Errorf("mkdir .oldroot: %w", err)
	}
	if err := syscall.PivotRoot(rootfs, oldroot); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir /: %w", err)
	}
	// Старый корень — вся файловая система окружения. Оставить его — оставить выход
	// из контейнера; в этом главное отличие pivot_root от chroot.
	if err := syscall.Unmount("/.oldroot", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("umount /.oldroot: %w", err)
	}
	if err := os.Remove("/.oldroot"); err != nil {
		return fmt.Errorf("rmdir /.oldroot: %w", err)
	}
	if err := mountProc(); err != nil {
		return err
	}
	if err := mountDev(); err != nil {
		return err
	}
	if err := os.Setenv("PATH", "/bin:/sbin:/usr/bin:/usr/sbin"); err != nil {
		return fmt.Errorf("PATH: %w", err)
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("команда %s: %w", argv[0], err)
	}
	// Exec заменяет child командой пользователя: она и становится PID 1.
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=/root", "TERM=" + os.Getenv("TERM")}
	return fmt.Errorf("exec %s: %w", path, syscall.Exec(path, argv, env))
}

// #endregion child

// mountProc монтирует свежий procfs: он показывает процессы *своего* PID namespace.
func mountProc() error {
	if err := os.MkdirAll("/proc", 0o555); err != nil {
		return fmt.Errorf("mkdir /proc: %w", err)
	}
	if err := syscall.Mount("proc", "/proc", "proc", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, ""); err != nil {
		return fmt.Errorf("mount /proc: %w", err)
	}
	return nil
}

// devices — минимум, без которого не работают shell и большинство программ
// (фоновые задания shell читают /dev/null). runc создаёт тот же набор и ещё несколько.
var devices = []struct {
	name         string
	major, minor uint32
}{
	{"null", 1, 3},
	{"zero", 1, 5},
	{"urandom", 1, 9},
}

func mountDev() error {
	if err := os.MkdirAll("/dev", 0o755); err != nil {
		return fmt.Errorf("mkdir /dev: %w", err)
	}
	if err := syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_NOEXEC, "mode=755,size=64k"); err != nil {
		return fmt.Errorf("mount /dev: %w", err)
	}
	for _, d := range devices {
		dev := int(d.major<<8 | d.minor) // makedev для старших/младших номеров < 256
		if err := syscall.Mknod("/dev/"+d.name, syscall.S_IFCHR|0o666, dev); err != nil {
			return fmt.Errorf("mknod /dev/%s: %w", d.name, err)
		}
	}
	return nil
}
```

Если `go vet` или `golangci-lint` (лабы им не проверяются, но `vet` — да) ругается на `dev` как `int` — использовать `unix.Mkdev` нельзя (без внешних зависимостей), оставить формулу с комментарием.

- [ ] **Step 9: `cmd/hog/main.go`**

```go
// Command hog занимает память кусками по 1 МиБ и трогает каждую страницу. Только
// тронутая страница становится настоящей памятью процесса: make без записи — обещание,
// и OOM-killer его не увидит (урок 1.15).
package main

import (
	"flag"
	"fmt"
	"runtime"
)

const (
	chunk    = 1 << 20
	pageSize = 4096
	// reportEvery — печатать прогресс каждые 16 МиБ, чтобы в выводе было видно, где убили.
	reportEvery = 16
)

func main() {
	mib := flag.Int("mib", 256, "сколько МиБ занять")
	flag.Parse()
	chunks := make([][]byte, 0, *mib)
	for i := 1; i <= *mib; i++ {
		b := make([]byte, chunk)
		for j := 0; j < len(b); j += pageSize {
			b[j] = 1
		}
		chunks = append(chunks, b)
		if i%reportEvery == 0 {
			fmt.Printf("занято %d МиБ\n", i)
		}
	}
	fmt.Printf("занято %d МиБ — лимит не сработал\n", *mib)
	runtime.KeepAlive(chunks)
}
```

- [ ] **Step 10: Окружение лабы**

`labs/1c/Dockerfile`:

```dockerfile
# syntax=docker/dockerfile:1.19@sha256:b6afd42430b15f2d2a4c5a02b919e98a525b785b1aaff16747d2f623364e39b6

# region rootfs
# Rootfs контейнера — файловая система образа busybox, закреплённого по digest.
# Это ровно то, что docker run распаковал бы для busybox: каталоги и статический бинарь.
FROM busybox:1.37@sha256:bdf57e528e45e4433820e045b29b4597825a1c9e38353532d90a01445013f82e AS rootfs
# endregion rootfs

FROM golang:1.26.9-trixie@sha256:f06b7122f888680048afb247c4e5f49c0069eb6c4c1ccae0f3f4834527a8d22c AS build
WORKDIR /src
COPY go.mod ./
COPY 1c ./1c
# hog — статический: в rootfs нет libc.
RUN CGO_ENABLED=0 go build -o /out/minicontainer ./1c/cmd/minicontainer \
 && CGO_ENABLED=0 go build -o /out/hog ./1c/cmd/hog

FROM golang:1.26.9-trixie@sha256:f06b7122f888680048afb247c4e5f49c0069eb6c4c1ccae0f3f4834527a8d22c
RUN apt-get update \
 && apt-get install -y --no-install-recommends iproute2 procps util-linux \
 && rm -rf /var/lib/apt/lists/*
COPY --from=rootfs / /rootfs
COPY --from=build /out/hog /rootfs/bin/hog
COPY --from=build /out/minicontainer /usr/local/bin/minicontainer
COPY 1c/entrypoint.sh 1c/scenario.sh /lab/
WORKDIR /lab
ENTRYPOINT ["/lab/entrypoint.sh"]
CMD ["bash"]
```

`labs/1c/compose.yaml`:

```yaml
name: lab1c

services:
  lab:
    build:
      context: ..
      dockerfile: 1c/Dockerfile
    image: lab1c:local
    # region privileged
    # privileged: лаба сама делает то, что обычно делает Docker, — mount, pivot_root,
    # запись в cgroup. Это почти права хоста, поэтому только в лабе и никогда в проде.
    privileged: true
    # Свой cgroup namespace: лаба видит свою группу как корень /sys/fs/cgroup и
    # создаёт дочерние группы, не трогая чужие.
    cgroup: private
    # endregion privileged
    hostname: lab
    stdin_open: true
    tty: true
```

`labs/1c/entrypoint.sh`:

```sh
#!/bin/sh
# region entrypoint
# В своём cgroup namespace контейнер лабы — корень иерархии, и в этом корне живут его
# процессы. Правило cgroup v2 «нет процессов во внутренних узлах»: включить контроллеры
# для дочерних групп нельзя, пока в группе есть процессы. Поэтому переносим все процессы
# в лист init и только потом включаем cpu, memory и pids — так же поступает entrypoint
# образа docker:dind.
# endregion entrypoint
set -eu

cg=/sys/fs/cgroup
if [ -w "$cg/cgroup.subtree_control" ] && ! grep -qw memory "$cg/cgroup.subtree_control"; then
	mkdir -p "$cg/init"
	# Процесс мог завершиться между чтением списка и записью — это не ошибка.
	while read -r pid; do
		echo "$pid" >"$cg/init/cgroup.procs" 2>/dev/null || true
	done <"$cg/cgroup.procs"
	echo "+cpu +memory +pids" >"$cg/cgroup.subtree_control"
fi
exec "$@"
```

(Маркеры `# region` — в комментарии, до `set -eu`: VitePress вырезает строки маркеров, текст между ними попадает в урок.)

`labs/1c/scenario.sh`:

```bash
#!/bin/bash
# Сценарный прогон пяти экспериментов урока 1.15 внутри контейнера лабы.
# Падает на первом расхождении; check:labs запускает его на каждой проверке.
set -euo pipefail

fail() {
	echo "ПРОВАЛ: $*" >&2
	exit 1
}

# 1. Свой PID namespace: процесс видит себя единственным и с PID 1.
out=$(minicontainer run /rootfs ps 2>/dev/null)
lines=$(echo "$out" | awk 'NR > 1' | wc -l)
pid=$(echo "$out" | awk 'NR == 2 { print $1 }')
[ "$lines" -eq 1 ] && [ "$pid" = 1 ] || fail "ps в контейнере: ждали один процесс с PID 1, получили:
$out"
echo "ок 1: в контейнере один процесс, PID 1"

# 2. Свой UTS namespace: hostname внутри не меняет hostname окружения.
inside=$(minicontainer run --hostname box /rootfs hostname 2>/dev/null)
[ "$inside" = box ] || fail "hostname внутри: $inside"
[ "$(hostname)" = lab ] || fail "hostname окружения изменился: $(hostname)"
echo "ок 2: hostname внутри — box, снаружи — lab"

# 3. Свой net namespace: только loopback, и тот выключен.
links=$(minicontainer run /rootfs ip -o link 2>/dev/null)
[ "$(echo "$links" | wc -l)" -eq 1 ] && echo "$links" | grep -q 'lo:' && echo "$links" | grep -q 'state DOWN' ||
	fail "сеть в контейнере: $links"
echo "ок 3: в сети только lo в состоянии DOWN"

# 4. pids.max: 64 фоновых процесса при лимите 16 — часть не родится.
err=$(minicontainer run --pids 16 /rootfs sh -c 'for i in $(seq 1 64); do sleep 1 & done; wait' 2>&1 >/dev/null || true)
echo "$err" | grep -Eq 'pids_max=[1-9]' || fail "pids.max не сработал: $err"
echo "ок 4: лимит pids.max отказал в fork"

# 5. memory.max: hog хочет 128 МиБ при лимите 32 — OOM-kill, код 137.
set +e
err=$(minicontainer run --mem 32M /rootfs hog -mib 128 2>&1 >/dev/null)
code=$?
set -e
[ "$code" -eq 137 ] || fail "код выхода $code, ждали 137: $err"
echo "$err" | grep -Eq 'oom_kill=[1-9]' || fail "oom_kill не отмечен: $err"
echo "ок 5: OOM-kill по memory.max, код выхода 137"

# Ни одна группа не осталась после запусков — в том числе после OOM-kill.
if ls -d /sys/fs/cgroup/minicontainer-* >/dev/null 2>&1; then
	fail "остались группы: $(ls -d /sys/fs/cgroup/minicontainer-*)"
fi
echo "Все пять экспериментов прошли."
```

Сделать оба скрипта исполняемыми: `chmod +x labs/1c/entrypoint.sh labs/1c/scenario.sh` (git хранит бит).

- [ ] **Step 11: Прогнать лабу вручную**

```bash
cd labs/1c
docker compose build
docker compose run --rm -T lab /lab/scenario.sh
```

Expected: пять строк «ок N: …» и «Все пять экспериментов прошли.» Если эксперимент 4 или 5 падает, разобраться по выводу; частые причины и что делать:
- `pids_max=0` — busybox `sh` создаёт процессы медленнее, чем выходят предыдущие `sleep 1`: увеличить `sleep` до 2 (а не ослаблять проверку);
- код 1 вместо 137 и «can't fork» — `--mem` слишком мал для старта `hog`: держать 32M, но проверить `minMemory`;
- `clone3: operation not permitted` — compose запущен без `privileged` или без `cgroup: private`.

Записать реальный вывод в `labs/1c/README.md` (шаг 13) — его же цитирует урок.

- [ ] **Step 12: `check:labs`**

В `course/scripts/check-labs.ts` после проверки 0b:

```ts
run('docker', ['compose', '-f', '1c/compose.yaml', 'config', '-q'])
// Сценарий 1c — настоящие namespaces и cgroup в privileged-контейнере: сборка образа
// при первом запуске и пять экспериментов.
const scenario = run('docker', ['compose', '-f', '1c/compose.yaml', 'run', '--rm', '--build', '-T', 'lab', '/lab/scenario.sh'], 900_000)
if (!scenario.includes('Все пять экспериментов прошли.')) {
  console.error(`Сценарий лабы 1c:\n${scenario}`)
  process.exit(1)
}
```

и обновить итоговую строку: `'Лабораторные: gofmt, go vet, go test -race, compose config и сценарий 1c — без ошибок.'`

- [ ] **Step 13: README лабы**

`labs/1c/README.md` — по образцу `labs/0b/README.md`: что это, требования (Docker с cgroup v2; на Docker Desktop всё работает внутри его VM), как собрать и войти (`docker compose run --rm lab`), пять экспериментов с командами и реальным выводом, как запустить сценарий целиком, почему `privileged` и `cgroup: private`, ссылка на урок 1.15. Внизу — «Как это сломать» (2–3 упражнения: убрать `MS_PRIVATE`, заменить `pivot_root` на `chroot` и выбраться через `..`, убрать `memory.swap.max`).

- [ ] **Step 14: Прогнать проверки**

Run (из `course`): `npm run check:labs`
Expected: итог без ошибок, сценарий 1c прошёл.

- [ ] **Step 15: Commit**

```bash
git add labs/1c course/scripts/check-labs.ts
git commit -m "feat(labs): лаба 1c — контейнер с нуля на Go: namespaces, pivot_root, cgroup v2

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Урок 1.15 «Контейнер — это процесс»

**Files:**
- Create: `course/modules/01-go-service/15-container-internals.md`

**Interfaces:**
- Consumes: сцены `ContainerStart`, `CgroupLimits` (задача 3); регионы `clone`, `child`, `cgroup-limits` (`labs/1c/cmd/minicontainer/*.go`), `entrypoint` (`labs/1c/entrypoint.sh`), `privileged`, `rootfs` (`labs/1c/compose.yaml`, `labs/1c/Dockerfile`); термины задачи 1.

- [ ] **Step 1: Собрать реальный вывод**

- Прогнать пять экспериментов вручную (`docker compose run --rm lab`, затем команды из README) и сохранить вывод.
- Сверка с настоящим Docker: запустить контейнер, который точно есть локально (`docker run -d --name probe lab1c:local sleep 600`; образ мессенджера появится только в задаче 6 — в уроке так и написать: «любой контейнер; в уроке 1.17 повторим на мессенджере»), затем:
  - `docker inspect -f '{{.State.Pid}}' probe`;
  - на Docker Desktop все следующие команды выполняются в VM: `docker run --rm -it --privileged --pid=host debian:trixie-slim nsenter -t 1 -m -u -n -i sh` (если нет `debian:trixie-slim` — `lab1c:local` с `--entrypoint sh`); на «голом» Linux — прямо на хосте под `sudo`;
  - `lsns -p <pid>`, `ls -l /proc/<pid>/ns/` и то же для PID 1 — номера namespaces различаются;
  - `cat /proc/<pid>/cgroup` и `cat /sys/fs/cgroup/<путь>/memory.max` (запустить второй контейнер с `--memory 64m` и показать `67108864`);
  - `docker inspect -f '{{json .GraphDriver}}' probe` и `mount | grep overlay` (если Docker Desktop использует containerd image store, драйвер будет `overlayfs` snapshotter — показать то, что есть, и объяснить).
- Весь вывод вставляется в урок как есть (длинные строки можно обрезать `…` с пометкой).

- [ ] **Step 2: Написать урок (контракт)**

Front matter `title: "1.15. Контейнер — это процесс"`; строка «Модуль 1 · Урок 15» — как в остальных уроках. Разделы:
- **Проблема** — «у меня работает, а в проде нет»: разные версии библиотек, порты, конфиги; виртуалка решает, но стоит гигабайты и десятки секунд старта; что на самом деле обещает Docker и откуда миллисекундный старт.
- **Теория:**
  - процесс и что он видит (PID, файловая система, сеть, hostname) — и что всё это можно подменить по отдельности;
  - namespaces: таблица из 8 типов (mount, UTS, IPC, PID, net, user, cgroup, time) — что прячет каждый, год появления в ядре, создаёт ли его Docker по умолчанию (user и time — нет); `clone(2)`, `unshare(2)`, `setns(2)`;
  - PID namespace: два номера у одного процесса, PID 1 и его особенности (сигналы по умолчанию, сироты) — коротко, подробно в 1.17;
  - mount namespace и rootfs: `chroot` против `pivot_root`, почему из `chroot` можно выбраться;
  - cgroups v2: единая иерархия, контроллеры, файлы `memory.max`, `memory.events`, `cpu.max`, `cpu.stat`, `pids.max`; правило «нет процессов во внутренних узлах»; почему лимит пишется до запуска процесса (`CLONE_INTO_CGROUP`);
  - OverlayFS: lowerdir/upperdir/workdir, copy-up, whiteout; связь со слоями образа (подробно — 1.16);
  - OCI image и runtime spec, цепочка `docker` CLI → `dockerd` → `containerd` → `containerd-shim` → `runc`; runc — та же программа, что наша лаба, только полная;
  - чего контейнер не изолирует: общее ядро (уязвимость ядра — побег), часы, часть `/proc` и `/sys`; почему `--privileged` почти равен хосту; gVisor и Kata как сильная изоляция — абзацем;
  - `<Deep>` — «Почему контейнеры не появились раньше»: chroot (1979), FreeBSD jails (2000), Solaris Zones, namespaces в Linux с 2002 по 2013, cgroups от Google (2007), LXC, Docker 2013 — что именно добавил Docker (формат образа и удобство, а не изоляцию).
- **Визуализация** — `<ContainerStart />` и `<CgroupLimits />`, к каждой — абзац «что смотреть».
- **Код: контейнер с нуля** — регионы `clone`, `child`, `cgroup-limits`, `entrypoint`, `privileged`, `rootfs`; разбор каждого шага; пять экспериментов с реальным выводом; «как это сломать» — ссылка на README лабы.
- **Сверка с настоящим Docker** (`###` внутри «Кода» или отдельный `##` с `<Part kind="practice" />`) — вывод из шага 1 с объяснением каждой строки; отдельно — как попасть в VM Docker Desktop.
- **Что изменилось в репозитории** — `labs/1c` (мессенджер не менялся); тег `m01-l15`.
- **Типичные ошибки** (5–7): «контейнер — лёгкая виртуалка»; root в контейнере = безопасно; `--privileged` «чтобы заработало»; лимит памяти без учёта page cache; `chroot` как изоляция; ожидание, что контейнер видит лимиты в `free`/`/proc/meminfo` (не видит — без lxcfs).
- **Компромисс** — изоляция против накладных расходов: процесс → контейнер → gVisor → microVM (Firecracker) → VM; где что выбирают (мультиарендность облаков — microVM).
- **Самопроверка** — 6–8 вопросов.
- **Источники** — man-страницы `namespaces(7)`, `cgroups(7)`, `pivot_root(2)`, `clone(2)`; `Documentation/admin-guide/cgroup-v2.rst`; OCI runtime spec; Liz Rice, «Containers From Scratch» (доклад) и «Container Security» (O'Reilly, 2020); документация Docker о `--privileged`.

- [ ] **Step 3: Проверить**

Run: `npm run build && npm run check:refs && npm test`
Expected: PASS; все `<T>` известны; регионы найдены.

- [ ] **Step 4: Commit и тег**

```bash
git add course/modules/01-go-service/15-container-internals.md
git commit -m "docs(course): урок 1.15 — контейнер это процесс: namespaces, cgroups, overlayfs

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l15
```

---

### Task 6: Образ мессенджера, урок 1.16 и ADR-0007

**Files:**
- Create: `messenger/Dockerfile`, `messenger/.dockerignore`, `messenger/cmd/messenger/cli.go`, `messenger/cmd/messenger/cli_test.go`
- Modify: `messenger/cmd/messenger/main.go`
- Create: `course/modules/01-go-service/examples/dockerfiles/{naive,alpine,scratch}.Dockerfile`
- Create: `course/scripts/check-images.ts`; Modify: `course/package.json`
- Create: `messenger/docs/adr/0007-distroless-вместо-scratch-и-alpine.md`
- Create: `course/modules/01-go-service/16-image.md`

**Interfaces:**
- Produces: `var version = "dev"` в `package main` мессенджера (ставится `-ldflags "-X main.version=…"`); `subcommand(args []string, stdout, stderr io.Writer) (code int, handled bool)` — задача 7 добавляет в неё `healthcheck`; образ `messenger:local`; `npm run check:images`; регионы Dockerfile `build`, `runtime`.

- [ ] **Step 1: Тест служебных команд**

`messenger/cmd/messenger/cli_test.go`:

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSubcommand(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		handled bool
		code    int
		stdout  string
		stderr  string
	}{
		{"без аргументов — запуск сервера", nil, false, 0, "", ""},
		{"version", []string{"version"}, true, 0, "dev\n", ""},
		{"--version", []string{"--version"}, true, 0, "dev\n", ""},
		{"неизвестная команда", []string{"serve"}, true, 2, "", "неизвестная команда"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code, handled := subcommand(c.args, &out, &errOut)
			if handled != c.handled || code != c.code || out.String() != c.stdout || !strings.Contains(errOut.String(), c.stderr) {
				t.Fatalf("code=%d handled=%v stdout=%q stderr=%q", code, handled, out.String(), errOut.String())
			}
		})
	}
}
```

- [ ] **Step 2: Run** `go test ./cmd/messenger -run TestSubcommand` — Expected: FAIL.

- [ ] **Step 3: `cli.go`**

```go
package main

import (
	"fmt"
	"io"
)

// version подставляет сборка: go build -ldflags "-X main.version=1.2.3". По нему в
// журнале видно, какая сборка ответила, — без этого «в проде старая версия?» не проверить.
var version = "dev"

// subcommand выполняет служебную команду бинаря. handled == false — аргументов нет,
// и main запускает сервер. В образе distroless нет shell и утилит, поэтому всё, что
// нужно рядом с сервером (версия, healthcheck), умеет сам бинарь.
func subcommand(args []string, stdout, stderr io.Writer) (code int, handled bool) {
	if len(args) == 0 {
		return 0, false
	}
	switch args[0] {
	case "version", "--version":
		_, _ = fmt.Fprintln(stdout, version)
		return 0, true
	default:
		_, _ = fmt.Fprintf(stderr, "неизвестная команда %q; есть: version\n", args[0])
		return 2, true
	}
}
```

В `main.go`: первой строкой `main` —

```go
	if code, handled := subcommand(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
```

и в `run` строку `log.Info("слушаю", "addr", cfg.Addr)` заменить на `log.Info("слушаю", "addr", cfg.Addr, "version", version)`.

- [ ] **Step 4: Run** `go test ./cmd/messenger` — Expected: PASS.

- [ ] **Step 5: `messenger/.dockerignore`**

```
# Allowlist: в контекст сборки попадает только то, что нужно go build. Всё остальное
# (документация, ключи *.pem, артефакты IDE) не попадает в образ и не сбрасывает кэш
# слоя COPY . . при правке README.
*
!go.mod
!go.sum
!cmd
!internal
# Тесты не нужны для сборки: их правка не должна пересобирать образ.
**/*_test.go
**/testdata
```

Проверить: если `go build ./cmd/messenger` внутри образа падает из-за отсутствующего файла (например, встроенного через `//go:embed`), добавить его в allowlist и написать в комментарии почему.

- [ ] **Step 6: `messenger/Dockerfile`**

```dockerfile
# syntax=docker/dockerfile:1.19@sha256:b6afd42430b15f2d2a4c5a02b919e98a525b785b1aaff16747d2f623364e39b6

# region build
# Стадия сборки идёт на платформе сборщика ($BUILDPLATFORM): Go сам кросс-компилирует
# под TARGETOS/TARGETARCH, и для arm64-образа не нужна эмуляция QEMU (урок 1.19).
FROM --platform=$BUILDPLATFORM golang:1.26.9-trixie@sha256:f06b7122f888680048afb247c4e5f49c0069eb6c4c1ccae0f3f4834527a8d22c AS build
WORKDIR /src
# Сначала только манифесты модулей: слой зависимостей пересобирается, когда меняются
# они, а не при каждой правке кода.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
ARG TARGETOS TARGETARCH
ARG VERSION=dev
# CGO_ENABLED=0 — статический бинарь без libc: он запустится в образе, где libc нет.
# -trimpath убирает пути машины сборщика (воспроизводимость), -s -w — таблицы отладки.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/messenger ./cmd/messenger
# endregion build

# region runtime
# distroless static: CA-сертификаты, tzdata, /etc/passwd с пользователем nonroot (65532)
# — и больше ничего: ни shell, ни пакетного менеджера (ADR-0007).
FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
LABEL org.opencontainers.image.title="messenger" \
      org.opencontainers.image.description="Мессенджер курса «System Design на практике»: монолит v0"
COPY --from=build /out/messenger /messenger
# Внутри контейнера loopback недоступен снаружи: слушаем все интерфейсы. Наружу порт
# откроет только явный -p / ports: (урок 1.17).
ENV MESSENGER_ADDR=:8080
EXPOSE 8080
USER 65532:65532
# Exec-форма: messenger — PID 1 и сам получает SIGTERM (урок 1.17).
ENTRYPOINT ["/messenger"]
# endregion runtime
```

- [ ] **Step 7: Собрать и проверить вручную**

```bash
cd messenger
docker build -t messenger:local --build-arg VERSION=1c-dev .
docker image inspect messenger:local --format '{{.Size}} {{.Config.User}}'
docker run --rm messenger:local version
docker run --rm messenger:local          # без ключа: сообщение «задайте MESSENGER_JWT_KEY_FILE…», код 1
```

Expected: размер < 25 000 000 байт, пользователь `65532:65532`, `version` печатает `1c-dev`, запуск без ключа — понятная ошибка в JSON-журнале и код 1. Повторная сборка без изменений — все шаги `CACHED`.

- [ ] **Step 8: Варианты для сравнения**

В `course/modules/01-go-service/examples/dockerfiles/` (контекст сборки у всех — `messenger/`, `.dockerignore` тот же):
- `naive.Dockerfile` — `FROM golang:1.26.9-trixie@sha256:…`, `WORKDIR /src`, `COPY . .`, `RUN go build -o /messenger ./cmd/messenger`, `CMD ["/messenger"]` (без multi-stage, root, без кэша);
- `alpine.Dockerfile` — стадия сборки как в основном Dockerfile, финальная `FROM alpine:3.23@sha256:85fe1e81d6758c208f3e1eed4338a1997e19d4be002d4dd32d3100c9a8c010a0`, `RUN adduser -D -u 65532 app`, `USER 65532`;
- `scratch.Dockerfile` — стадия сборки как в основном, финальная `FROM scratch`, `COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/`, `COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo`, `USER 65532:65532`; в комментарии — чего не хватает по сравнению с distroless (`/etc/passwd`, `/tmp`, `/home/nonroot`) и почему для Go это обычно терпимо.

Каждый файл начинается комментарием «для урока 1.16, не для продакшена» (кроме сравнения ничего не делает).

- [ ] **Step 9: `check:images`**

`course/scripts/check-images.ts`:

```ts
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

// Сборка образов — минуты и сеть, поэтому отдельно от check:messenger. В CI то же
// самое делает джоба image (сборка, trivy, smoke).
const MESSENGER = fileURLToPath(new URL('../../messenger', import.meta.url))
const EXAMPLES = fileURLToPath(new URL('../modules/01-go-service/examples/dockerfiles', import.meta.url))
// Порог из спеки (§11): distroless static + статический бинарь мессенджера.
const MAX_IMAGE_BYTES = 25_000_000
const TRIVY = 'aquasec/trivy:0.74.0'

function run(cmd: string, args: string[], timeout = 900_000): string {
  try {
    return execFileSync(cmd, args, { cwd: MESSENGER, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], timeout, maxBuffer: 64 * 1024 * 1024 })
  } catch (e) {
    const err = e as { stderr?: string; stdout?: string; message: string }
    console.error(`${cmd} ${args.join(' ')}:\n${err.stdout ?? ''}${err.stderr || err.message}`)
    process.exit(1)
  }
}

function build(tag: string, dockerfile?: string): void {
  run('docker', ['build', '-q', '-t', tag, ...(dockerfile === undefined ? [] : ['-f', dockerfile]), '--build-arg', 'VERSION=check', '.'])
}

const inspect = (tag: string, format: string): string => run('docker', ['image', 'inspect', tag, '--format', format]).trim()

// Число уязвимостей HIGH и CRITICAL с исправлением; trivy читает образ через сокет Docker.
function vulns(tag: string): number {
  const json = run('docker', ['run', '--rm', '-v', '/var/run/docker.sock:/var/run/docker.sock', TRIVY, 'image', '--quiet', '--format', 'json', '--severity', 'HIGH,CRITICAL', '--ignore-unfixed', tag])
  const report = JSON.parse(json) as { Results?: { Vulnerabilities?: unknown[] }[] }
  return (report.Results ?? []).reduce((n, r) => n + (r.Vulnerabilities?.length ?? 0), 0)
}

build('messenger:local')
const size = Number(inspect('messenger:local', '{{.Size}}'))
const user = inspect('messenger:local', '{{.Config.User}}')
if (size >= MAX_IMAGE_BYTES || user !== '65532:65532') {
  console.error(`messenger:local: размер ${size} (порог ${MAX_IMAGE_BYTES}), пользователь «${user}» (ждали 65532:65532)`)
  process.exit(1)
}
if (run('docker', ['run', '--rm', 'messenger:local', 'version']).trim() !== 'check') {
  console.error('messenger:local version: ждали «check»')
  process.exit(1)
}
const mainVulns = vulns('messenger:local')
if (mainVulns > 0) {
  console.error(`messenger:local: уязвимостей HIGH/CRITICAL с исправлением — ${mainVulns}`)
  process.exit(1)
}

// Варианты для урока 1.16: собираются и сравниваются, но порогов не имеют.
const rows = [['distroless (наш)', 'messenger:local']]
for (const name of ['naive', 'alpine', 'scratch']) {
  const tag = `messenger-example:${name}`
  build(tag, join(EXAMPLES, `${name}.Dockerfile`))
  rows.push([name, tag])
}
for (const [name, tag] of rows) {
  const mb = (Number(inspect(tag, '{{.Size}}')) / 1e6).toFixed(1)
  console.log(`${name.padEnd(18)} ${mb.padStart(7)} МБ  HIGH/CRITICAL с исправлением: ${vulns(tag)}`)
}
console.log('Образы: сборка, размер, пользователь, version и trivy — без ошибок.')
```

`package.json`: `"check:images": "node scripts/check-images.ts"`. Задача 7 добавит в этот скрипт smoke-тест.

- [ ] **Step 10: Run** `npm run check:images` — Expected: таблица четырёх вариантов и итог без ошибок. Сохранить таблицу: она идёт в урок. Посчитать пакеты в каждом образе для урока: `docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.74.0 image --list-all-pkgs --format json --quiet <tag>` (поле `Packages`) — либо `syft`, если удобнее; в уроке указать, чем считали.

- [ ] **Step 11: ADR-0007**

`messenger/docs/adr/0007-distroless-вместо-scratch-и-alpine.md` — по шаблону `0000-template.md` и в стиле ADR-0006 (Статус «Принято, 2026-10-15», Контекст, силы, Рассмотренные варианты — наивный `golang`, alpine, scratch, distroless static, distroless base/cc; Решение; Последствия с плюсами, минусами и «что пересмотрит решение» — например, появление cgo-зависимости). Цифры размеров и CVE — из шага 10.

- [ ] **Step 12: Урок 1.16 (контракт)**

`course/modules/01-go-service/16-image.md`, `title: "1.16. Образ: слои, кэш и distroless"`. Разделы:
- **Проблема** — наивный Dockerfile: образ под гигабайт, каждая правка — минуты сборки с нуля, внутри компилятор, shell и сотни пакетов с уязвимостями; реальные цифры варианта `naive` из шага 10.
- **Теория:**
  - образ = манифест + конфиг + слои (tar-архивы), всё адресуется digest'ом; тег — изменяемая ссылка, digest — нет; мультиархитектурный индекс (подробно — 1.19);
  - слой и инструкция: какие инструкции дают слой (`RUN`, `COPY`, `ADD`), какие — только метаданные; удаление файла в следующем слое не уменьшает образ (whiteout);
  - кэш BuildKit: из чего ключ (инструкция, родитель, содержимое файлов для `COPY`), почему промах «ломает» всё ниже, правильный порядок; `.dockerignore` и контекст сборки;
  - cache mounts: кэш модулей и компилятора между сборками, почему он не попадает в образ;
  - multi-stage: стадии, `COPY --from`, что попадает в итоговый образ;
  - статическая сборка Go: `CGO_ENABLED=0` (что сломается с cgo: DNS-резолвер, `os/user`; почему для нас нет), `-trimpath`, `-ldflags "-s -w -X"`, воспроизводимость;
  - базовые образы: таблица четырёх вариантов (размер, пакеты, CVE, shell, пользователь, CA, tzdata) — реальные числа; почему distroless static: что в нём есть и чего нет; alpine и musl; scratch и что придётся положить самому;
  - закрепление по digest и кто его обновляет (Dependabot — урок 1.19);
  - `<Deep>` — «Как BuildKit строит граф»: LLB, параллельная сборка независимых стадий, кэш-экспорт (`type=gha`, `type=registry`) — прозой.
- **Визуализация** — `<LayerCacheSandbox />`, абзац «что попробовать» (три опыта: порядок, `.dockerignore`, multi-stage).
- **Код** — регионы `build`, `runtime` Dockerfile; `.dockerignore` целиком (через `<<<` без региона); `cli.go#…` не нужен — показать `docker run --rm messenger:local version`; реальный вывод сборки с `CACHED`; вывод `docker history messenger:local`.
- **Что изменилось в мессенджере** — Dockerfile, `.dockerignore`, `version`, ADR-0007; тег `m01-l16`.
- **Типичные ошибки** — `COPY . .` первой строкой; `latest` в `FROM`; секреты в `ARG`/`ENV` (видны в `docker history`); `apt-get update` отдельным слоем; удаление файлов «потом»; запуск от root; shell-форма `ENTRYPOINT` (ссылка на 1.17).
- **Компромисс** — distroless против alpine: отладка без shell (`docker debug`, эфемерные контейнеры Kubernetes `kubectl debug`) против поверхности атаки.
- **Самопроверка** — 6–8 вопросов. **Источники** — Dockerfile reference и BuildKit docs (cache, mounts), OCI image spec, GoogleContainerTools/distroless README, Go `cmd/go` (`-trimpath`, `-ldflags`), Docker docs «Multi-stage builds», «Build cache».

- [ ] **Step 13: Проверить**

Run: `go test -race ./...` (messenger), `npm run check:messenger && npm run build && npm test`
Expected: PASS.

- [ ] **Step 14: Commit и тег**

```bash
git add messenger/Dockerfile messenger/.dockerignore messenger/cmd/messenger messenger/docs/adr/0007-*.md course/scripts/check-images.ts course/package.json course/modules/01-go-service/examples course/modules/01-go-service/16-image.md
git commit -m "feat(messenger): образ multi-stage на distroless, version; урок 1.16 и ADR-0007

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l16
```

---
### Task 7: Сервис в контейнере — пробы, таймаут запроса, дренаж, healthcheck, конфиг

**Files:**
- Create: `messenger/internal/platform/health/{doc.go,health.go,health_test.go}`
- Create: `messenger/internal/platform/middleware/{timeout.go,timeout_test.go}`
- Modify: `messenger/internal/platform/httpserver/{server.go,server_test.go}`
- Create: `messenger/cmd/messenger/{healthcheck.go,healthcheck_test.go}`
- Modify: `messenger/cmd/messenger/{main.go,app.go,app_test.go,cli.go,cli_test.go}`
- Modify: `messenger/Dockerfile` (HEALTHCHECK), `messenger/.golangci.yml`
- Create: `messenger/scripts/smoke.sh`; Modify: `course/scripts/check-images.ts`

**Interfaces:**
- Consumes: `middleware.Middleware`, `statusRecorder` (пакет `middleware`), `problem.Write`, `problem.CodeTimeout`, `problem.CodeMethodNotAllowed`; `subcommand` и `version` (задача 6).
- Produces:
  - `health.New(log *slog.Logger, checks ...health.Check) (*health.Probes, error)`, `health.Check{Name string; Checker health.Checker}`, `health.Checker` (`Check(ctx) error`), `(*Probes).StartDraining()`, `(*Probes).Register(mux *http.ServeMux, notAllowed http.HandlerFunc)`, `health.LivePath = "/livez"`, `health.ReadyPath = "/readyz"`;
  - `middleware.Timeout(d time.Duration, onTimeout func(http.ResponseWriter, *http.Request)) middleware.Middleware`;
  - `httpserver.New(addr string, h http.Handler, log *slog.Logger, requestTimeout time.Duration) *http.Server`, `httpserver.Run(ctx, srv, httpserver.Stop) error`, `httpserver.Stop{Drain func(); DrainDelay, Timeout time.Duration}`;
  - `newApp(cfg config, log *slog.Logger) (*app, error)`, `type app struct{ handler http.Handler; probes *health.Probes }`;
  - поля `config.RequestTimeout`, `ShutdownTimeout`, `DrainDelay`; переменные окружения `MESSENGER_REQUEST_TIMEOUT`, `MESSENGER_SHUTDOWN_TIMEOUT`, `MESSENGER_DRAIN_DELAY`;
  - подкоманда `messenger healthcheck`; `messenger/scripts/smoke.sh <образ>` (его вызывают `check:images` и CI);
  - регионы для урока 1.17: `probes` (health.go), `timeout` (timeout.go), `drain` (server.go), `healthcheck` (healthcheck.go), `root-mux` (app.go), `config` (app.go), `healthcheck` (Dockerfile).

- [ ] **Step 1: Тесты `health`**

`messenger/internal/platform/health/health_test.go`:

```go
package health_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"messenger/internal/platform/health"
)

type checkerFunc func(ctx context.Context) error

func (f checkerFunc) Check(ctx context.Context) error { return f(ctx) }

var ok = checkerFunc(func(context.Context) error { return nil })

func serve(t *testing.T, p *health.Probes, method, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	p.Register(mux, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusMethodNotAllowed) })
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, path, nil))
	var body map[string]any
	if method != http.MethodHead && rec.Code != http.StatusMethodNotAllowed {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("тело %q: %v", rec.Body, err)
		}
	}
	return rec, body
}

func newProbes(t *testing.T, logs *bytes.Buffer, checks ...health.Check) *health.Probes {
	t.Helper()
	p, err := health.New(slog.New(slog.NewJSONHandler(logs, nil)), checks...)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLiveIgnoresDependencies(t *testing.T) {
	down := checkerFunc(func(context.Context) error { return errors.New("postgres: connection refused") })
	p := newProbes(t, &bytes.Buffer{}, health.Check{Name: "postgres", Checker: down})
	p.StartDraining()

	rec, body := serve(t, p, http.MethodGet, health.LivePath)

	if rec.Code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("livez: %d %v — живость не зависит ни от зависимостей, ни от дренажа", rec.Code, body)
	}
}

func TestReady(t *testing.T) {
	down := checkerFunc(func(context.Context) error { return errors.New("dial tcp 10.0.0.5:5432: password=секрет") })
	cases := []struct {
		name   string
		checks []health.Check
		drain  bool
		status int
		failed map[string]any
	}{
		{"проверки прошли", []health.Check{{Name: "postgres", Checker: ok}}, false, http.StatusOK, nil},
		{"без проверок", nil, false, http.StatusOK, nil},
		{"зависимость недоступна", []health.Check{{Name: "postgres", Checker: ok}, {Name: "redis", Checker: down}}, false, http.StatusServiceUnavailable, map[string]any{"redis": "fail"}},
		{"дренаж", []health.Check{{Name: "postgres", Checker: ok}}, true, http.StatusServiceUnavailable, map[string]any{"shutdown": "draining"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logs := &bytes.Buffer{}
			p := newProbes(t, logs, c.checks...)
			if c.drain {
				p.StartDraining()
			}

			rec, body := serve(t, p, http.MethodGet, health.ReadyPath)

			if rec.Code != c.status {
				t.Fatalf("статус %d, ждали %d; тело %v", rec.Code, c.status, body)
			}
			if c.failed == nil && body["checks"] != nil {
				t.Fatalf("checks в успешном ответе: %v", body)
			}
			if c.failed != nil {
				got, _ := body["checks"].(map[string]any)
				if len(got) != len(c.failed) {
					t.Fatalf("checks %v, ждали %v", got, c.failed)
				}
				for k, v := range c.failed {
					if got[k] != v {
						t.Fatalf("checks %v, ждали %v", got, c.failed)
					}
				}
			}
			// Причина — в журнал, наружу — только имя проверки.
			if strings.Contains(rec.Body.String(), "секрет") || strings.Contains(rec.Body.String(), "10.0.0.5") {
				t.Fatalf("внутренности в ответе: %s", rec.Body)
			}
			if c.name == "зависимость недоступна" && !strings.Contains(logs.String(), "10.0.0.5") {
				t.Fatalf("причины нет в журнале: %s", logs)
			}
			if rec.Header().Get("Cache-Control") != "no-store" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("заголовки %v", rec.Header())
			}
		})
	}
}

func TestReadyBoundsSlowCheck(t *testing.T) {
	stuck := checkerFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	p := newProbes(t, &bytes.Buffer{}, health.Check{Name: "slow", Checker: stuck})

	start := time.Now()
	rec, _ := serve(t, p, http.MethodGet, health.ReadyPath)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("статус %d, ждали 503", rec.Code)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("проба ждала %v — срок проверок не соблюдён", d)
	}
}

func TestProbesMethods(t *testing.T) {
	p := newProbes(t, &bytes.Buffer{})
	for _, path := range []string{health.LivePath, health.ReadyPath} {
		if rec, _ := serve(t, p, http.MethodHead, path); rec.Code != http.StatusOK {
			t.Errorf("HEAD %s: %d", path, rec.Code)
		}
		rec, _ := serve(t, p, http.MethodPost, path)
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
			t.Errorf("POST %s: %d, Allow=%q", path, rec.Code, rec.Header().Get("Allow"))
		}
	}
}

func TestNewRejectsBadChecks(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	for _, c := range []health.Check{{Name: "", Checker: ok}, {Name: "x", Checker: nil}} {
		if _, err := health.New(log, c); err == nil {
			t.Errorf("New(%+v): ждали ошибку", c)
		}
	}
	if _, err := health.New(nil); err == nil {
		t.Error("New без журнала: ждали ошибку")
	}
	if _, err := health.New(log, health.Check{Name: "a", Checker: ok}, health.Check{Name: "a", Checker: ok}); err == nil {
		t.Error("повтор имени: ждали ошибку")
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/platform/health` — Expected: FAIL.

- [ ] **Step 3: `health`**

`doc.go`:

```go
// Package health — пробы платформы для оркестратора: жив ли процесс (/livez) и готов
// ли он принимать трафик (/readyz). Это не бизнес-API: пробы не описаны в контракте,
// не проходят аутентификацию и не пишутся в журнал доступа (ADR-0008).
package health
```

`health.go`:

```go
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
// (в модуле 2a — Postgres). Check обязан уважать ctx.
type Checker interface {
	Check(ctx context.Context) error
}

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
```

- [ ] **Step 4: Run** `go test -race ./internal/platform/health` — Expected: PASS.

- [ ] **Step 5: Тесты `Timeout`**

`messenger/internal/platform/middleware/timeout_test.go`:

```go
package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"messenger/internal/platform/middleware"
)

const testTimeout = 50 * time.Millisecond

// timeoutChain — Timeout с onTimeout, который отвечает 503 и считает вызовы.
func timeoutChain(h http.Handler) (http.Handler, *atomic.Int32) {
	var calls atomic.Int32
	onTimeout := func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	return middleware.Chain(h, middleware.Timeout(testTimeout, onTimeout)), &calls
}

func serveTimeout(t *testing.T, h http.Handler, ctx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/chats", nil))
	return rec
}

func TestTimeoutHandlerRespectsContext(t *testing.T) {
	// Обработчик, как и наши use case'ы, бросает работу по ctx.Done и ничего не пишет.
	h, calls := timeoutChain(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))

	start := time.Now()
	rec := serveTimeout(t, h, t.Context())

	if rec.Code != http.StatusServiceUnavailable || calls.Load() != 1 {
		t.Fatalf("статус %d, вызовов onTimeout %d", rec.Code, calls.Load())
	}
	if d := time.Since(start); d > 10*testTimeout {
		t.Fatalf("ответ через %v при сроке %v", d, testTimeout)
	}
}

func TestTimeoutDeadlineVisibleToHandler(t *testing.T) {
	var deadline time.Time
	var ok bool
	h, _ := timeoutChain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok = r.Context().Deadline()
		w.WriteHeader(http.StatusNoContent)
	}))

	serveTimeout(t, h, t.Context())

	if !ok || time.Until(deadline) > testTimeout {
		t.Fatalf("срок у обработчика: %v, %v", deadline, ok)
	}
}

func TestTimeoutFastHandlerUntouched(t *testing.T) {
	h, calls := timeoutChain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	rec := serveTimeout(t, h, t.Context())

	if rec.Code != http.StatusCreated || calls.Load() != 0 {
		t.Fatalf("статус %d, вызовов onTimeout %d", rec.Code, calls.Load())
	}
}

func TestTimeoutLateWriteKeepsHandlerResponse(t *testing.T) {
	// Обработчик не смотрит на ctx и отвечает после срока. Перехватить ответ уже
	// нельзя: Timeout не прерывает обработчик, а второй ответ поверх начатого — ошибка.
	h, calls := timeoutChain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * testTimeout)
		w.WriteHeader(http.StatusOK)
	}))

	rec := serveTimeout(t, h, t.Context())

	if rec.Code != http.StatusOK || calls.Load() != 0 {
		t.Fatalf("статус %d, вызовов onTimeout %d", rec.Code, calls.Load())
	}
}

func TestTimeoutHandlerWroteTimeoutItself(t *testing.T) {
	// Адаптер сам перевёл DeadlineExceeded в 503 problem+json — Timeout не дублирует.
	h, calls := timeoutChain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	rec := serveTimeout(t, h, t.Context())

	if rec.Code != http.StatusServiceUnavailable || calls.Load() != 0 {
		t.Fatalf("статус %d, вызовов onTimeout %d", rec.Code, calls.Load())
	}
}

func TestTimeoutClientGoneNoResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	h, calls := timeoutChain(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		cancel() // клиент ушёл раньше срока
		<-r.Context().Done()
	}))

	serveTimeout(t, h, ctx)

	if calls.Load() != 0 {
		t.Fatal("клиент ушёл, а Timeout пытается ему ответить")
	}
}

func TestTimeoutNoWriteWithoutDeadline(t *testing.T) {
	// Обработчик вернулся молча до срока: это не таймаут, отвечает net/http (200).
	h, calls := timeoutChain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	serveTimeout(t, h, t.Context())

	if calls.Load() != 0 {
		t.Fatal("onTimeout без истечения срока")
	}
}
```

- [ ] **Step 6: Run** `go test ./internal/platform/middleware -run Timeout` — Expected: FAIL.

- [ ] **Step 7: `timeout.go`**

```go
package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// #region timeout
// Timeout ставит каждому запросу срок d через context. Обработчики и use case'ы
// уважают ctx (урок 1.5): с истечением срока они бросают работу, а адаптер отвечает
// 503 timeout сам. onTimeout — страховка на случай, когда обработчик вернулся по
// сроку, ничего не записав.
//
// Timeout не прерывает обработчик, который ctx не смотрит, — в Go горутину снаружи не
// остановить. Такой обработчик ответит поздно, и его ответ дойдёт как есть; от совсем
// зависших спасает WriteTimeout сервера. http.TimeoutHandler прерывал бы ответ, но для
// этого буферизует его целиком и ломает потоковую передачу (модуль 3), поэтому не он.
func Timeout(d time.Duration, onTimeout func(w http.ResponseWriter, r *http.Request)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r.WithContext(ctx))
			// Отвечаем, только если ответа ещё нет, истёк именно наш срок и клиент на месте:
			// ушедшему клиенту (r.Context() отменён) отвечать некому.
			if rec.status == 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) && r.Context().Err() == nil {
				onTimeout(rec, r)
			}
		})
	}
}

// #endregion timeout
```

- [ ] **Step 8: Run** `go test -race ./internal/platform/middleware` — Expected: PASS.

- [ ] **Step 9: `httpserver` — тесты**

В `server_test.go`:
- все вызовы `httpserver.New(addr, h, log)` → `httpserver.New(addr, h, log, time.Second)`; все `httpserver.Run(ctx, srv, X)` → `httpserver.Run(ctx, srv, httpserver.Stop{Timeout: X})`; в `startInFlight` параметр `shutdownTimeout` передаётся так же;
- добавить:

```go
func TestNewWriteTimeoutCoversRequestTimeout(t *testing.T) {
	srv := httpserver.New("127.0.0.1:0", http.NotFoundHandler(), slog.New(slog.DiscardHandler), 7*time.Second)
	// Ответ о таймауте (503) должен успеть уйти до того, как сервер оборвёт запись.
	if srv.WriteTimeout <= 7*time.Second {
		t.Fatalf("WriteTimeout %v не больше таймаута запроса", srv.WriteTimeout)
	}
	if srv.ReadHeaderTimeout == 0 || srv.ReadTimeout == 0 || srv.IdleTimeout == 0 {
		t.Fatalf("таймауты не выставлены: %+v", srv)
	}
}

func TestRunDrainsBeforeShutdown(t *testing.T) {
	addr := freeAddr(t)
	var drainedAt, stoppedAt atomic.Int64
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	srv := httpserver.New(addr, h, slog.New(slog.DiscardHandler), time.Second)
	srv.RegisterOnShutdown(func() { stoppedAt.Store(time.Now().UnixNano()) })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var drains atomic.Int32
	runc := make(chan error, 1)
	go func() {
		runc <- httpserver.Run(ctx, srv, httpserver.Stop{
			Drain:      func() { drains.Add(1); drainedAt.Store(time.Now().UnixNano()) },
			DrainDelay: 200 * time.Millisecond,
			Timeout:    time.Second,
		})
	}()
	if err := getUntilServed(t.Context(), "http://"+addr+"/"); err != nil {
		t.Fatal(err)
	}

	cancel()

	if err := <-runc; err != nil {
		t.Fatalf("Run: %v", err)
	}
	if drains.Load() != 1 {
		t.Fatalf("Drain вызван %d раз", drains.Load())
	}
	if gap := time.Duration(stoppedAt.Load() - drainedAt.Load()); gap < 200*time.Millisecond {
		t.Fatalf("Shutdown через %v после Drain, ждали не меньше DrainDelay", gap)
	}
}

func TestRunServesDuringDrainDelay(t *testing.T) {
	addr := freeAddr(t)
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	srv := httpserver.New(addr, h, slog.New(slog.DiscardHandler), time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	drained := make(chan struct{})
	runc := make(chan error, 1)
	go func() {
		runc <- httpserver.Run(ctx, srv, httpserver.Stop{Drain: func() { close(drained) }, DrainDelay: 500 * time.Millisecond, Timeout: time.Second})
	}()
	if err := getUntilServed(t.Context(), "http://"+addr+"/"); err != nil {
		t.Fatal(err)
	}

	cancel()
	<-drained
	// Балансировщик ещё не знает о дренаже и шлёт запросы — их надо обслужить.
	if err := getUntilServed(t.Context(), "http://"+addr+"/"); err != nil {
		t.Fatalf("запрос во время дренажа: %v", err)
	}

	if err := <-runc; err != nil {
		t.Fatalf("Run: %v", err)
	}
}
```

(импортировать `sync/atomic`.)

- [ ] **Step 10: Run** `go test ./internal/platform/httpserver` — Expected: FAIL (сигнатуры).

- [ ] **Step 11: `server.go`**

- Константу `writeTimeout` заменить на `writeMargin = 5 * time.Second` с комментарием: «запас записи сверх таймаута запроса: ответ 503 timeout должен успеть уйти до того, как сервер оборвёт запись»; `New` получает `requestTimeout time.Duration` и ставит `WriteTimeout: requestTimeout + writeMargin`. Остальные таймауты не меняются. Комментарий у `readTimeout` дополнить: «таймаут запроса (MESSENGER_REQUEST_TIMEOUT) отсчитывается после чтения тела и сюда не входит».
- `Run`:

```go
// Stop — как останавливаться по сигналу.
type Stop struct {
	// Drain вызывается сразу по сигналу: экземпляр объявляет себя неготовым (/readyz → 503).
	Drain func()
	// DrainDelay — сколько ещё принимать новые запросы после Drain. Балансировщик узнаёт
	// о неготовности только со следующей пробой и до того продолжает слать трафик:
	// закрыть порт сразу — значит отказать этим запросам.
	DrainDelay time.Duration
	// Timeout — сколько Shutdown ждёт начатые запросы.
	Timeout time.Duration
}

// #region drain
// Run обслуживает запросы, пока не отменят ctx, затем останавливается в три шага:
// Drain → ожидание DrainDelay (запросы ещё принимаются) → Shutdown не дольше Timeout.
func Run(ctx context.Context, srv *http.Server, stop Stop) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	select {
	case err := <-serveErr:
		return fmt.Errorf("httpserver: %w", err)
	case <-ctx.Done():
	}

	if stop.Drain != nil {
		stop.Drain()
	}
	delay := time.NewTimer(stop.DrainDelay)
	select {
	case err := <-serveErr:
		delay.Stop()
		return fmt.Errorf("httpserver: во время дренажа: %w", err)
	case <-delay.C:
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stop.Timeout)
	defer cancel()
	// … дальше без изменений: Shutdown, Close по таймауту, ErrServerClosed …
}

// #endregion drain
```

Комментарии существующего `Run` про владельца горутины, буфер 1 и `WithoutCancel` сохранить; фразу «минимальная версия; подробно — модуль 1.5» заменить на «подробно — урок 1.17».

- [ ] **Step 12: Run** `go test -race ./internal/platform/httpserver` — Expected: PASS.

- [ ] **Step 13: Тесты healthcheck и конфига**

`messenger/cmd/messenger/healthcheck_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbeURL(t *testing.T) {
	cases := []struct {
		addr    string
		want    string
		wantErr bool
	}{
		{":8080", "http://127.0.0.1:8080/livez", false},
		{"0.0.0.0:8080", "http://127.0.0.1:8080/livez", false},
		{"[::]:8080", "http://[::1]:8080/livez", false},
		{"127.0.0.1:9090", "http://127.0.0.1:9090/livez", false},
		{"localhost:8080", "http://localhost:8080/livez", false},
		{"[::1]:8080", "http://[::1]:8080/livez", false},
		{"8080", "", true},
		{"", "", true},
		{"host:", "", true},
	}
	for _, c := range cases {
		got, err := probeURL(c.addr)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("probeURL(%q) = %q, %v; ждали %q, ошибка=%v", c.addr, got, err, c.want, c.wantErr)
		}
	}
}

func TestHealthcheck(t *testing.T) {
	status := func(code int) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }))
		t.Cleanup(srv.Close)
		return srv
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	t.Cleanup(slow.Close)
	gone := httptest.NewServer(http.NotFoundHandler())
	gone.Close()

	cases := []struct {
		name    string
		url     string
		timeout time.Duration
		ok      bool
	}{
		{"200 — живой", status(http.StatusOK).URL, time.Second, true},
		{"503 — нет", status(http.StatusServiceUnavailable).URL, time.Second, false},
		{"не слушает — нет", gone.URL, time.Second, false},
		{"не успел — нет", slow.URL, 50 * time.Millisecond, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := healthcheck(t.Context(), &http.Client{Timeout: c.timeout}, c.url)
			if (err == nil) != c.ok {
				t.Fatalf("healthcheck: %v", err)
			}
		})
	}
}
```

В `cli_test.go` добавить случай `{"healthcheck без сервера", []string{"healthcheck"}, true, 1, "", "проба"}` — с `t.Setenv("MESSENGER_ADDR", "127.0.0.1:1")` в подтесте (порт 1 не слушается). Случай «неизвестная команда» теперь перечисляет `version, healthcheck`.

В `app_test.go` → `TestLoadConfig` добавить табличный подтест для сроков (ключ подписи — тот же временный файл):

| env | ждём |
|---|---|
| ничего не задано | `RequestTimeout` 10 с, `ShutdownTimeout` 10 с, `DrainDelay` 5 с |
| `MESSENGER_REQUEST_TIMEOUT=2s` | 2 с |
| `MESSENGER_DRAIN_DELAY=0s` | 0 (дренаж без задержки разрешён) |
| `MESSENGER_REQUEST_TIMEOUT=0s` | ошибка, в тексте имя переменной |
| `MESSENGER_SHUTDOWN_TIMEOUT=-1s` | ошибка |
| `MESSENGER_DRAIN_DELAY=пять` | ошибка, в тексте имя переменной |
| `MESSENGER_DRAIN_DELAY=20s` + `MESSENGER_SHUTDOWN_TIMEOUT=10s` | ошибка: сумма 30 с не меньше grace period |

В `newClient` (app_test.go): `newApp(config{SigningKey: key, HasherParams: fastHasher, RequestTimeout: defaultRequestTimeout}, …)` и `a.handler` вместо `handler`; в `TestNewAppRejectsBadConfig` — случай «нулевой таймаут запроса → ошибка». Новый тест:

```go
// TestProbesOutsideAPI — пробы отвечают мимо аутентификации и контракта; дренаж
// переключает только готовность, API продолжает работать.
func TestProbesOutsideAPI(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	a, err := newApp(config{SigningKey: key, HasherParams: fastHasher, RequestTimeout: defaultRequestTimeout}, slog.New(slog.NewJSONHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	// Без валидатора контракта: проб в контракте нет — и не должно быть.
	srv := httptest.NewServer(a.handler)
	t.Cleanup(srv.Close)
	c := client{t: t, base: srv.URL, http: srv.Client()}

	c.expect(c.do(http.MethodGet, "/livez", "", ""), http.StatusOK, "livez без токена")
	c.expect(c.do(http.MethodGet, "/readyz", "", ""), http.StatusOK, "readyz без токена")
	r := c.do(http.MethodPost, "/readyz", "", "")
	c.expect(r, http.StatusMethodNotAllowed, "POST readyz")
	if r.header.Get("Allow") != "GET, HEAD" || !strings.HasPrefix(r.header.Get("Content-Type"), "application/problem+json") {
		t.Fatalf("405: заголовки %v", r.header)
	}

	a.probes.StartDraining()

	c.expect(c.do(http.MethodGet, "/readyz", "", ""), http.StatusServiceUnavailable, "readyz во время дренажа")
	c.expect(c.do(http.MethodGet, "/livez", "", ""), http.StatusOK, "livez во время дренажа")
	c.signUp("dora", "дренаж не мешает регистрироваться")
	if strings.Contains(logs.String(), `"path":"/readyz"`) {
		t.Fatal("пробы попали в журнал доступа")
	}
}
```

- [ ] **Step 14: Run** `go test ./cmd/messenger` — Expected: FAIL.

- [ ] **Step 15: `healthcheck.go`**

```go
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"messenger/internal/platform/health"
)

// healthcheckTimeout меньше --timeout=3s у HEALTHCHECK в Dockerfile: проверка должна
// закончиться сама и сказать почему, а не быть убитой Docker'ом молча.
const healthcheckTimeout = 2 * time.Second

// probeLimit — сколько байт тела дочитать: ответ пробы — пара десятков байт, а
// дочитанное тело позволяет переиспользовать соединение (здесь не нужно, но привычка).
const probeLimit = 1 << 10

// #region healthcheck
// probeURL — адрес /livez того же процесса. Сервер может слушать «все интерфейсы»
// (:8080, 0.0.0.0, [::]) — проба идёт на loopback того же семейства.
func probeURL(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("адрес %q: %w", addr, err)
	}
	if port == "" {
		return "", fmt.Errorf("адрес %q: нет порта", addr)
	}
	switch host {
	case "", "0.0.0.0":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	}
	return "http://" + net.JoinHostPort(host, port) + health.LivePath, nil
}

func healthcheck(ctx context.Context, c *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("проба: %w", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("проба: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, probeLimit))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("проба: статус %d", resp.StatusCode)
	}
	return nil
}

// #endregion healthcheck

// runHealthcheck — подкоманда `messenger healthcheck` для HEALTHCHECK в образе без shell.
func runHealthcheck(stderr io.Writer) int {
	url, err := probeURL(addrFromEnv())
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
		defer cancel()
		err = healthcheck(ctx, &http.Client{}, url)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
```

В `app.go` выделить `addrFromEnv() string` (`MESSENGER_ADDR` или `defaultAddr`) и использовать её в `loadConfig`. В `cli.go` — ветка `case "healthcheck": return runHealthcheck(stderr), true` и список команд в сообщении об ошибке.

- [ ] **Step 16: Конфиг и сборка в `app.go`**

```go
// #region config
const (
	defaultRequestTimeout  = 10 * time.Second
	defaultShutdownTimeout = 10 * time.Second
	// defaultDrainDelay — две пробы готовности с периодом по умолчанию (Kubernetes —
	// 10 с, наш compose — 1–2 с) обычно меньше; 5 с — компромисс для compose и kind.
	defaultDrainDelay = 5 * time.Second
	// terminationGrace — сколько Kubernetes (и наш compose) ждёт после SIGTERM до
	// SIGKILL. Дренаж и остановка должны уложиться в него с запасом.
	terminationGrace = 30 * time.Second
)

// durationEnv читает срок из окружения; пустая переменная — значение по умолчанию.
func durationEnv(name string, def time.Duration, allowZero bool) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	if d < 0 || (d == 0 && !allowZero) {
		return 0, fmt.Errorf("%s: срок должен быть больше нуля, получено %s", name, raw)
	}
	return d, nil
}

func loadTimeouts(cfg *config) error {
	var err error
	if cfg.RequestTimeout, err = durationEnv("MESSENGER_REQUEST_TIMEOUT", defaultRequestTimeout, false); err != nil {
		return err
	}
	if cfg.ShutdownTimeout, err = durationEnv("MESSENGER_SHUTDOWN_TIMEOUT", defaultShutdownTimeout, false); err != nil {
		return err
	}
	if cfg.DrainDelay, err = durationEnv("MESSENGER_DRAIN_DELAY", defaultDrainDelay, true); err != nil {
		return err
	}
	if total := cfg.DrainDelay + cfg.ShutdownTimeout; total >= terminationGrace {
		return fmt.Errorf("MESSENGER_DRAIN_DELAY + MESSENGER_SHUTDOWN_TIMEOUT = %s: должно быть меньше %s, иначе процесс убьют SIGKILL посреди остановки", total, terminationGrace)
	}
	return nil
}

// #endregion config
```

`loadConfig` вызывает `loadTimeouts(&cfg)`; комментарий у `defaultAddr` поправить: «в образе (урок 1.16) `MESSENGER_ADDR=:8080` задан в Dockerfile».

`newApp`:
- `cfg.RequestTimeout <= 0` → ошибка «сборка: нужен таймаут запроса»;
- цепочка — как раньше, плюс `middleware.Timeout(cfg.RequestTimeout, timeoutError(log))` **после** `Recover` и **до** `httpauth.Middleware`; комментарий над цепочкой дополнить: «Timeout — внутри Recover, чтобы паника по сроку тоже стала 500; до аутентификации — срок есть у всего, включая проверку токена»;
- корневой mux:

```go
	// #region root-mux
	// Пробы — мимо цепочки: без аутентификации, журнала доступа и таймаута запроса.
	// Оркестратор дёргает их каждые несколько секунд, и журнал на 90 % состоял бы из них.
	probes, err := health.New(log)
	if err != nil {
		return nil, fmt.Errorf("сборка: %w", err)
	}
	root := http.NewServeMux()
	probes.Register(root, methodNotAllowed(log))
	root.Handle("/", api)
	return &app{handler: root, probes: probes}, nil
	// #endregion root-mux
```

где `api` — результат прежнего `middleware.Chain(...)`; `methodNotAllowed(log)` пишет `problem.Problem{Code: problem.CodeMethodNotAllowed, Detail: "метод не поддерживается для этого пути"}` (заголовок `Allow` уже выставил `health`). `routeError` использует ту же функцию для 405, чтобы текст был в одном месте.
- `timeoutError(log)` пишет `problem.Problem{Code: problem.CodeTimeout, Detail: "повторите запрос позже"}` — тот же текст, что у `problem.Respond`; вынести строку в константу в `app.go` или переиспользовать, если в `problem` она уже есть константой.
- Тип:

```go
// app — то, что сборка отдаёт main: обработчик и пробы (main переводит их в дренаж по SIGTERM).
type app struct {
	handler http.Handler
	probes  *health.Probes
}
```

- [ ] **Step 17: `main.go` — дренаж**

`shutdownTimeout` удалить (теперь в конфиге). `run`:

```go
	a, err := newApp(cfg, log)
	…
	srv := httpserver.New(cfg.Addr, a.handler, log, cfg.RequestTimeout)
	log.Info("слушаю", "addr", cfg.Addr, "version", version)
	err = httpserver.Run(ctx, srv, httpserver.Stop{
		Drain: func() {
			a.probes.StartDraining()
			log.Info("останавливаюсь: /readyz отвечает 503", "drain_delay", cfg.DrainDelay)
		},
		DrainDelay: cfg.DrainDelay,
		Timeout:    cfg.ShutdownTimeout,
	})
	if err != nil {
		return err
	}
	log.Info("сервер остановлен")
```

Комментарий у пакета `main` дополнить: «служебные команды — в cli.go».

- [ ] **Step 18: Run** `go test -race ./...` — Expected: PASS (все пакеты, включая сквозной тест).

- [ ] **Step 19: Правило слоёв**

В `.golangci.yml` → `depguard.rules` добавить:

```yaml
        # Контексты не знают о пробах: проверки готовности (Postgres в 2a) регистрирует
        # composition root — так контекст не зависит от того, как его здоровье спрашивают.
        contexts-no-health:
          list-mode: lax
          files: ["**/internal/user/**", "**/internal/chat/**", "**/internal/auth/**"]
          deny:
            - pkg: messenger/internal/platform/health
              desc: "пробы собирает composition root (ADR-0008)"
```

Run: `PATH=$HOME/go/bin:$PATH golangci-lint run ./...` — Expected: 0 issues.

- [ ] **Step 20: HEALTHCHECK и smoke**

В `messenger/Dockerfile` перед `ENTRYPOINT`:

```dockerfile
# region healthcheck
# В distroless нет curl и shell — проверку делает сам бинарь. start-interval (Docker 25+)
# проверяет чаще, пока контейнер стартует: healthy через секунду, а не через interval.
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --start-interval=1s --retries=3 \
    CMD ["/messenger", "healthcheck"]
# endregion healthcheck
```

(регион `runtime` закрывается после `ENTRYPOINT`, регион `healthcheck` — внутри него; вложенные регионы VitePress поддерживает — проверить `npm run check:refs` и сборкой в задаче 8; если вложенность ломает вывод, вынести `# endregion runtime` перед `# region healthcheck` и добавить отдельный регион `entrypoint`.)

`messenger/scripts/smoke.sh`:

```bash
#!/bin/bash
# Smoke-тест образа: контейнер становится healthy, отвечает на пробы и API,
# корректно останавливается по SIGTERM. Используют check:images и CI (джоба image).
# Запуск из каталога messenger: scripts/smoke.sh <образ>
set -euo pipefail

image=${1:?нужно: scripts/smoke.sh <образ>}
port=${SMOKE_PORT:-18080}
name=messenger-smoke-$$
# Ключ — во временном каталоге внутри проекта: Docker Desktop не видит /tmp хоста.
dir=$(mktemp -d "$PWD/.smoke-XXXXXX")
cleanup() {
	docker rm -f "$name" >/dev/null 2>&1 || true
	rm -rf "$dir"
}
trap cleanup EXIT

go run ./cmd/keygen -out "$dir/jwt.pem"
# Одноразовый ключ smoke-теста: на «голом» Docker Engine bind mount сохраняет владельца
# хоста, и процесс с UID 65532 не прочитал бы файл 0600 (урок 1.17).
chmod 0444 "$dir/jwt.pem"

docker run -d --name "$name" -p "127.0.0.1:$port:8080" \
	-v "$dir/jwt.pem:/run/secrets/jwt_key:ro" \
	-e MESSENGER_JWT_KEY_FILE=/run/secrets/jwt_key \
	--read-only --cap-drop ALL --security-opt no-new-privileges:true \
	"$image" >/dev/null

status=starting
for _ in $(seq 1 30); do
	status=$(docker inspect -f '{{.State.Health.Status}}' "$name")
	[ "$status" = healthy ] && break
	sleep 1
done
if [ "$status" != healthy ]; then
	docker logs "$name" >&2
	echo "ПРОВАЛ: контейнер не стал healthy (статус $status)" >&2
	exit 1
fi

curl -fsS "http://127.0.0.1:$port/readyz" >/dev/null
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:$port/v1/users" \
	-H 'Content-Type: application/json' \
	-d '{"username":"smoke","display_name":"Smoke","password":"достаточно длинный пароль"}')
[ "$code" = 201 ] || { echo "ПРОВАЛ: POST /v1/users — $code" >&2; exit 1; }

# docker stop шлёт SIGTERM и ждёт 10 с: дренаж (5 с) + остановка должны уложиться.
docker stop "$name" >/dev/null
exit_code=$(docker inspect -f '{{.State.ExitCode}}' "$name")
[ "$exit_code" = 0 ] || { docker logs "$name" >&2; echo "ПРОВАЛ: код выхода $exit_code" >&2; exit 1; }
docker logs "$name" 2>&1 | grep -q 'сервер остановлен' || { echo "ПРОВАЛ: нет записи об остановке" >&2; exit 1; }
echo "smoke: healthy, readyz, POST /v1/users 201, остановка по SIGTERM с кодом 0"
```

`chmod +x messenger/scripts/smoke.sh`. В `messenger/.gitignore` добавить `.smoke-*`. В `check-images.ts` после проверок `messenger:local` — `run('bash', ['scripts/smoke.sh', 'messenger:local'], 120_000)` и вывести его последнюю строку.

- [ ] **Step 21: Run** `npm run check:images` (из `course`) и `npm run check:messenger` — Expected: PASS; smoke печатает итоговую строку.

- [ ] **Step 22: Commit**

```bash
git add messenger course/scripts/check-images.ts
git commit -m "feat(messenger): пробы livez/readyz, дренаж по SIGTERM, таймаут запроса, healthcheck, конфиг сроков

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: `compose.yaml`, урок 1.17, ADR-0008, правки уроков 1.5/1.7/1.8

**Files:**
- Create: `messenger/compose.yaml`, `messenger/docs/adr/0008-пробы-здоровья-и-остановка.md`, `course/modules/01-go-service/17-service-in-container.md`
- Modify: `course/modules/01-go-service/{05-context.md,07-http-adapter.md,08-resources.md}`, `messenger/README.md` (если есть раздел «Запуск»; иначе корневой `README.md` — в задаче 12)

**Interfaces:**
- Consumes: всё из задачи 7; сцены `GracefulShutdown`, `GoMemLimit` (задача 3).
- Produces: `docker compose up` в `messenger/`; регионы `service`, `secrets` в `compose.yaml`.

- [ ] **Step 1: `messenger/compose.yaml`**

```yaml
name: messenger

services:
  messenger:
    build:
      context: .
      args:
        VERSION: compose
    image: messenger:local
    # region service
    # Порт — только на loopback хоста: снаружи сервис доступен через шлюз (модуль 6),
    # а не напрямую.
    ports:
      - "127.0.0.1:8080:8080"
    environment:
      MESSENGER_JWT_KEY_FILE: /run/secrets/jwt_key
      # Мягкий лимит GC ~90 % от mem_limit: рантайм Go не читает memory.max сам,
      # и без этой строки куча растёт до OOM-kill (урок 1.17).
      GOMEMLIMIT: 230MiB
    secrets:
      - jwt_key
    # Безопасность запуска: файловая система только для чтения, ни одной capability,
    # запрет повышать привилегии через setuid. Мессенджеру ничего из этого не нужно.
    read_only: true
    tmpfs:
      - /tmp
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
    user: "65532:65532"
    mem_limit: 256m
    cpus: 1
    # Столько же, сколько Kubernetes ждёт по умолчанию: дренаж + остановка укладываются.
    stop_grace_period: 30s
    # endregion service
    # healthcheck не задаём: он есть в образе (HEALTHCHECK в Dockerfile).

# region secrets
# Ключ — файлом, а не переменной окружения: окружение видно в docker inspect и
# /proc/<pid>/environ. Ключ создаёт go run ./cmd/keygen -out jwt.pem.
secrets:
  jwt_key:
    file: ./jwt.pem
# endregion secrets
```

- [ ] **Step 2: Прогнать вручную и собрать вывод для урока**

```bash
cd messenger
go run ./cmd/keygen -out jwt.pem
docker compose up -d --build
docker compose ps                       # STATUS: Up … (healthy)
curl -s localhost:8080/livez; curl -s localhost:8080/readyz
```

- сквозной сценарий урока 1.14 через `curl` против контейнера (регистрация, вход, чат, сообщение) — реальный вывод;
- остановка: `time docker compose stop` и `docker compose logs messenger | tail -5` — видны «останавливаюсь: /readyz отвечает 503», «сервер остановлен», время ≈ `DRAIN_DELAY`;
- PID 1 и сигналы: `docker compose exec` невозможен (нет shell) — показать `docker top messenger-messenger-1` и `docker inspect -f '{{.Config.Entrypoint}}'`; для контраста собрать временный образ на базе `busybox:1.37@sha256:…` + бинарь с обёрткой `start.sh`, которая запускает `/messenger` без `exec` (файлы `course/modules/01-go-service/examples/dockerfiles/wrapper.Dockerfile` и `start.sh`), и показать, что `docker stop` ждёт 10 с и завершает кодом 137. Там же измерить и честно описать shell-форму `ENTRYPOINT /messenger`: многие `sh` (dash, busybox ash) выполняют последнюю простую команду через `exec`, и тогда приложение всё-таки становится PID 1 — проверить на этом образе и написать, что получилось;
- ловушка `127.0.0.1`: `docker run … -e MESSENGER_ADDR=127.0.0.1:8080 …` → `curl` с хоста: `connection reset`/`empty reply`, внутри — работает (через healthcheck: контейнер healthy, а снаружи недоступен);
- лимиты: `docker stats --no-stream`, `cat /sys/fs/cgroup/…/memory.max` через `nsenter` (как в 1.15) — `268435456`; `GOMAXPROCS`: в стартовом журнале его нет — показать через временный запуск `docker run --rm --cpus 1 --entrypoint /messenger messenger:local version` не поможет; вместо этого — короткая программа в уроке (`<!-- go-check -->`) с `runtime.GOMAXPROCS(0)` и прогон её в контейнере `golang:1.26.9-trixie` с `--cpus 1` (реальный вывод `1`) и без (`= числу ядер VM`).
- `docker compose down`.

Если на Docker Desktop `docker compose up` падает на чтении ключа — это противоречит спайку; разобраться и описать в уроке.

- [ ] **Step 3: ADR-0008**

`messenger/docs/adr/0008-пробы-здоровья-и-остановка.md`: контекст (оркестратор решает о перезапуске и трафике по пробам; потерянные запросы при выкатке); силы; варианты — одна проба `/health` с проверкой всего; liveness = readiness; liveness без зависимостей + readiness с зависимостями и дренажем; пробы внутри цепочки middleware и в контракте; пробы на отдельном порту; решение — третий вариант, пробы на корневом mux вне цепочки и контракта, тот же порт (отдельный порт — когда появятся метрики, модуль наблюдаемости); последствия, включая «что пересмотрит решение».

- [ ] **Step 4: Урок 1.17 (контракт)**

`17-service-in-container.md`, `title: "1.17. Сервис в контейнере: пробы, сигналы, лимиты"`. Разделы:
- **Проблема** — выкатка новой версии теряет запросы: старый процесс убит посреди ответа, новый получает трафик раньше, чем готов; контейнер «Up», а сервис не отвечает; OOM-kill без видимой причины.
- **Теория:**
  - PID 1: ядро не применяет к нему действия сигналов по умолчанию (сигнал без обработчика игнорируется), сироты и зомби; **но Go-рантайм ставит свои обработчики**: Go-программа без `signal.Notify` на SIGTERM в роли PID 1 не зависает, а сразу завершается с кодом 2 (`runtime.dieFromSignal`, сверить по исходникам Go 1.26) — без `Shutdown` и дренажа; застревает сигнал в `sh`-обёртке без `exec` и в программах на других языках; exec-форма против shell-формы `ENTRYPOINT`/`CMD`; `tini` и `docker run --init` — когда нужны (дочерние процессы), почему нам нет;
  - жизненный цикл остановки в Docker и Kubernetes: SIGTERM → grace period → SIGKILL; `docker stop -t`, `stop_grace_period`, `terminationGracePeriodSeconds`, `preStop`;
  - пробы: liveness, readiness, startup; что проверяет каждая и чего не должна; каскадный рестарт как антипример; дренаж и почему `DrainDelay` > 0 (балансировщик узнаёт со следующей пробой; в Kubernetes ещё и задержка обновления Endpoints); HEALTHCHECK в Docker и чем он отличается от проб Kubernetes (Kubernetes его игнорирует);
  - таймауты: четыре таймаута `http.Server` (схема по времени: соединение → заголовки → тело → обработчик → запись → keep-alive), таймаут запроса на контексте, их соотношение (`WriteTimeout` > таймаут запроса); почему не `http.TimeoutHandler`; что Timeout не умеет (остановить обработчик, не смотрящий на ctx);
  - 12-factor: конфиг в окружении, проверка при старте, секреты — файлами (почему не env), ловушка `127.0.0.1` в контейнере;
  - Go в cgroup: `GOMAXPROCS` (Go 1.25+: учитывает `cpu.max`, пересчитывает при изменении; до 1.25 — `automaxprocs`), `GOMEMLIMIT` (Go 1.19+, мягкий лимит, не читается из cgroup; ~90 % от лимита; что будет при утечке — GC-шторм вместо OOM);
  - безопасность запуска: nonroot, read-only rootfs + tmpfs, `cap_drop: ALL`, `no-new-privileges`; права на файл секрета: Docker Desktop против Docker Engine (спайк), как это делает Kubernetes (`defaultMode`, `fsGroup`);
  - `<Deep>` — «Почему потерянные при выкатке запросы — системная проблема»: распределённое согласование «кто получает трафик» (балансировщик, kube-proxy/iptables, Endpoints) без общей синхронизации — отсюда задержки и обходной путь `preStop: sleep`; связь с 0b (keep-alive соединения после Shutdown: `Connection: close`).
- **Визуализация** — `<GracefulShutdown />` и `<GoMemLimit />`.
- **Код** — регионы `probes`, `timeout`, `drain`, `config`, `root-mux`, `healthcheck` (Go), `healthcheck` (Dockerfile), `service`, `secrets` (compose); каждый разобран.
- **Живой контейнер** (`<Part kind="practice" />`) — весь реальный вывод шага 2.
- **Что изменилось в мессенджере** — список файлов, новые переменные окружения таблицей, ADR-0008; тег `m01-l17`.
- **Типичные ошибки** — liveness проверяет базу; `/health` через auth-middleware; shell-форма `ENTRYPOINT`; ключ в env; `127.0.0.1` в контейнере; нет `GOMEMLIMIT`; `Shutdown` без дренажа; `os.Exit` в обработчике сигнала (defer не выполнятся).
- **Компромисс** — длина дренажа: дольше — медленнее выкатка, короче — потерянные запросы; как выбирать (период пробы × порог + задержка Endpoints).
- **Самопроверка** — 6–8. **Источники** — документация Kubernetes (Pod lifecycle, probes, termination), Docker (`HEALTHCHECK`, `--init`, `stop`), Go `net/http` (`Server`, `Shutdown`), Go 1.25 release notes (GOMAXPROCS), «A Guide to the Go Garbage Collector» (GOMEMLIMIT), The Twelve-Factor App, Cloudflare «The complete guide to Go net/http timeouts».

- [ ] **Step 5: Правки уроков 1.5, 1.7, 1.8**

- 1.5 и 1.7: места, где сказано, что middleware таймаута запроса «не сделан, в бэклоге», переписать: «таймаут на каждый запрос ставит middleware `Timeout` — он появится в уроке 1.17» (со ссылкой).
- 1.8: после описания цепочки одна строка: «В уроке 1.17 между `Recover` и аутентификацией встанет `Timeout` — срок на каждый запрос»; плеер `request-pipeline` не меняется (рулинг 7).
- Сверить, что ни один урок больше не обещает несделанного про таймаут (`grep -rn "таймаут" course/modules/01-go-service`).

- [ ] **Step 6: Проверить**

Run: `npm run build && npm run check:refs && npm run check:messenger && npm test`
Expected: PASS.

- [ ] **Step 7: Commit и тег**

```bash
git add messenger/compose.yaml messenger/docs/adr/0008-*.md course/modules/01-go-service
git commit -m "docs(course): урок 1.17 — сервис в контейнере; compose.yaml и ADR-0008

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l17
```

---

### Task 9: CI — `ci.yml`, урок 1.18, ADR-0009

**Files:**
- Create: `.github/workflows/ci.yml`, `messenger/docs/adr/0009-ci-агрегирующая-проверка-и-закрепление-по-sha.md`, `course/modules/01-go-service/18-ci.md`
- Modify: `course/package.json`

**Interfaces:**
- Consumes: `check:refs`, `check:labs`, `check:go`, `scripts/smoke.sh`, `.nvmrc`; сцены `CiGraph`, `SupplyChainSandbox`.
- Produces: workflow `ci` с джобой `ci-ok` (её имя — обязательная проверка в ruleset, задача 13); `npm run check:workflows`; регионы `filter`, `gates`, `ci-ok`, `image` в `ci.yml`. Задача 10 ссылается на workflow по имени `ci`.

- [ ] **Step 1: `ci.yml`**

```yaml
name: ci

on:
  pull_request:
  push:
    branches: [main]
  workflow_dispatch:

# Токен по умолчанию — только чтение; джобам, которым нужно больше, права выдаются точечно.
permissions:
  contents: read

# Новый пуш в PR отменяет устаревший прогон; на main прогоны не отменяются — каждый
# коммит main должен получить свой результат.
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}

env:
  # Ветка 1.26 с последним патчем, а не go-version-file: в go.mod записано 1.26.0, и
  # govulncheck нашёл бы уязвимости стандартной библиотеки, закрытые в патчах.
  GO_VERSION: "1.26.x"
  GOLANGCI_LINT_VERSION: v2.12.2
  GOVULNCHECK_VERSION: v1.8.0
  ACTIONLINT_VERSION: v1.7.12
  TRIVY_VERSION: v0.74.0

jobs:
  # region filter
  # Монорепо: правка урока не должна гонять тесты мессенджера. На push в main
  # проверяется всё — main обязан быть зелёным целиком.
  changes:
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    permissions:
      contents: read
      pull-requests: read # paths-filter читает список файлов PR через API
    outputs:
      messenger: ${{ github.event_name != 'pull_request' || steps.filter.outputs.messenger }}
      course: ${{ github.event_name != 'pull_request' || steps.filter.outputs.course }}
      labs: ${{ github.event_name != 'pull_request' || steps.filter.outputs.labs }}
      workflows: ${{ github.event_name != 'pull_request' || steps.filter.outputs.workflows }}
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: dorny/paths-filter@ceb8a2b8f2d89434be7ff52d3de7ec3738c5cc9d # v4.0.3
        id: filter
        if: github.event_name == 'pull_request'
        with:
          filters: |
            messenger:
              - 'messenger/**'
              - '.github/workflows/ci.yml'
            course:
              - 'course/**'
              - 'messenger/**'
              - 'labs/**'
              - '.github/workflows/ci.yml'
            labs:
              - 'labs/**'
              - 'course/scripts/check-labs.ts'
              - '.github/workflows/ci.yml'
            workflows:
              - '.github/**'
  # endregion filter

  # region gates
  messenger-lint:
    needs: changes
    if: needs.changes.outputs.messenger == 'true'
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: messenger/go.sum
      - uses: golangci/golangci-lint-action@ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a # v9.3.0
        with:
          version: ${{ env.GOLANGCI_LINT_VERSION }}
          working-directory: messenger

  messenger-test:
    needs: changes
    if: needs.changes.outputs.messenger == 'true'
    runs-on: ubuntu-24.04
    timeout-minutes: 15
    defaults:
      run:
        working-directory: messenger
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: messenger/go.sum
      # -count=1 — без кэша тестов: CI проверяет код, а не прошлый результат.
      - run: go test -race -count=1 -coverprofile=coverage.out ./...
      - uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a # v7.0.1
        with:
          name: coverage
          path: messenger/coverage.out
          retention-days: 14

  messenger-generate:
    needs: changes
    if: needs.changes.outputs.messenger == 'true'
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    defaults:
      run:
        working-directory: messenger
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: messenger/go.sum
      # На чистом checkout сравнение с git строгое: любое отличие — забытый go generate.
      - run: go generate ./...
      - name: Сгенерированный код совпадает с контрактом
        run: |
          git diff --exit-code -- .
          test -z "$(git status --porcelain -- .)"

  messenger-vuln:
    needs: changes
    if: needs.changes.outputs.messenger == 'true'
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    defaults:
      run:
        working-directory: messenger
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: messenger/go.sum
      # govulncheck смотрит, вызывается ли уязвимый код, а не только есть ли модуль в go.sum.
      - run: go run "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" ./...

  course:
    needs: changes
    if: needs.changes.outputs.course == 'true'
    runs-on: ubuntu-24.04
    timeout-minutes: 15
    defaults:
      run:
        working-directory: course
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020 # v7.0.0
        with:
          node-version-file: course/.nvmrc
          cache: npm
          cache-dependency-path: course/package-lock.json
      # check:go компилирует Go-фрагменты уроков.
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: messenger/go.sum
      - run: npm ci
      - run: npm test
      - run: npm run typecheck
      - run: npm run check:go
      - run: npm run check:refs
      - run: npm run build

  labs:
    needs: changes
    if: needs.changes.outputs.labs == 'true'
    runs-on: ubuntu-24.04
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020 # v7.0.0
        with:
          node-version-file: course/.nvmrc
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: labs/go.mod
      # check-labs.ts не тянет npm-зависимостей: npm ci не нужен. Сценарий 1c запускает
      # privileged-контейнер — раннеры GitHub это разрешают.
      - run: node scripts/check-labs.ts
        working-directory: course

  image:
    needs: changes
    if: needs.changes.outputs.messenger == 'true'
    runs-on: ubuntu-24.04
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version: ${{ env.GO_VERSION }}
          cache-dependency-path: messenger/go.sum
      - uses: docker/setup-buildx-action@f87e5991a6d7451dcb8d9637bfbc97413f497069 # v4.4.1
      # region image
      # На PR образ только собирается и проверяется; публикует release.yml после зелёного main.
      - uses: docker/build-push-action@c3c9e263c25d99ce0380d002d59b67737d91b0dc # v7.4.0
        with:
          context: messenger
          load: true
          push: false
          tags: messenger:ci
          build-args: VERSION=ci-${{ github.sha }}
          cache-from: type=gha,scope=messenger
          cache-to: type=gha,mode=max,scope=messenger
      - uses: aquasecurity/trivy-action@ed142fd0673e97e23eac54620cfb913e5ce36c25 # v0.36.0
        with:
          image-ref: messenger:ci
          version: ${{ env.TRIVY_VERSION }}
          severity: HIGH,CRITICAL
          ignore-unfixed: true
          exit-code: "1"
      - name: Smoke
        run: scripts/smoke.sh messenger:ci
        working-directory: messenger
      # endregion image

  workflows:
    needs: changes
    if: needs.changes.outputs.workflows == 'true'
    runs-on: ubuntu-24.04
    timeout-minutes: 5
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: false
      - run: go run "github.com/rhysd/actionlint/cmd/actionlint@${ACTIONLINT_VERSION}"
  # endregion gates

  # region ci-ok
  # Единственная обязательная проверка в ruleset. Джоба, пропущенная по if:, для
  # GitHub — успех; поэтому список обязательных проверок по отдельным джобам принял бы
  # «всё пропущено» (например, упала changes или прогон отменили) за зелёный PR и
  # требовал бы правки ruleset при каждом переименовании джобы. ci-ok запускается
  # всегда и падает, если хоть одна джоба упала или отменена.
  ci-ok:
    if: always()
    needs: [changes, messenger-lint, messenger-test, messenger-generate, messenger-vuln, course, labs, image, workflows]
    runs-on: ubuntu-24.04
    timeout-minutes: 2
    steps:
      - name: Все ворота пройдены или пропущены по фильтру
        env:
          RESULTS: ${{ toJSON(needs.*.result) }}
        run: |
          echo "$RESULTS"
          if echo "$RESULTS" | grep -Eq '"(failure|cancelled)"'; then
            exit 1
          fi
  # endregion ci-ok
```

Проверить по документации версии `trivy-action`, что вход `version` существует (иначе удалить строку и закрепить версию trivy через `TRIVY_VERSION` в `env` шага, как велит README этой версии); то же — `cache-dependency-path` у `setup-go` v7 и `node-version-file` у `setup-node` v7. Расхождение с документацией — исправить и записать в отчёт.

- [ ] **Step 2: `check:workflows`**

`package.json`: `"check:workflows": "cd .. && go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12"`.

Run: `npm run check:workflows` — Expected: без замечаний. `shellcheck`, если не установлен, `actionlint` пропускает — указать это в отчёте; если установлен (`apt install shellcheck`) — замечаний нет и в `run:`-блоках.

- [ ] **Step 3: ADR-0009**

`0009-ci-агрегирующая-проверка-и-закрепление-по-sha.md`: контекст (монорепо, одиночный разработчик + будущие PR от ботов, публичный репозиторий); решения — фильтр путей + агрегирующая `ci-ok` как единственная обязательная; actions по полному SHA + Dependabot для обновления; минимальные `permissions`; `pull_request`, а не `pull_request_target`; Go `1.26.x`; варианты — без фильтра (всё всегда), фильтр через `on.paths` (пропущенный workflow оставляет обязательные проверки в Pending), обязательные проверки по каждой джобе (пропуск по `if:` считается успехом, имена джоб зашиты в ruleset), матрица по каталогам, отдельные workflow на каталог; последствия. Факты о поведении пропусков — сверить с GitHub Docs «Troubleshooting required status checks» и сослаться.

- [ ] **Step 4: Урок 1.18 (контракт)**

`18-ci.md`, `title: "1.18. CI: ворота качества, которым можно доверять"`. Разделы:
- **Проблема** — «у меня тесты проходили»: забыли `go generate`, гонка проявилась только с `-race`, уязвимость в зависимости; ручные проверки забывают; CI как контракт команды.
- **Теория:**
  - CI и CD: что проверяем на каждом изменении и почему быстро;
  - GitHub Actions: событие → workflow → джобы (параллельно, `needs`) → шаги; раннер (свежая VM на каждую джобу, ubuntu-24.04, что там предустановлено); `uses` против `run`; контексты и выражения; `env`, `outputs`, артефакты, кэш (`setup-go`/`setup-node`, `type=gha` у buildx);
  - ворота качества мессенджера — таблица: что ловит каждая (lint, `-race`, generate, govulncheck, сборка + trivy + smoke, тесты курса, лабы, actionlint);
  - монорепо и фильтр путей; пропуск по `if:` = Success, пропуск workflow по `on.paths` = Pending навсегда (GitHub Docs, «Troubleshooting required status checks»); зачем агрегирующая джоба: одно стабильное имя в ruleset и «упала/отменена хоть одна джоба → красный», включая падение самой `changes`;
  - безопасность pipeline: `GITHUB_TOKEN` и `permissions`; `pull_request` против `pull_request_target` (PR из форка, «pwn request»); подстановка `${{ }}` в `run:` как инъекция (и почему мы передаём через `env`); закрепление по SHA — инцидент tj-actions/changed-files (март 2025, CVE-2025-30066): что произошло, кого задело, что спасло; `persist-credentials: false`;
  - защита ветки: ruleset (обязательные проверки, запрет force-push, PR), secret scanning и push protection, CodeQL default setup — что включаем руками в задаче 13 и почему это часть CI;
  - `<Deep>` — «Почему CI должен быть быстрым и детерминированным»: исследования DORA (lead time, частота выкатки), flaky-тесты и доверие к красному статусу, `-count=1`, закреплённые версии инструментов.
- **Визуализация** — `<CiGraph />`, `<SupplyChainSandbox />`.
- **Код** — регионы `filter`, `gates`, `image`, `ci-ok` из `ci.yml`; разбор каждого блока; `npm run check:workflows` с реальным выводом.
- **Что изменилось в репозитории** — `ci.yml`, ADR-0009, `check:workflows`; тег `m01-l18`. Отдельно: «первый прогон на GitHub — после публикации репозитория» (шаги для ученика: `gh repo create`, `git push`, вкладка Actions, настройка ruleset; ссылка на 1.19).
- **Типичные ошибки** — `uses: …@main`; `permissions: write-all` «чтобы заработало»; секрет в `echo`; фильтр путей через `on.paths` в workflow с обязательными проверками; кэш, ключ которого не зависит от `go.sum`; тесты без `-race` в CI; `pull_request_target` с checkout кода PR.
- **Компромисс** — полнота против скорости: что гонять на PR, что на main, что ночью (здесь — всё на PR, потому что быстро; когда станет медленно — матрица и шардирование).
- **Самопроверка** — 6–8. **Источники** — GitHub Docs (Actions: workflow syntax, security hardening, `GITHUB_TOKEN` permissions, rulesets, required status checks), advisory GHSA для tj-actions/changed-files (CVE-2025-30066), OpenSSF Scorecard (Pinned-Dependencies, Token-Permissions), документация govulncheck, trivy, actionlint; «Accelerate» (Forsgren, Humble, Kim).

- [ ] **Step 5: Проверить**

Run: `npm run check:workflows && npm run build && npm run check:refs && npm test`
Expected: PASS.

- [ ] **Step 6: Commit и тег**

```bash
git add .github/workflows/ci.yml messenger/docs/adr/0009-*.md course/modules/01-go-service/18-ci.md course/package.json
git commit -m "ci: workflow ci с фильтром путей и агрегирующей проверкой; урок 1.18 и ADR-0009

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l18
```

---
### Task 10: Доставка — `release.yml`, `pages.yml`, Dependabot, базовый путь курса, урок 1.19, ADR-0010

**Files:**
- Create: `.github/workflows/release.yml`, `.github/workflows/pages.yml`, `.github/dependabot.yml`
- Modify: `course/.vitepress/config.ts`, `course/package.json`
- Create: `course/scripts/check-base.ts`
- Create: `messenger/docs/adr/0010-доставка-ghcr-и-аттестации.md`, `course/modules/01-go-service/19-delivery.md`

**Interfaces:**
- Consumes: workflow `ci` (задача 9), `messenger/Dockerfile`.
- Produces: образ `ghcr.io/<owner>/messenger` (теги `sha-<7>`, `main`, на релизе `X.Y.Z`, `X.Y`, `latest`); сайт курса на Pages; `COURSE_BASE` (переменная окружения сборки курса, по умолчанию `/`); `npm run check:base`; регионы `release-tags`, `release-build`, `attest` (release.yml), `pages` (pages.yml), `dependabot` (dependabot.yml).

- [ ] **Step 1: Базовый путь курса**

`course/.vitepress/config.ts`:

```ts
// На GitHub Pages курс живёт по адресу https://<owner>.github.io/<repo>/: все ссылки
// и ассеты должны начинаться с /<repo>/. Локально (npm run dev) — с корня.
const BASE = process.env.COURSE_BASE ?? '/'
if (!BASE.startsWith('/') || !BASE.endsWith('/')) throw new Error(`COURSE_BASE должен начинаться и заканчиваться на «/»: ${BASE}`)
```

В `defineConfig`: `base: BASE`; `head` — `href: \`${BASE}favicon.svg\`` (VitePress не добавляет base к ссылкам в `head` сам). Найти в `course/.vitepress/theme`, `course/viz`, `course/glossary` и уроках абсолютные пути в шаблонах (`href="/`, `src="/`, `url(/`) и перевести на `withBase()` из `vitepress` или на относительные; Markdown-ссылки `[…](/modules/…)` VitePress переписывает сам — их не трогать.

- [ ] **Step 2: `check:base`**

`course/scripts/check-base.ts`:

```ts
import { execFileSync } from 'node:child_process'
import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

// Собирает курс под подпутём, как на GitHub Pages, и ищет ссылки мимо него: такой
// href работает локально и ведёт в 404 на Pages.
const COURSE = fileURLToPath(new URL('..', import.meta.url))
const BASE = '/check-base/'
const DIST = join(COURSE, '.vitepress', 'dist')

execFileSync('npx', ['vitepress', 'build'], { cwd: COURSE, env: { ...process.env, COURSE_BASE: BASE }, stdio: 'inherit', timeout: 600_000 })

function files(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) return files(p)
    return /\.(html|css|js)$/.test(e.name) ? [p] : []
  })
}

// Абсолютный путь без базы: href="/x", src="/x", url(/x). Протокол-относительные
// (//cdn) и якоря не считаются.
const BAD = /(?:href|src)=["']\/(?!\/|check-base\/)|url\(\s*["']?\/(?!\/|check-base\/)/g
const problems: string[] = []
for (const f of files(DIST)) {
  const text = readFileSync(f, 'utf8')
  for (const m of text.matchAll(BAD)) {
    const at = m.index ?? 0
    problems.push(`${relative(DIST, f)}: …${text.slice(Math.max(0, at - 40), at + 60)}…`)
  }
}
if (problems.length > 0) {
  console.error(`Ссылки мимо базового пути ${BASE}:\n${problems.slice(0, 50).join('\n')}`)
  process.exit(1)
}
console.log(`Курс под ${BASE}: ссылок мимо базового пути нет.`)
```

`package.json`: `"check:base": "node scripts/check-base.ts"`. После прогона обычная сборка (`npm run build`) пересобирает `dist` с `/` — указать в отчёте, что `check:base` перезаписывает `dist`.

Run: `npm run check:base` — Expected: итог без ошибок (исправить все найденные ссылки в шаге 1). JS-бандлы VitePress содержат строки вида `"/"` для роутера — если регулярка ловит заведомо безопасные места в `.js`, сузить проверку до `.html` и `.css` и записать это в отчёт с примером.

- [ ] **Step 3: `release.yml`**

```yaml
name: release

# Публикуем только проверенное: после зелёного ci на main и на тег релиза.
on:
  workflow_run:
    workflows: [ci]
    types: [completed]
    branches: [main]
  push:
    tags: ["v*"]

permissions: {}

concurrency:
  group: release-${{ github.ref }}
  cancel-in-progress: false

jobs:
  image:
    # workflow_run приходит и на упавший ci, и на ci из PR — их пропускаем.
    if: github.event_name == 'push' || (github.event.workflow_run.conclusion == 'success' && github.event.workflow_run.event == 'push')
    runs-on: ubuntu-24.04
    timeout-minutes: 30
    permissions:
      contents: read
      packages: write # пуш в GHCR тем же GITHUB_TOKEN, без личного токена
      id-token: write # OIDC-токен для keyless-подписи аттестации (Sigstore)
      attestations: write
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          # workflow_run видит последний коммит main, а собирать надо проверенный.
          ref: ${{ github.event.workflow_run.head_sha || github.sha }}
          persist-credentials: false
      - name: Имя образа
        env:
          OWNER: ${{ github.repository_owner }}
        # Имена в GHCR — только в нижнем регистре, а логин владельца может быть любым.
        run: echo "IMAGE=ghcr.io/${OWNER,,}/messenger" >> "$GITHUB_ENV"
      - uses: docker/setup-buildx-action@f87e5991a6d7451dcb8d9637bfbc97413f497069 # v4.4.1
      - uses: docker/login-action@dbcb813823bdd20940b903addbd779551569679f # v4.6.0
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      # region release-tags
      # context: git — теги и метки из реально собранного коммита (head_sha), а не из
      # github.sha события workflow_run.
      - uses: docker/metadata-action@dc802804100637a589fabce1cb79ff13a1411302 # v6.2.0
        id: meta
        with:
          context: git
          images: ${{ env.IMAGE }}
          flavor: latest=false
          tags: |
            type=sha,prefix=sha-,format=short
            type=raw,value=main,enable=${{ github.event_name == 'workflow_run' }}
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=raw,value=latest,enable=${{ startsWith(github.ref, 'refs/tags/v') }}
      # endregion release-tags
      # region release-build
      - uses: docker/build-push-action@c3c9e263c25d99ce0380d002d59b67737d91b0dc # v7.4.0
        id: build
        with:
          context: messenger
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          build-args: VERSION=${{ steps.meta.outputs.version }}
          cache-from: type=gha,scope=messenger
          cache-to: type=gha,mode=max,scope=messenger-release
          # SBOM и provenance BuildKit кладёт в индекс образа как аттестации.
          sbom: true
          provenance: mode=max
      # endregion release-build
      # region attest
      # Аттестация GitHub: подписанное через Sigstore утверждение «этот digest собран
      # этим workflow из этого коммита». Проверка — gh attestation verify.
      - uses: actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8 # v4.2.2
        with:
          subject-name: ${{ env.IMAGE }}
          subject-digest: ${{ steps.build.outputs.digest }}
          push-to-registry: true
      # endregion attest
```

Сверить с README закреплённых версий: имя входа `context` у `metadata-action` v6 и значения `git`/`workflow`; `provenance: mode=max` и `sbom: true` у `build-push-action` v7; входы `attest-build-provenance` v4. Расхождения — исправить и записать в отчёт.

- [ ] **Step 4: `pages.yml`**

```yaml
name: pages

on:
  workflow_run:
    workflows: [ci]
    types: [completed]
    branches: [main]
  workflow_dispatch:

permissions: {}

# Выкладки сайта идут по одной; новая не отменяет начатую — иначе сайт может остаться
# наполовину обновлённым.
concurrency:
  group: pages
  cancel-in-progress: false

jobs:
  # region pages
  build:
    if: github.event_name == 'workflow_dispatch' || (github.event.workflow_run.conclusion == 'success' && github.event.workflow_run.event == 'push')
    runs-on: ubuntu-24.04
    timeout-minutes: 15
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          ref: ${{ github.event.workflow_run.head_sha || github.sha }}
          persist-credentials: false
      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020 # v7.0.0
        with:
          node-version-file: course/.nvmrc
          cache: npm
          cache-dependency-path: course/package-lock.json
      - uses: actions/configure-pages@45bfe0192ca1faeb007ade9deae92b16b8254a0d # v6.0.0
        id: pages
      - run: npm ci
        working-directory: course
      - run: npm run build
        working-directory: course
        env:
          # base_path — «/<repo>» без завершающего слэша.
          COURSE_BASE: ${{ steps.pages.outputs.base_path }}/
      - uses: actions/upload-pages-artifact@fc324d3547104276b827a68afc52ff2a11cc49c9 # v5.0.0
        with:
          path: course/.vitepress/dist

  deploy:
    needs: build
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    permissions:
      pages: write
      id-token: write
    environment:
      name: github-pages
      url: ${{ steps.deploy.outputs.page_url }}
    steps:
      - uses: actions/deploy-pages@368f82528645a54fb793d4d04e342629a3f51346 # v5.0.1
        id: deploy
  # endregion pages
```

- [ ] **Step 5: `dependabot.yml`**

```yaml
# region dependabot
version: 2
updates:
  - package-ecosystem: gomod
    directories: ["/messenger", "/labs"]
    schedule:
      interval: weekly
    # Не брать версии моложе двух недель: у свежего релиза ещё не нашли регрессии и
    # не успели отозвать скомпрометированный (то же правило, что для ручных обновлений).
    cooldown:
      default-days: 14
    groups:
      go-minor:
        update-types: [minor, patch]
  - package-ecosystem: npm
    directory: /course
    schedule:
      interval: weekly
    cooldown:
      default-days: 14
    groups:
      npm-minor:
        update-types: [minor, patch]
  # Обновляет SHA закреплённых actions вместе с комментарием версии.
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
    cooldown:
      default-days: 14
    groups:
      actions:
        update-types: [minor, patch]
  # Обновляет digest базовых образов: закрепление по digest без этого — замороженные уязвимости.
  - package-ecosystem: docker
    directories: ["/messenger", "/labs/0b", "/labs/1c"]
    schedule:
      interval: weekly
    cooldown:
      default-days: 14
# endregion dependabot
```

Сверить с документацией GitHub ключи `directories`, `cooldown.default-days`, `groups.*.update-types` (если `cooldown` не поддерживается для какой-то экосистемы — убрать у неё и написать в уроке).

- [ ] **Step 6: Проверить workflow**

Run: `npm run check:workflows` — Expected: без замечаний (`actionlint` проверяет и `release.yml`, и `pages.yml`; `dependabot.yml` он не проверяет — его схему проверит GitHub при первом пуше, задача 13).

- [ ] **Step 7: Мультиархитектурная сборка локально**

```bash
cd messenger
docker buildx build --platform linux/amd64,linux/arm64 -t messenger:multi .
```

Expected: обе платформы собрались без QEMU (`--platform=$BUILDPLATFORM` в стадии сборки); время сборки второй архитектуры — секунды (кросс-компиляция). Вывод — в урок. Если драйвер `docker` не умеет мультиплатформенные сборки без containerd image store — создать builder `docker buildx create --use --name course` и повторить; в уроке объяснить.

- [ ] **Step 8: ADR-0010**

`0010-доставка-ghcr-и-аттестации.md`: контекст (публичный репозиторий, образ должен быть доступен и проверяем); варианты — Docker Hub, GHCR, свой реестр; подпись — cosign с ключом, keyless Sigstore через аттестации GitHub, без подписи; теги — только `latest`, только semver, sha + main + semver; мультиархитектура через QEMU против кросс-компиляции; решение и последствия (в т. ч. «аттестации проверяются `gh attestation verify`; политика допуска — модуль 13a»).

- [ ] **Step 9: Урок 1.19 (контракт)**

`19-delivery.md`, `title: "1.19. Доставка: реестр, теги, SBOM и происхождение"`. Разделы:
- **Проблема** — «какой код сейчас в проде?», «этот образ собрали мы или кто-то подменил?», «в нашей базе нашли CVE — какие сервисы её содержат?».
- **Теория:**
  - реестр образов, распространение по digest, GHCR, `GITHUB_TOKEN` с `packages: write` вместо личного токена;
  - теги: изменяемые и неизменяемые, стратегия `sha-…` + `main` + semver + `latest` только на релизе; почему деплоить надо по digest;
  - мультиархитектурный образ: индекс (manifest list), `--platform`, `$BUILDPLATFORM`/`$TARGETARCH`, кросс-компиляция против QEMU (реальное время из шага 7);
  - цепочка поставок: SBOM (SPDX/CycloneDX — что внутри и зачем: ответ на «где у нас log4j»), provenance (SLSA v1.0: уровни Build L1–L3, что даёт GitHub-hosted runner), подпись keyless (OIDC → Fulcio → Rekor, без хранения ключа), аттестация GitHub; проверка `gh attestation verify` — что именно она проверяет (подпись, издатель, репозиторий, workflow);
  - Dependabot: что обновляет, группировка, cooldown, обновление digest базовых образов — почему закрепление без автообновления хуже, чем без закрепления;
  - GitHub Pages для курса: `workflow_run`, `configure-pages`, базовый путь;
  - `<Deep>` — «SolarWinds, Codecov, xz: три способа отравить сборку» — что произошло в каждом случае и какая мера SLSA закрыла бы его.
- **Код** — регионы `release-tags`, `release-build`, `attest`, `pages`, `dependabot`; локальная мультиархитектурная сборка с реальным выводом; `docker buildx imagetools inspect messenger:multi` (если образ в локальном хранилище — `docker image inspect`) с реальным выводом.
- **После публикации** (`<Part kind="practice" />`) — пошагово для ученика: `gh repo create … --public --source . --push`, включить Pages (Source: GitHub Actions), сделать пакет публичным, проверить `docker pull ghcr.io/<owner>/messenger:main`, `gh attestation verify oci://ghcr.io/<owner>/messenger@sha256:… --owner <owner>`. Вывод — **пример с пометкой** (рулинг 10); задача 13 заменит реальным.
- **Что изменилось в репозитории** — три файла `.github`, `COURSE_BASE`, `check:base`, ADR-0010; тег `m01-l19`.
- **Типичные ошибки** — деплой по `latest`; PAT в секретах вместо `GITHUB_TOKEN`; публикация из PR; закрепление без Dependabot; SBOM, который никто не читает (нет сканирования по нему); QEMU для Go.
- **Компромисс** — строгость цепочки поставок против трения: SLSA L3 и политика допуска против скорости одиночного проекта; что мы делаем и что откладываем до 13a.
- **Самопроверка** — 6–8. **Источники** — GitHub Docs (GHCR, artifact attestations, `gh attestation verify`, Dependabot options), SLSA v1.0 spec, Sigstore docs, OCI image index, Docker docs (multi-platform builds, attestations), CISA SBOM resources.

- [ ] **Step 10: Проверить**

Run: `npm run check:workflows && npm run check:base && npm run build && npm run check:refs && npm test`
Expected: PASS.

- [ ] **Step 11: Commit и тег**

```bash
git add .github course/.vitepress/config.ts course/scripts/check-base.ts course/package.json messenger/docs/adr/0010-*.md course/modules/01-go-service/19-delivery.md
git commit -m "ci: release в GHCR с аттестациями, Pages для курса, Dependabot; урок 1.19 и ADR-0010

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l19
```

---

### Task 11: Симулятор собеседования и урок 1.20

**Files:**
- Create: `course/interview/{schema.ts,schema.test.ts,session.ts,session.test.ts,bank.test.ts,interview.data.ts,module-01.yaml,InterviewSim.vue}`
- Create: `course/modules/01-go-service/20-interview.md`

**Interfaces:**
- Consumes: уроки 1.1–1.19 (пути файлов), `yaml` (devDependency), `defineLoader` из `vitepress`, `VizFrame`.
- Produces: `parseBank(raw: unknown, source: string): Bank`; `pickSession(questions, size, seed): Session`; `scoreSession(questions, session, marks): Score`; `mulberry32(seed): () => number`; данные `data: Record<string, Bank>` из `interview.data.ts` (ключ — `module`); компонент `<InterviewSim :bank="…" />` — переиспользуется модулями дальше.

- [ ] **Step 1: Тесты схемы**

`course/interview/schema.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { parseBank } from './schema'

const question = {
  id: 'ports-at-consumer',
  lesson: '01-go-service/04-usecases',
  topic: 'Архитектура',
  level: 'M+',
  question: 'Почему порт объявляют в пакете потребителя?',
  answer: 'Потому что…',
  checklist: ['правило зависимостей', 'узкий интерфейс', 'подмена в тестах'],
  red_flags: ['«так принято»'],
  follow_ups: [{ q: 'А если потребителей два?', a: 'У каждого свой узкий интерфейс…' }],
}
const bank = { module: '01', title: 'Модуль 1', questions: [question] }

describe('parseBank', () => {
  it('принимает корректный банк', () => {
    expect(parseBank(bank, 't.yaml').questions[0]?.id).toBe('ports-at-consumer')
  })
  it.each([
    ['нет вопросов', { ...bank, questions: [] }],
    ['уровень не M+/S', { ...bank, questions: [{ ...question, level: 'Junior' }] }],
    ['чеклист короче 3', { ...bank, questions: [{ ...question, checklist: ['a', 'b'] }] }],
    ['чеклист длиннее 6', { ...bank, questions: [{ ...question, checklist: ['1', '2', '3', '4', '5', '6', '7'] }] }],
    ['нет красных флагов', { ...bank, questions: [{ ...question, red_flags: [] }] }],
    ['фоллоу-апов больше 3', { ...bank, questions: [{ ...question, follow_ups: [question.follow_ups[0], question.follow_ups[0], question.follow_ups[0], question.follow_ups[0]] }] }],
    ['пустой ответ', { ...bank, questions: [{ ...question, answer: ' ' }] }],
    ['повтор id', { ...bank, questions: [question, question] }],
    ['id не kebab-case', { ...bank, questions: [{ ...question, id: 'Порты' }] }],
    ['урок без модуля', { ...bank, questions: [{ ...question, lesson: '04-usecases' }] }],
    ['не объект', 'строка'],
  ])('отклоняет: %s', (_, raw) => {
    expect(() => parseBank(raw, 't.yaml')).toThrow(/t\.yaml/)
  })
})
```

- [ ] **Step 2: Run** `npx vitest run interview/schema` — Expected: FAIL.

- [ ] **Step 3: `schema.ts`**

```ts
// Банк вопросов мини-интервью: YAML → проверенные типы. Ошибка называет файл и
// вопрос: банк пишут руками, и опечатку надо найти за секунды.

export type Level = 'M+' | 'S'

export interface FollowUp {
  q: string
  a: string
}

export interface Question {
  id: string
  lesson: string // путь урока без .md относительно course/modules, например 01-go-service/04-usecases
  topic: string
  level: Level
  question: string
  answer: string // Markdown
  checklist: readonly string[]
  red_flags: readonly string[]
  follow_ups: readonly FollowUp[]
}

export interface Bank {
  module: string
  title: string
  questions: readonly Question[]
}

const LIMITS = { checklist: [3, 6], red_flags: [1, 3], follow_ups: [1, 3] } as const
const ID = /^[a-z0-9]+(?:-[a-z0-9]+)*$/
const LESSON = /^[0-9]{2}[a-z0-9-]*\/[0-9]{2}-[a-z0-9-]+$/

type Obj = Record<string, unknown>
const isObj = (v: unknown): v is Obj => typeof v === 'object' && v !== null && !Array.isArray(v)

function text(o: Obj, key: string, where: string): string {
  const v = o[key]
  if (typeof v !== 'string' || v.trim() === '') throw new Error(`${where}: поле ${key} — непустая строка`)
  return v
}

function list<T>(o: Obj, key: keyof typeof LIMITS, where: string, item: (v: unknown, at: string) => T): T[] {
  const v = o[key]
  const [min, max] = LIMITS[key]
  if (!Array.isArray(v) || v.length < min || v.length > max) throw new Error(`${where}: ${key} — от ${min} до ${max} пунктов`)
  return v.map((x, i) => item(x, `${where}.${key}[${i}]`))
}

const str = (v: unknown, at: string): string => {
  if (typeof v !== 'string' || v.trim() === '') throw new Error(`${at}: непустая строка`)
  return v
}

function parseQuestion(raw: unknown, where: string): Question {
  if (!isObj(raw)) throw new Error(`${where}: вопрос — объект`)
  const id = text(raw, 'id', where)
  if (!ID.test(id)) throw new Error(`${where}: id «${id}» — латиница в kebab-case`)
  const at = `${where} (${id})`
  const lesson = text(raw, 'lesson', at)
  if (!LESSON.test(lesson)) throw new Error(`${at}: lesson «${lesson}» — вида 01-go-service/04-usecases`)
  const level = raw.level
  if (level !== 'M+' && level !== 'S') throw new Error(`${at}: level — M+ или S`)
  return {
    id,
    lesson,
    level,
    topic: text(raw, 'topic', at),
    question: text(raw, 'question', at),
    answer: text(raw, 'answer', at),
    checklist: list(raw, 'checklist', at, str),
    red_flags: list(raw, 'red_flags', at, str),
    follow_ups: list(raw, 'follow_ups', at, (v, a) => {
      if (!isObj(v)) throw new Error(`${a}: объект {q, a}`)
      return { q: text(v, 'q', a), a: text(v, 'a', a) }
    }),
  }
}

export function parseBank(raw: unknown, source: string): Bank {
  if (!isObj(raw)) throw new Error(`${source}: банк — объект`)
  const questions = raw.questions
  if (!Array.isArray(questions) || questions.length === 0) throw new Error(`${source}: questions — непустой список`)
  const parsed = questions.map((q, i) => parseQuestion(q, `${source}: questions[${i}]`))
  const seen = new Set<string>()
  for (const q of parsed) {
    if (seen.has(q.id)) throw new Error(`${source}: id «${q.id}» повторяется`)
    seen.add(q.id)
  }
  return { module: text(raw, 'module', source), title: text(raw, 'title', source), questions: parsed }
}
```

- [ ] **Step 4: Run** `npx vitest run interview/schema` — Expected: PASS.

- [ ] **Step 5: Тесты сессии**

`course/interview/session.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import type { Question } from './schema'
import { mulberry32, pickSession, scoreSession } from './session'

const q = (id: string, topic: string, checklist = 4): Question => ({
  id,
  lesson: '01-go-service/01-architecture',
  topic,
  level: 'M+',
  question: id,
  answer: id,
  checklist: Array.from({ length: checklist }, (_, i) => `${id}-${i}`),
  red_flags: ['x'],
  follow_ups: [{ q: 'q', a: 'a' }],
})

const bank = [
  q('a1', 'A'), q('a2', 'A'), q('a3', 'A'),
  q('b1', 'B'), q('b2', 'B'),
  q('c1', 'C'), q('c2', 'C'),
  q('d1', 'D'), q('e1', 'E'), q('f1', 'F'),
]
const topicOf = (id: string) => bank.find((x) => x.id === id)?.topic

describe('mulberry32', () => {
  it('детерминирован и в [0, 1)', () => {
    const a = mulberry32(42), b = mulberry32(42)
    const xs = Array.from({ length: 100 }, () => a())
    expect(xs).toEqual(Array.from({ length: 100 }, () => b()))
    expect(xs.every((x) => x >= 0 && x < 1)).toBe(true)
  })
})

describe('pickSession', () => {
  it('тот же сид — та же сессия, другой — другая', () => {
    expect(pickSession(bank, 5, 7)).toEqual(pickSession(bank, 5, 7))
    expect(pickSession(bank, 5, 7).ids).not.toEqual(pickSession(bank, 5, 8).ids)
  })
  it('размер ограничен банком, вопросы не повторяются', () => {
    const s = pickSession(bank, 50, 1)
    expect(s.ids).toHaveLength(bank.length)
    expect(new Set(s.ids).size).toBe(bank.length)
  })
  it('тем хватает — все темы разные', () => {
    for (const seed of [1, 2, 3, 4, 5]) {
      const topics = pickSession(bank, 6, seed).ids.map(topicOf)
      expect(new Set(topics).size).toBe(6)
    }
  })
  it('две одинаковые темы подряд не идут, если этого можно избежать', () => {
    for (const seed of [1, 2, 3, 4, 5, 6, 7, 8]) {
      const topics = pickSession(bank, 10, seed).ids.map(topicOf)
      for (let i = 1; i < topics.length; i++) expect(topics[i]).not.toBe(topics[i - 1])
    }
  })
  it('неверный размер — ошибка', () => {
    expect(() => pickSession(bank, 0, 1)).toThrow()
  })
})

describe('scoreSession', () => {
  it('считает отмеченные пункты по темам и находит слабые', () => {
    const s = { seed: 1, ids: ['a1', 'b1', 'c1'] }
    const score = scoreSession(bank, s, {
      a1: [true, true, true, true],
      b1: [true, false, false, false],
      // c1 не отмечен вовсе — 0 из 4
    })
    expect(score.total).toEqual({ got: 5, total: 12 })
    expect(score.topics.find((t) => t.topic === 'A')).toMatchObject({ got: 4, total: 4 })
    expect(score.weak.map((t) => t.topic)).toEqual(['C', 'B'])
  })
  it('метки длиннее чеклиста не дают лишних баллов', () => {
    const score = scoreSession(bank, { seed: 1, ids: ['d1'] }, { d1: [true, true, true, true, true, true] })
    expect(score.total).toEqual({ got: 4, total: 4 })
  })
})
```

Выравнивание массива `bank` в одну строку на тему — допустимо; если `prettier`/линтер проекта против — разложить по строке на вопрос.

- [ ] **Step 6: Run** `npx vitest run interview/session` — Expected: FAIL.

- [ ] **Step 7: `session.ts`**

```ts
import type { Question } from './schema'

export interface Session {
  seed: number
  ids: readonly string[]
}

// Отметки самооценки: по вопросу — флажки пунктов чеклиста.
export type Marks = Readonly<Record<string, readonly boolean[]>>

export interface TopicScore {
  topic: string
  lessons: readonly string[]
  got: number
  total: number
}

export interface Score {
  total: { got: number; total: number }
  topics: readonly TopicScore[]
  weak: readonly TopicScore[]
}

// Тема «слабая», если отмечено меньше этой доли пунктов чеклиста.
const WEAK_RATIO = 0.6

// mulberry32 — маленький детерминированный генератор: сессию можно повторить по сиду,
// а тесты не зависят от Math.random.
export function mulberry32(seed: number): () => number {
  let a = seed >>> 0
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

function shuffle<T>(xs: readonly T[], rnd: () => number): T[] {
  const out = [...xs]
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(rnd() * (i + 1))
    ;[out[i], out[j]] = [out[j] as T, out[i] as T]
  }
  return out
}

// pickSession выбирает вопросы так, чтобы покрыть как можно больше тем и не ставить
// одну тему два раза подряд: на настоящем собеседовании темы тоже чередуются.
export function pickSession(questions: readonly Question[], size: number, seed: number): Session {
  if (!Number.isInteger(size) || size < 1) throw new Error(`размер сессии — целое ≥ 1, получено ${size}`)
  const pool = shuffle(questions, mulberry32(seed))
  const used = new Set<string>()
  const ids: string[] = []
  let last: string | undefined
  const left = (topic: string) => pool.filter((q) => q.topic === topic && !ids.includes(q.id)).length
  while (ids.length < Math.min(size, questions.length)) {
    const free = pool.filter((q) => !ids.includes(q.id))
    // Сначала — новая тема (в случайном порядке пула).
    const fresh = free.find((q) => !used.has(q.topic) && q.topic !== last)
    // Новых тем нет — тема, где осталось больше всего вопросов: иначе в конце
    // останутся вопросы одной темы и пойдут подряд (жадный алгоритм перестановки).
    const busiest = free
      .filter((q) => q.topic !== last)
      .reduce<Question | undefined>((best, q) => (best === undefined || left(q.topic) > left(best.topic) ? q : best), undefined)
    const next = fresh ?? busiest ?? free[0]
    if (next === undefined) break
    ids.push(next.id)
    used.add(next.topic)
    last = next.topic
  }
  return { seed, ids }
}

export function scoreSession(questions: readonly Question[], session: Session, marks: Marks): Score {
  const byTopic = new Map<string, { lessons: Set<string>; got: number; total: number }>()
  for (const id of session.ids) {
    const q = questions.find((x) => x.id === id)
    if (q === undefined) continue
    const got = (marks[id] ?? []).slice(0, q.checklist.length).filter(Boolean).length
    const t = byTopic.get(q.topic) ?? { lessons: new Set<string>(), got: 0, total: 0 }
    t.lessons.add(q.lesson)
    t.got += got
    t.total += q.checklist.length
    byTopic.set(q.topic, t)
  }
  const topics = [...byTopic].map(([topic, t]) => ({ topic, lessons: [...t.lessons], got: t.got, total: t.total }))
  const ratio = (t: TopicScore) => (t.total === 0 ? 1 : t.got / t.total)
  const weak = topics.filter((t) => ratio(t) < WEAK_RATIO).sort((a, b) => ratio(a) - ratio(b))
  const total = topics.reduce((acc, t) => ({ got: acc.got + t.got, total: acc.total + t.total }), { got: 0, total: 0 })
  return { total, topics, weak }
}
```

- [ ] **Step 8: Run** `npx vitest run interview/session` — Expected: PASS. Если тест на чередование тем падает на каком-то сиде — исправлять алгоритм, а не тест.

- [ ] **Step 9: Банк вопросов (контракт) и его тест**

`course/interview/module-01.yaml`:

```yaml
module: "01"
title: "Модуль 1. Сервис на Go"
questions:
  - id: …
    lesson: 01-go-service/04-usecases
    …
```

- ≈ 25 вопросов (не меньше 22, не больше 30) по урокам 1.1–1.19: хотя бы один на каждый урок, по два-три на 1.8–1.14 и 1.17–1.18; уровни — примерно поровну `M+` и `S`; темы — 8–12 крупных (Архитектура, Домен и ошибки, Context и конкурентность, Тесты, HTTP и REST, Контракт, Ошибки API, Пагинация, Идемпотентность, Безопасность, Контейнеры, CI/CD).
- Каждый вопрос — как задаёт интервьюер (открыто, «почему» и «что будет, если»), а не тест на память; `answer` — эталон 5–15 предложений в структуре «тезис → почему → компромисс → пример из мессенджера»; `checklist` — что интервьюер должен услышать (3–6 пунктов, коротко); `red_flags` — что выдаёт непонимание; `follow_ups` — 1–3 углубления с коротким ответом.
- Примеры тем вопросов (не исчерпывающе, формулировки — свои): порт у потребителя; инвариант агрегата и где его держать; `errors.Is` против сравнения строк; отмена по `context` через всю цепочку; фейк против мока; 404 против 403 для чужого чата; contract-first против code-first; что класть в `detail` RFC 9457; offset против keyset при вставках; два приёма идемпотентности и когда какой; почему Argon2id и что такое фиктивный хеш; что проверять в JWT и почему не HMAC; что изолирует контейнер и что нет; порядок слоёв в Dockerfile; liveness против readiness; GOMEMLIMIT; PID 1 и shell-форма; агрегирующая обязательная проверка; закрепление actions по SHA; что доказывает provenance.

`course/interview/bank.test.ts`:

```ts
import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { parse } from 'yaml'
import { parseBank } from './schema'

const COURSE = fileURLToPath(new URL('..', import.meta.url))
const bank = parseBank(parse(readFileSync(join(COURSE, 'interview', 'module-01.yaml'), 'utf8')), 'module-01.yaml')

describe('банк вопросов модуля 1', () => {
  it('каждый вопрос ссылается на существующий урок', () => {
    for (const q of bank.questions) expect(existsSync(join(COURSE, 'modules', `${q.lesson}.md`)), q.id).toBe(true)
  })
  it('у каждого урока 1.1–1.19 есть хотя бы один вопрос', () => {
    const lessons = readdirSync(join(COURSE, 'modules', '01-go-service'))
      .filter((f) => /^(0[1-9]|1[0-9])-.+\.md$/.test(f))
      .map((f) => `01-go-service/${f.replace(/\.md$/, '')}`)
    expect(lessons).toHaveLength(19)
    const covered = new Set(bank.questions.map((q) => q.lesson))
    expect(lessons.filter((l) => !covered.has(l))).toEqual([])
  })
  it('размер и баланс уровней', () => {
    expect(bank.questions.length).toBeGreaterThanOrEqual(22)
    expect(bank.questions.length).toBeLessThanOrEqual(30)
    const senior = bank.questions.filter((q) => q.level === 'S').length
    expect(senior / bank.questions.length).toBeGreaterThan(0.3)
    expect(senior / bank.questions.length).toBeLessThan(0.7)
  })
})
```

Run: `npx vitest run interview` — Expected: PASS.

- [ ] **Step 10: Загрузчик данных**

`course/interview/interview.data.ts` — по образцу `glossary/terms.data.ts`:

```ts
import { readFileSync } from 'node:fs'
import { basename } from 'node:path'
import { createMarkdownRenderer, defineLoader, type SiteConfig } from 'vitepress'
import { parse } from 'yaml'
import { parseBank, type Bank, type Question } from './schema'

// Ответы и фоллоу-апы пишутся в Markdown — рендерим их при сборке, как глоссарий.
export interface RenderedQuestion extends Question {
  answerHtml: string
}
export interface RenderedBank extends Omit<Bank, 'questions'> {
  questions: readonly RenderedQuestion[]
}

declare const data: Record<string, RenderedBank>
export { data }

export default defineLoader({
  watch: ['./*.yaml'],
  async load(files: string[]): Promise<Record<string, RenderedBank>> {
    const config = (globalThis as { VITEPRESS_CONFIG?: SiteConfig }).VITEPRESS_CONFIG
    if (config === undefined) throw new Error('VITEPRESS_CONFIG недоступен: loader запущен вне VitePress')
    const md = await createMarkdownRenderer(config.srcDir, config.markdown, config.site.base, config.logger)
    const out: Record<string, RenderedBank> = {}
    for (const file of files) {
      const bank = parseBank(parse(readFileSync(file, 'utf8')), basename(file))
      if (out[bank.module] !== undefined) throw new Error(`${basename(file)}: модуль ${bank.module} уже описан`)
      out[bank.module] = { ...bank, questions: bank.questions.map((q) => ({ ...q, answerHtml: md.render(q.answer) })) }
    }
    return out
  },
})
```

- [ ] **Step 11: Компонент `InterviewSim.vue` (контракт)**

`course/interview/InterviewSim.vue`, `props: { bank: RenderedBank }`:
- обёрнут в `VizFrame label="Симулятор собеседования: модуль N"`;
- два режима (переключатель `role="radiogroup"`): «Сессия» и «Все вопросы»;
- **Сессия:** до старта — «Начать» и поле сида (по умолчанию — случайный, показывается; «Повторить по сиду»); 8 вопросов из `pickSession(bank.questions, 8, seed)`; на экране — номер `k / 8`, тема и уровень, текст вопроса, таймер 3:00 (отсчёт; по нулю — только меняет цвет и пишет «время вышло», не скрывает вопрос; таймер на `setInterval`, создаётся в `onMounted`/по старту, очищается в `onBeforeUnmount` и при смене вопроса), `<textarea>` черновика (не сохраняется), кнопка «Показать эталон»;
- после «Показать эталон»: `answerHtml` (через `v-html` — HTML собран при сборке из нашего YAML), чеклист с флажками, красные флаги, фоллоу-апы (раскрывающиеся `<details>`), «Следующий вопрос»;
- итог: «отмечено X из Y пунктов», таблица по темам, список слабых тем со ссылками на уроки (`withBase('/modules/' + lesson)`), «Ещё сессия»;
- **Все вопросы:** список с фильтрами по уроку и уровню, каждый вопрос раскрывается в `<details>` с эталоном, чеклистом, флагами, фоллоу-апами;
- история: после итога в `localStorage` под ключом `interview:v1:<module>` пишется `{ at: ISO, seed, ratio, weak: topic[] }` (последние 10); чтение и запись — в `try/catch`, без хранилища компонент работает, блок «прошлые результаты» просто не показывается; читается в `onMounted`;
- доступность: управление клавиатурой, видимый фокус, `aria-live="polite"` на таймере (не чаще раза в минуту — объявлять только «осталась минута» и «время вышло»), флажки — настоящие `<input type="checkbox">` с `<label>`;
- стиль — палитра `--viz-*`/`--vp-c-*`, обе темы, 375 px без горизонтальной прокрутки.

- [ ] **Step 12: Урок 1.20 (контракт)**

`20-interview.md`, `title: "1.20. Мини-интервью: модуль 1"`. Подключение: `import InterviewSim from '../../interview/InterviewSim.vue'` и `import { data } from '../../interview/interview.data'`, `<InterviewSim :bank="data['01']" />`. Разделы:
- **Проблема** — знать и уметь рассказать — разные навыки; на собеседовании оценивают рассуждение, а не формулировку из учебника.
- **Как устроено техническое собеседование бэкенда Middle+/Senior** (`theory`) — этапы (скрининг, кодинг, system design, поведенческое), что смотрит интервьюер на каждом, чем Senior-ответ отличается от Middle+ (компромиссы, границы применимости, опыт отказов), как отвечать: тезис → почему → компромисс → пример; как говорить «не знаю».
- **Симулятор** (`viz`) — `<InterviewSim />`, как им пользоваться: вслух или письменно, честная самооценка по чеклисту, повтор слабых тем.
- **Сквозная задача: «Спроектируй API сервиса закладок»** (`practice`) — 6 шагов (ресурсы и URI → ошибки → пагинация → идемпотентность → аутентификация → образ и CI), на каждом: что сказать, частые ошибки, вопрос интервьюера и ориентир ответа (ссылки на уроки модуля); разбор свёрнут в `<details>`, чтобы сначала попробовать самому.
- **Итог модуля 1** — что умеет мессенджер на теге `m01-l20`, что ученик может объяснить (список по частям 1a, 1b, 1c), что дальше (модуль 2a — Postgres за тем же портом).
- **Типичные ошибки на собеседовании** — отвечать «как в документации» без «почему»; не задавать уточняющих вопросов в задаче проектирования; молча выбирать первое решение; спорить с интервьюером вместо обсуждения компромисса.
- **Самопроверка** — 3–5 вопросов про сам формат (не дублируя банк). **Источники** — Alex Xu, «System Design Interview» (т. 1, гл. 3 «A framework for system design interviews»), Gayle McDowell, «Cracking the Coding Interview» (главы о поведенческой части), Martin Kleppmann, «Designing Data-Intensive Applications» (как общая опора модуля).

Тег на этом уроке — `m01-l20`; код мессенджера не меняется.

- [ ] **Step 13: Проверить**

Run: `npm test && npm run typecheck && npm run build && npm run check:base`
Expected: PASS. В собранном уроке 1.20 (`npm run preview`) симулятор проходит сессию из 8 вопросов до итога.

- [ ] **Step 14: Commit и тег**

```bash
git add course/interview course/modules/01-go-service/20-interview.md
git commit -m "feat(course): симулятор собеседования и урок 1.20 — мини-интервью модуля 1

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git tag m01-l20
```

---

### Task 12: Итоговая проверка, страница модуля, README, бэклог

**Files:**
- Modify: `course/modules/01-go-service/index.md`, `README.md`, `docs/superpowers/backlog-after-module-0.md`
- Modify (по находкам): уроки 1.1–1.19

**Interfaces:**
- Consumes: всё из задач 1–11.

- [ ] **Step 1: Страница модуля**

`index.md`: в списке «В этом модуле вы:» добавить пункты 1c (контейнер как процесс; образ на distroless; пробы, дренаж и лимиты; CI с обязательной проверкой; доставка с provenance; мини-интервью); в «Уроки модуля» — 15–20 с одной строкой каждый, как у 1–14. Исправить существующую пунктуацию списка, если в нём пункты с точкой и без вперемешку.

- [ ] **Step 2: README**

Корневой `README.md`: раздел про запуск мессенджера в Docker (`go run ./cmd/keygen -out jwt.pem && docker compose up --build`), лабу 1c, новые npm-скрипты (`check:refs`, `check:images`, `check:workflows`, `check:base`), где CI и что он проверяет, что курс будет на Pages после публикации (адрес — в задаче 13). Без дублирования разделов (в 1b был дубль — проверить).

- [ ] **Step 3: Сверка текста с кодом**

- `grep -rn` по урокам 1.1–1.19: упоминания `httpserver.Run(ctx, srv, shutdownTimeout)`, `newApp` (теперь возвращает `*app`), «таймаут … не сделан», `X-User-ID` в настоящем времени, `shutdownTimeout` как константа `main.go` — поправить на актуальное или явно обозначить как состояние на теге урока.
- Регионы, на которые ссылаются уроки 1.1–1.14, существуют (`npm run check:refs`).

- [ ] **Step 4: Бэклог**

`docs/superpowers/backlog-after-module-0.md`: раздел «Выполнено в 1c» (таймаут запроса; строгая свежесть генерации в CI; путь `gen` относительно `messenger/`) и «Остатки модуля 1c» — всё отложенное в ходе задач (из отчётов исполнителей и запаркованных замечаний ревью); отдельной строкой — «первый прогон на GitHub — задача 13 после разрешённого пуша».

- [ ] **Step 5: Полная проверка**

```bash
cd course
npm test && npm run typecheck && npm run check:go && npm run check:refs && npm run check:messenger && npm run check:labs && npm run check:images && npm run check:workflows && npm run check:base && npm run build
for t in m01-l15 m01-l16 m01-l17 m01-l18 m01-l19 m01-l20; do
  git -C .. worktree add -f "/home/idel/DevGo/system-desine/.superpowers/tag-$t" "$t" >/dev/null &&
  (cd "/home/idel/DevGo/system-desine/.superpowers/tag-$t/messenger" && go build ./... && go test -race ./... >/dev/null) &&
  (cd "/home/idel/DevGo/system-desine/.superpowers/tag-$t/labs" && go build ./... && go test -race ./... >/dev/null) &&
  echo "$t ok"
  git -C .. worktree remove --force "/home/idel/DevGo/system-desine/.superpowers/tag-$t"
done
```

Expected: все скрипты без ошибок; шесть строк `m01-lNN ok`.

- [ ] **Step 6: Playwright**

`npm run preview` и Playwright (MCP-инструменты браузера) для уроков 1.15–1.20 на 375 и 1280 px, в тёмной и светлой теме: нет горизонтальной прокрутки (`document.documentElement.scrollWidth <= innerWidth`), песочницы реагируют на кнопки, плееры листаются, полноэкранный режим открывается и закрывается (Esc — в режиме оверлея), симулятор проходит сессию до итога. То же для сборки под `COURSE_BASE=/check-base/` (`npx vitepress preview` после `check:base`) на одном уроке: ассеты и ссылки работают. Снимки экрана — в рабочий каталог SDD, не в репозиторий.

- [ ] **Step 7: Commit**

```bash
git add README.md docs/superpowers/backlog-after-module-0.md course/modules/01-go-service
git commit -m "docs: итог части 1c — страница модуля, README, бэклог, сверка уроков с кодом

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Тег `m01-l20` не переносится, если задача 12 меняла только уроки и README (код на теге тот же). Если задача 12 меняла код мессенджера или лаб — перенести `m01-l20` на этот коммит (`git tag -f`).

---

### Task 13: Первый прогон на GitHub (после слияния и разрешённого пуша)

> **Выполняется не в цикле SDD.** Задача начинается только после того, как пользователь (1) одобрил слияние `feat/module-1c` в `main`, (2) создал публичный репозиторий на GitHub, (3) выполнил `gh auth login` и (4) явно разрешил `git push`. Контроллер выполняет её сам вместе с пользователем; изменения — коммитами в `main` (или PR, если ruleset уже включён), теги `m01-l15…l20` не переносятся.

- [ ] **Step 1:** `git remote add origin …` (адрес даёт пользователь), `git push -u origin main --tags`.
- [ ] **Step 2:** Дождаться `ci` на `main` (`gh run watch`). Каждое падение на настоящих раннерах — найти причину по журналу (`gh run view --log-failed`), исправить, запушить; не ослаблять ворота ради зелёного статуса. Типичные кандидаты: права на файлы в `smoke.sh`, privileged-сценарий лабы на раннере, кэш `type=gha`, вход `version` у `trivy-action`.
- [ ] **Step 3:** После зелёного `ci` — `release` и `pages` (запускаются сами через `workflow_run`); проверить: `docker pull ghcr.io/<owner>/messenger:main` (сделать пакет публичным в настройках пакета — шаг пользователя), `docker buildx imagetools inspect` показывает `linux/amd64` и `linux/arm64`, `gh attestation verify oci://ghcr.io/<owner>/messenger@<digest> --owner <owner>` проходит; курс открывается на `https://<owner>.github.io/<repo>/`, навигация и ассеты работают (Playwright на двух уроках).
- [ ] **Step 4:** Настройки репозитория (пользователь в веб-интерфейсе или контроллер через `gh api` с согласия пользователя): Pages → Source: GitHub Actions; ruleset для `main` (обязательная проверка `ci-ok`, запрет force-push и удаления, PR обязателен, обход — владельцу); Code security: secret scanning, push protection, Dependabot alerts, CodeQL default setup (Go, JavaScript/TypeScript). Проверить, что Dependabot принял `dependabot.yml` (вкладка Insights → Dependency graph → Dependabot, без ошибок конфигурации).
- [ ] **Step 5:** Заменить в уроке 1.19 «пример вывода» реальным (`gh attestation verify`, `imagetools inspect`, адрес Pages), в README — адрес курса и бейдж статуса `ci`; коммит в `main` через PR (проверка ruleset в деле: PR без зелёной `ci-ok` не сливается).
- [ ] **Step 6:** Обновить память проекта: репозиторий опубликован, адрес, что модуль 1 закрыт.
