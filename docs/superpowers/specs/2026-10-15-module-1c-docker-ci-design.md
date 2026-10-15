# Модуль 1, часть 1c «Docker изнутри, CI и доставка»

- **Дата:** 2026-10-15
- **Статус:** черновик на проверку
- **Подпроект:** 6. Порядок: … → 1a (архитектура и домен) → 1b (REST API, JWT) → **этот подпроект** → модуль 2a (Postgres).
- **Основная спецификация:** `docs/superpowers/specs/2026-10-09-course-platform-design.md`. Действуют все стандарты:
  - §1 «Голос автора»;
  - §2.2 подробность, 10 правил, включая самодостаточность;
  - §2.3 визуализация и полноэкранный режим;
  - §2.4 стиль «Ночной терминал», метки, иерархия заголовков;
  - §2.5 стандарт кода.
- **Предыдущие части:** `2026-10-13-module-1a-service-architecture-design.md` и `2026-10-14-module-1b-rest-api-design.md`. Их правила раскладки, слоёв, проверок и связи с курсом действуют и здесь, если ниже не сказано иное.
- **Бэклог:** `docs/superpowers/backlog-after-module-0.md`.
- **Принцип выбора решений:** берём то, что применяют большие продакшен-проекты, а не упрощённый учебный вариант. Упрощённые варианты показываем как контраст.

---

## 0. Место в модуле 1

| Часть | Содержание | Уроки |
|---|---|---|
| 1a (готова) | Слои, DDD-lite, ошибки, `context`, use case'ы и порты, тесты, тонкий HTTP-адаптер | 1.1–1.7 |
| 1b (готова) | Ресурсы и семантика HTTP, middleware, contract-first, RFC 9457, курсоры, идемпотентность, пароли, JWT | 1.8–1.14 |
| **1c** (эта) | Контейнер изнутри, образ, сервис в контейнере, CI, доставка, мини-интервью модуля 1 | 1.15–1.20 |

После 1c модуль 1 закрыт: монолит v0 (пользователи, чаты, сообщения в памяти, REST, JWT) собирается в образ, проходит CI и публикуется в реестр.

## 1. Цель и критерий успеха

**Цель.** Ученик понимает контейнер как процесс Linux, а не как «лёгкую виртуалку», собирает образ Go-сервиса так, как это делают в продакшене, и строит CI/CD, которому можно доверять. На собеседовании он может объяснить:
- какие namespaces и cgroups создаёт `docker run` и что от чего изолирует (и чего не изолирует: ядро общее);
- откуда берётся слой образа, когда кэш сборки инвалидируется и почему порядок строк в Dockerfile важен;
- почему distroless, а не alpine и не scratch, и почему `CGO_ENABLED=0`;
- чем liveness отличается от readiness и почему liveness не проверяет базу;
- почему `ENTRYPOINT` в shell-форме ломает graceful shutdown и что особенного в PID 1;
- что делает Go-рантайм в cgroup: `GOMAXPROCS` против `cpu.max`, `GOMEMLIMIT` против `memory.max`;
- как устроен pipeline: джобы, кэш, обязательные проверки, минимальные права токена, закрепление actions по SHA;
- что такое SBOM и provenance-аттестация и как проверить, что образ собран из этого коммита этим workflow.

**Критерий успеха** — §8.

## 2. Уроки

**Папка:** `course/modules/01-go-service/`. Строка над заголовком — «Модуль 1 · Урок N». Каждый урок, меняющий код, заканчивается тегом `m01-lNN` и разделом «Что изменилось в мессенджере» перед «Типичными ошибками» (как в 1a и 1b).

| Урок | Файл | Основное содержание | Визуализация | Код |
|---|---|---|---|---|
| 1.15 | `15-container-internals.md` | Контейнер — это процесс. Namespaces: PID, mount, UTS, net, IPC, user, cgroup — что каждый прячет. cgroups v2: единая иерархия, контроллеры `memory`, `cpu`, `pids`, файлы `memory.max`, `cpu.max`, `pids.max`, `memory.events`. Rootfs и `pivot_root` против `chroot`. Overlayfs: lowerdir, upperdir, workdir, copy-up. OCI image spec и runtime spec, цепочка `docker` → `containerd` → `runc`. Чего контейнер не изолирует: ядро, время, часть `/proc`; почему privileged-контейнер почти хост. Лаба «контейнер с нуля на Go», затем те же механизмы у настоящего контейнера через `lsns`, `/proc/<pid>/ns`, `/sys/fs/cgroup`, `docker inspect` | Плееры `container-start` и `cgroup-limits` | `labs/1c` |
| 1.16 | `16-image.md` | Образ как стек слоёв и манифест. Кэш BuildKit: что входит в ключ слоя, почему `COPY go.mod go.sum` идёт раньше `COPY . .`, cache mounts для модулей и кэша сборки. Multi-stage. Статическая сборка: `CGO_ENABLED=0`, `-trimpath`, `-ldflags="-s -w -X …"`, `-buildvcs`. Базовые образы: наивный `golang`, alpine, distroless `static:nonroot`, scratch — размер, число пакетов и CVE по `trivy`, всё измерено. Почему distroless: CA-сертификаты, tzdata, `/etc/passwd` для nonroot, нет shell. Закрепление по digest, `.dockerignore`, OCI-метки | **Песочница `layer-cache`** | `messenger/Dockerfile`, `messenger/.dockerignore`; **ADR-0007** |
| 1.17 | `17-service-in-container.md` | PID 1: обработчики сигналов по умолчанию, зомби, когда нужен `tini`/`--init`; exec-форма против shell-формы. Graceful shutdown под оркестратором: SIGTERM → readiness 503 → дренаж → выход до `terminationGracePeriodSeconds`. Пробы: liveness без зависимостей, readiness с зависимостями, startup; каскадный рестарт как антипример. Healthcheck в образе без shell. Таймауты: четыре таймаута `http.Server` и таймаут запроса на контексте, их соотношение; почему не `http.TimeoutHandler`. 12-factor: конфиг из env, секреты файлами, а не env. Go в cgroup: `GOMAXPROCS` (Go ≥ 1.25 учитывает `cpu.max`), `GOMEMLIMIT` и OOM-kill без него. Безопасность запуска: nonroot, read-only rootfs, `cap_drop: ALL`, `no-new-privileges`. Ловушка `127.0.0.1` внутри контейнера | Плеер `graceful-shutdown`; плеер `gomemlimit` | `platform/health`, `middleware.Timeout`, подкоманда `healthcheck`, конфиг таймаутов, `messenger/compose.yaml`; **ADR-0008** |
| 1.18 | `18-ci.md` | Зачем CI: обязательная проверка как контракт команды. GitHub Actions: workflow, событие, джоба, шаг, раннер, `needs`, матрица, артефакты, кэш. Монорепо и фильтр путей; проблема пропущенных обязательных проверок и агрегирующая джоба. Ворота качества: lint, `test -race`, свежесть генерации, `govulncheck`, сборка и smoke-тест образа, `trivy`. Безопасность pipeline: `permissions` по минимуму, `pull_request` против `pull_request_target`, закрепление actions по SHA (инцидент `tj-actions/changed-files`, март 2025), `concurrency`, `timeout-minutes`. Ruleset для `main`, secret scanning и push protection, CodeQL | Плеер `ci-graph`; **песочница `supply-chain`** | `.github/workflows/ci.yml`; **ADR-0009** |
| 1.19 | `19-delivery.md` | Реестр образов и GHCR, аутентификация `GITHUB_TOKEN` без PAT. Теги: изменяемые (`main`, `latest`) против неизменяемых (`sha-…`, digest), semver-теги на релизе. Мультиархитектурный образ: манифест-лист, `--platform=$BUILDPLATFORM` и кросс-компиляция Go вместо QEMU. Цепочка поставок: SBOM (SPDX), provenance по SLSA, keyless-подпись через OIDC и Sigstore, проверка `gh attestation verify`. Dependabot: группировка обновлений, что делать с обновлением базового образа. Публикация курса на GitHub Pages | — | `.github/workflows/release.yml`, `.github/workflows/pages.yml`, `.github/dependabot.yml`; **ADR-0010** |
| 1.20 | `20-interview.md` | Мини-интервью модуля 1: как устроено техническое интервью Middle+/Senior по бэкенду, как отвечать (тезис → почему → компромисс → пример из своего кода). Симулятор с банком вопросов по урокам 1.1–1.19 и сквозная задача «Спроектируй API сервиса закладок» с пошаговым разбором | **Симулятор `InterviewSim`** | — |

## 3. Лаба «контейнер с нуля» (`labs/1c`)

### 3.1 Раскладка

```
labs/
├── go.mod                      # общий модуль лаб (есть с 0b)
└── 1c/
    ├── README.md               # как запустить, пять экспериментов, ожидаемый вывод
    ├── Dockerfile              # окружение лабы: golang + busybox rootfs + util-linux
    ├── compose.yaml            # privileged, cgroup: private
    ├── rootfs.sh               # docker export busybox@sha256:… → rootfs/
    ├── scenario.sh             # сценарный прогон пяти экспериментов для check:labs
    └── cmd/minicontainer/
        ├── main.go             # run / child
        ├── cgroup.go           # создание, лимиты, удаление cgroup v2
        ├── cgroup_test.go
        ├── flags.go            # разбор аргументов и лимитов
        └── flags_test.go
```

### 3.2 Поведение `minicontainer`

- `minicontainer run [--mem 64M] [--pids 32] [--cpu 50%] [--hostname box] <rootfs> <cmd> [args…]`.
- Двухступенчатый запуск, как у runc: `run` перезапускает `/proc/self/exe child …` с `SysProcAttr.Cloneflags = CLONE_NEWUTS|CLONE_NEWPID|CLONE_NEWNS|CLONE_NEWNET|CLONE_NEWIPC`.
- До старта ребёнка `run` создаёт `/sys/fs/cgroup/minicontainer-<id>/`, пишет `memory.max`, `memory.swap.max=0`, `pids.max`, `cpu.max` и помещает ребёнка в cgroup. Способ (запись PID после старта или `SysProcAttr.UseCgroupFD` + `CgroupFD`, Go ≥ 1.22, чтобы ребёнок с первой инструкции был под лимитом) выбирается в плане; предпочтителен второй.
- `child`: `sethostname`; `mount("", "/", "", MS_PRIVATE|MS_REC)`; bind-mount rootfs на себя; `pivot_root(rootfs, rootfs/.oldroot)`; `chdir("/")`; `umount2("/.oldroot", MNT_DETACH)`; `mount proc`; `syscall.Exec` команды.
- После выхода ребёнка `run` читает `memory.events` (`oom_kill`) и `pids.events` (`max`), печатает итог и удаляет cgroup. Код выхода — код ребёнка; для убитого сигналом — `128+N`, как в shell.
- Ошибки каждого системного вызова оборачиваются с именем вызова. `run` без root или без cgroup v2 завершается понятным сообщением.
- Юнит-тесты на чистые части: разбор `--mem`/`--cpu`/`--pids` в значения файлов cgroup (`50%` → `50000 100000`, `64M` → `67108864`), неверные значения.

### 3.3 Пять экспериментов (`scenario.sh`, `README.md`)

1. `ps` внутри — один процесс с PID 1.
2. `hostname box` внутри не меняет hostname окружения лабы.
3. `ip link` внутри — только `lo` в состоянии DOWN.
4. Fork-бомба упирается в `pids.max`; `pids.events` показывает `max > 0`; окружение живо.
5. Выделение памяти сверх `memory.max` — процесс убит, `oom_kill 1`, код выхода 137.

`scenario.sh` проверяет каждый пункт и возвращает ненулевой код при расхождении. `check:labs` запускает его в контейнере лабы, как сценарии 0b.

### 3.4 Сверка с настоящим Docker (часть урока 1.15)

С запущенным `messenger` из `compose.yaml`: `docker inspect` → PID на хосте; `lsns -p <pid>`; `readlink /proc/<pid>/ns/*` против PID 1 хоста; `cat /sys/fs/cgroup/…/memory.max`; `docker inspect --format '{{json .GraphDriver}}'` и `mount | grep overlay` — слои. Если у ученика Docker Desktop, команды выполняются внутри его VM через `docker run --rm -it --privileged --pid=host debian nsenter -t 1 -m -u -n -i sh`; урок показывает оба пути.

## 4. Код мессенджера

### 4.1 Раскладка (новое и изменённое)

```
messenger/
├── Dockerfile                      # new
├── .dockerignore                   # new
├── compose.yaml                    # new
├── cmd/messenger/
│   ├── main.go                     # подкоманда healthcheck, таймауты из конфига
│   ├── app.go                      # health в сборке, Timeout в цепочке, конфиг
│   ├── healthcheck.go              # new: GET /livez, код выхода
│   └── app_test.go                 # + readyz 503 во время shutdown, + конфиг
├── internal/platform/
│   ├── health/                     # new: /livez, /readyz, Checker, Draining
│   └── middleware/timeout.go       # new
└── docs/adr/0007…0010              # new
```

### 4.2 Health

- Пакет `platform/health`. Интерфейс `Checker { Check(ctx context.Context) error }` с именем проверки; `Handler` отдаёт:
  - `GET /livez` → `200 {"status":"ok"}` всегда, пока процесс обслуживает HTTP; зависимостей не проверяет.
  - `GET /readyz` → `200` если все проверки прошли за общий срок (1 с), иначе `503` с телом `{"status":"unavailable","checks":{"<name>":"<error>"}}`. Тексты ошибок проверок — без внутренностей (только имя и короткая причина).
- Состояние «дренаж»: `health.Draining` (атомарный флаг) — первая проверка `/readyz`. `main` выставляет флаг сразу по SIGTERM, ждёт `MESSENGER_DRAIN_DELAY` (по умолчанию 5 с, чтобы балансировщик успел увидеть 503), затем `Shutdown`.
- Пути вне `/v1` и вне `openapi.yaml` (это эндпоинты платформы, а не бизнес-API), публичные (мимо `httpauth`), не пишутся в access log, не проходят `Timeout` и `Idempotency`. Решение зафиксировано в ADR-0008.
- Ответы — `application/json`; не problem+json, потому что это не ошибка API, а состояние. 405 на них — как у остального маршрутизатора.

### 4.3 Healthcheck без shell

- `messenger healthcheck` — подкоманда того же бинаря: `GET http://127.0.0.1:<port>/livez` с таймаутом 2 с, порт из `MESSENGER_ADDR`; код выхода 0 при 200, иначе 1 с сообщением в stderr.
- `Dockerfile`: `HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 CMD ["/messenger","healthcheck"]`.
- Тест — против `httptest.Server`: 200 → 0, 503 → 1, недоступен → 1, таймаут → 1.

### 4.4 Таймаут запроса

- `middleware.Timeout(d time.Duration, onTimeout http.Handler) func(http.Handler) http.Handler`: `context.WithTimeout` на запрос. Если обработчик вернулся, ничего не записав, а `ctx.Err() == context.DeadlineExceeded`, вызывается `onTimeout`, который пишет `503` problem+json `code: timeout` (строка реестра есть с 1b). Если обработчик уже начал писать, ответ не трогается — урок объясняет почему.
- Обработчики и use case'ы уже уважают `ctx` (1a, урок 1.5); проверяется тестом: медленный обработчик, ждущий `ctx.Done()`, даёт 503 за ~`d`.
- Цепочка: `RequestID → AccessLog → Recover → Timeout → httpauth → RouteErrors(mux)`. Плеер `request-pipeline` (1.8) и уроки 1.5, 1.7, 1.8 обновляются; строка бэклога о таймауте закрывается.
- `http.Server`: `ReadHeaderTimeout` 5 с, `ReadTimeout` 15 с, `WriteTimeout` = таймаут запроса + 5 с, `IdleTimeout` 120 с. Что уже выставлено в 1a — проверяется в плане; значения — константы с комментарием «почему».

### 4.5 Конфиг

| Переменная | По умолчанию | Проверка |
|---|---|---|
| `MESSENGER_ADDR` | `127.0.0.1:8080` (в образе `:8080`) | как в 1b |
| `MESSENGER_JWT_KEY_FILE` | обязательна (в compose `/run/secrets/jwt_key`) | как в 1b |
| `MESSENGER_REQUEST_TIMEOUT` | `10s` | `time.ParseDuration`, > 0 |
| `MESSENGER_SHUTDOWN_TIMEOUT` | `10s` | > 0, меньше 30 с grace period Kubernetes — комментарий как в 1a |
| `MESSENGER_DRAIN_DELAY` | `5s` | ≥ 0, `DRAIN_DELAY + SHUTDOWN_TIMEOUT` < 30 с, иначе ошибка старта |

Неверный конфиг — выход с кодом 1 и сообщением с именем переменной. Тесты — табличные.

### 4.6 Dockerfile и compose

- `Dockerfile`:
  - `# syntax=docker/dockerfile:1` с закреплённой версией;
  - builder `golang:1.26.x-trixie@sha256:…` (версия Go — как в `go.mod`/CI), `FROM --platform=$BUILDPLATFORM`, `ARG TARGETOS TARGETARCH`;
  - `COPY go.mod go.sum` → `go mod download` с `--mount=type=cache,target=/go/pkg/mod`;
  - `COPY . .` → `CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /out/messenger ./cmd/messenger` с cache mount `/root/.cache/go-build`;
  - runtime `gcr.io/distroless/static-debian13:nonroot@sha256:…` (если на дату плана `debian13` нет — `debian12`, решение в плане), `COPY --from=build /out/messenger /messenger`, `ENV MESSENGER_ADDR=:8080`, `EXPOSE 8080`, `USER 65532:65532`, `HEALTHCHECK` (§4.3), `ENTRYPOINT ["/messenger"]`;
  - OCI-метки `org.opencontainers.image.{source,revision,version,created,licenses}` через `ARG`.
- `main.version` печатается в стартовом журнале и в `messenger --version`.
- `.dockerignore`: всё, кроме `go.mod`, `go.sum`, `cmd/`, `internal/`, `api/` (allowlist через `*` и `!`).
- `compose.yaml`: сервис `messenger` с `build`, `ports: 127.0.0.1:8080:8080`, `secrets: [jwt_key]` (файл из `cmd/keygen`), `read_only: true`, `tmpfs: [/tmp]`, `cap_drop: [ALL]`, `security_opt: [no-new-privileges:true]`, `mem_limit: 256m`, `cpus: 1`, `environment: GOMEMLIMIT=230MiB`, `stop_grace_period: 30s`, `healthcheck` берётся из образа.
- Размер образа, число пакетов и CVE у четырёх вариантов базового образа (урок 1.16) — измеряются исполнителем и вставляются реальным выводом; файлы вариантов — в `course/modules/01-go-service/examples/dockerfiles/`, проверяются сборкой в `check:messenger`.

### 4.7 Правила зависимостей

- `platform/health` не импортирует ничего из контекстов; контексты не импортируют `health` (проверки для 2a регистрирует composition root). Правило добавляется в `depguard`.
- `cmd/messenger/healthcheck.go` использует только стандартную библиотеку.

## 5. CI и доставка

### 5.1 Репозиторий

- Публичный репозиторий на GitHub. Создание и `gh auth login` выполняет ученик (интерактивный вход); урок 1.18 даёт шаги. Первый пуш — после слияния `feat/module-1c` в `main` и только с явного разрешения.
- Ruleset для `main`: обязательна проверка `ci-ok`, запрещены force-push и удаление, изменения только через PR (для владельца — разрешён обход, урок объясняет компромисс для одиночного проекта).
- Включаются: secret scanning, push protection, Dependabot alerts, CodeQL default setup (Go, JavaScript/TypeScript), Pages из Actions.

### 5.2 `ci.yml`

- События: `pull_request`, `push` в `main`, `workflow_dispatch`.
- На уровне workflow: `permissions: contents: read`; `concurrency: group: ci-${{ github.ref }}, cancel-in-progress: ${{ github.event_name == 'pull_request' }}`.
- Каждая джоба: `runs-on: ubuntu-24.04` (закреплённая метка, не `latest`), `timeout-minutes`.
- `changes` — `dorny/paths-filter` с фильтрами `messenger`, `course`, `labs`, `workflows`. На `push` в `main` все фильтры считаются истинными.
- Джобы (каждая `needs: changes` и `if:` по своему фильтру):
  - `messenger-lint` — `golangci-lint-action` с той же версией линтера, что локально;
  - `messenger-test` — `actions/setup-go` с `go-version-file: messenger/go.mod`, `go test -race -count=1 -coverprofile=… ./...`, отчёт покрытия артефактом;
  - `messenger-generate` — `go generate ./...` и `git diff --exit-code`; строка бэклога о нестрогой проверке свежести закрывается;
  - `messenger-vuln` — `govulncheck ./...` (закреплённая версия);
  - `course` — `setup-node` (версия из `.nvmrc` или `package.json` `engines`, решение в плане), `npm ci`, `npm test`, `npm run typecheck`, `check:go`, `check:messenger`, `npm run build`;
  - `labs` — `go test ./...` в `labs/`, сценарии 0b и 1c (`docker compose` на раннере, privileged);
  - `image` — `docker/setup-buildx-action`, `docker/build-push-action` с `push: false`, `load: true`, кэш `type=gha`; `trivy` с `exit-code: 1` на HIGH/CRITICAL без исправления (`ignore-unfixed`); smoke: `docker run` с ключом из `cmd/keygen`, ожидание `healthy`, `curl /readyz` → 200, `POST /v1/users` → 201;
  - `workflows` — `actionlint`;
  - `ci-ok` — `needs:` все джобы, `if: always()`; падает, если любая из `needs` в состоянии `failure` или `cancelled` (пропуск — успех). Только она — обязательная проверка.
- Все сторонние и официальные actions закреплены по полному SHA с комментарием `# vX.Y.Z`; версии — не моложе двух недель на дату плана.

### 5.3 `release.yml`

- События: `push` в `main` (после зелёного CI — через `workflow_run` по `ci.yml` с `conclusion == success` или повторным запуском ворот в самом release; решение в плане, предпочтительно `workflow_run`), `push` тегов `v*`.
- `permissions` джобы: `contents: read`, `packages: write`, `id-token: write`, `attestations: write`.
- `docker/login-action` в `ghcr.io` с `GITHUB_TOKEN`; `docker/metadata-action`: `type=sha`, `type=ref,event=branch` (`main`), `type=semver,pattern={{version}}`, `{{major}}.{{minor}}`, `latest` только на semver-тегах.
- Сборка `linux/amd64,linux/arm64`, `provenance: mode=max`, `sbom: true` в buildx; затем `actions/attest-build-provenance` с `push-to-registry: true` для digest. Урок показывает `gh attestation verify oci://ghcr.io/<owner>/messenger@sha256:… --owner <owner>` и что он проверяет.
- Пакет GHCR делается публичным (шаг урока).

### 5.4 `pages.yml`

- После успешного `ci.yml` на `main` (`workflow_run`): сборка VitePress с `base` = `/<repo>/`, `actions/upload-pages-artifact`, `actions/deploy-pages`; `permissions: pages: write, id-token: write`; `environment: github-pages`.
- `base` задаётся через переменную окружения в `config`, чтобы локальный `npm run dev` остался с `/`. Ссылки и ассеты курса проверяются на работу под подпутём (Playwright в §8).

### 5.5 `dependabot.yml`

- Экосистемы: `gomod` (`/messenger`, `/labs`), `npm` (`/course`), `github-actions` (`/`), `docker` (`/messenger`, `/labs/0b`, `/labs/1c`). Еженедельно, группы «минорные и патчи» на экосистему, мажорные — отдельными PR. `cooldown` (выдержка новых версий) — если поддерживается на дату плана; согласуется с правилом «не моложе двух недель».

## 6. Мини-интервью

### 6.1 Банк вопросов

- Файл `course/interview/module-01.yaml`, около 25 вопросов, покрывающих 1.1–1.19 (не меньше одного на урок, больше — на 1.8–1.14 и 1.17–1.18).
- Поля вопроса: `id`, `lesson` (путь урока), `topic`, `level` (`M+` | `S`), `question`, `answer` (эталон, Markdown, 5–15 предложений), `checklist` (3–6 пунктов «что должен услышать интервьюер»), `red_flags` (1–3), `follow_ups` (1–3, каждый с коротким ответом).
- Сквозная задача — отдельный документ в уроке 1.20: «Спроектируй API сервиса закладок» в 6 шагов (ресурсы и URI → ошибки → пагинация → идемпотентность → аутентификация → образ и CI), у каждого шага — что сказать, частые ошибки, вопрос интервьюера.
- Тест Vitest на банк: схема валидна, `id` уникальны, `lesson` указывает на существующий файл урока, у каждого урока 1.1–1.19 есть хотя бы один вопрос.

### 6.2 Компонент `InterviewSim`

- Режим «сессия»: 8 вопросов, выбранных случайно с учётом покрытия тем (не два вопроса одной темы подряд; сид отображается, сессию можно повторить по сиду). Таймер 3 минуты на вопрос (останавливается, не обрывает), поле черновика.
- «Показать эталон» раскрывает ответ, чеклист с флажками самооценки, красные флаги и фоллоу-апы.
- Итог: доля отмеченных пунктов по темам, список слабых тем со ссылками на уроки.
- Режим «все вопросы» — список с фильтром по уроку и уровню.
- Прогресс (последние результаты по вопросам) — в `localStorage` под ключом с версией, каждый доступ в try/catch; без хранилища всё работает, только без истории.
- Логика (выбор вопросов, подсчёт итога) — чистые функции с тестами; компонент — тонкий. Общие требования визуализаций (§7.3) действуют.
- Компонент принимает банк как параметр — интервью следующих модулей добавят только YAML.

## 7. Визуализации

Движок 1a/1b: `Scenario`/`Step`, `Simulation<S, A, F>`, `VizFrame`, `Player`.

### 7.1 Песочницы

1. **`layer-cache`** (1.16).
   - Модель: Dockerfile из 8–10 инструкций (наивный и правильный порядок), кэш слоёв с ключами.
   - Действия: «изменить файл исходника», «изменить `go.mod`», «изменить README», «переставить `COPY . .` выше», «включить/выключить cache mount», «включить/выключить `.dockerignore`», «пересобрать».
   - Вывод: какие слои взяты из кэша, какие пересобраны, оценочное время сборки, итоговый размер.
   - Тесты: правка исходника в правильном порядке не трогает слой зависимостей; в наивном — трогает; правка README без `.dockerignore` инвалидирует `COPY . .`, с ним — нет.
2. **`supply-chain`** (1.18).
   - Модель: workflow с тремя actions, злоумышленник подменяет тег одного из них.
   - Действия: «закрепить по тегу / по SHA», «permissions: write-all / минимальные», «секрет в env / нет», «подменить тег», «запустить».
   - Вывод: какая версия action выполнилась, что ей было доступно, что утекло.
   - Тесты: закрепление по SHA — выполняется исходный код; по тегу без минимальных прав — утечка токена с правом записи; минимальные права ограничивают ущерб при закреплении по тегу.

### 7.2 Пошаговые плееры

- **`container-start`** (1.15): `run` → `clone` с флагами → `child` → `pivot_root` → `mount proc` → `exec`; рядом два дерева процессов (хост и контейнер) и два дерева монтирования.
- **`cgroup-limits`** (1.15): процесс растёт по памяти до `memory.max` — reclaim, OOM-kill, `memory.events`; CPU-квота `cpu.max` — периоды и throttling.
- **`graceful-shutdown`** (1.17): SIGTERM → `Draining` → `/readyz` 503 → балансировщик убирает под → `DRAIN_DELAY` → `Shutdown` дожидается запросов → выход; вариант с shell-формой `ENTRYPOINT`, где SIGTERM не доходит и через grace period приходит SIGKILL.
- **`gomemlimit`** (1.17): куча и цель GC во времени при `memory.max` без `GOMEMLIMIT` (OOM-kill) и с ним (GC чаще, процесс жив).
- **`ci-graph`** (1.18): граф джоб, фильтр путей пропускает часть, `ci-ok` собирает результат; вариант без агрегирующей джобы, где PR висит в ожидании.

### 7.3 Общие требования

Как в 1a §4.3: подписи на русском с `termId`, чистые функции с тестами Vitest, `VizFrame` с полноэкранным режимом, палитра `--viz-*`, узкая раскладка для 375 px, `prefers-reduced-motion`.

## 8. Глоссарий

Новые термины (id без конфликтов с существующими; список сверяется с `terms.yaml` в плане):
- контейнеры: `container`, `namespace`, `cgroup`, `rootfs`, `pivot-root`, `overlayfs`, `oci`, `container-runtime`, `privileged-container`, `oom-kill`, `cpu-throttling`;
- образ: `container-image`, `image-layer`, `build-cache`, `multi-stage-build`, `distroless`, `static-binary`, `image-digest`, `image-registry`;
- запуск: `pid-1`, `graceful-shutdown`, `liveness-probe`, `readiness-probe`, `healthcheck`, `twelve-factor`, `request-timeout`, `gomemlimit`, `gomaxprocs`;
- CI/CD: `ci`, `cd`, `pipeline`, `ci-job`, `runner`, `required-check`, `monorepo`, `least-privilege`, `action-pinning`, `supply-chain-attack`, `sbom`, `provenance`, `slsa`, `attestation`, `dependency-update`;
- интервью: `technical-interview`.

Ни одного из этих id в `terms.yaml` пока нет (проверено 2026-10-15). Существующие `middleware`, `adr`, `request-id` и другие переиспользуются и при необходимости дополняются.

## 9. Попутно из бэклога

- Middleware таймаута запроса, обещанный в 1.5 и 1.7 (§4.4).
- `check:messenger`: нестрогая проверка свежести `go generate` становится строгой в CI на чистом checkout (§5.2); комментарий в скрипте.
- `check:messenger`: `generatedFiles` ищет `gen` по абсолютному пути — исправить на путь относительно `messenger/`.
- Esc в настоящем полноэкранном режиме браузера — по-прежнему проверяется только в режиме оверлея; ограничение остаётся записанным в бэклоге.

## 10. Вне рамок

- Kubernetes, манифесты, Helm, пробы в манифестах — модуль 13a (пробы и shutdown проектируются так, чтобы перенестись без изменений).
- Postgres и проверка готовности по базе — модуль 2a (`Checker` готов к ней).
- Метрики, трейсинг, профилирование в контейнере — модуль наблюдаемости.
- cosign с собственным ключом, политики допуска образов (Kyverno, admission) — упоминаются в 1.19.
- Деплой мессенджера куда-либо, кроме реестра; Pages — только для курса.
- Rootless-контейнеры и user namespace в лабе — упоминаются в 1.15 как следующий шаг.
- `act` и локальный прогон Actions — не используем; workflow проверяются `actionlint`, а работа — первым прогоном на GitHub.

## 11. Проверка готовности

- `npm test`, `npm run typecheck`, `npm run check:go`, `npm run check:labs` (включая сценарий 1c), `npm run check:messenger` (включая сборку образа и примеров Dockerfile), `npm run build` проходят.
- На каждом теге `m01-l15` … `m01-l20`: `go build ./...` и `go test -race ./...` в `messenger/` и `labs/` проходят; `golangci-lint` без замечаний.
- `docker build` образа мессенджера: размер меньше 25 МБ (точное значение — в уроке), пользователь 65532, `trivy` без HIGH/CRITICAL с исправлением.
- `docker compose up`: контейнер `healthy`; сквозной сценарий `curl` из урока 1.14 проходит против контейнера; реальный вывод в уроке 1.17.
- `docker compose stop`: процесс выходит меньше чем за `DRAIN_DELAY + SHUTDOWN_TIMEOUT`, в журнале — дренаж и «shutdown complete», код выхода 0.
- Лаба 1c: все пять экспериментов проходят `scenario.sh` в privileged-контейнере.
- `actionlint` по всем workflow без замечаний.
- После разрешённого пуша: `ci-ok` зелёная на `main`; образ `ghcr.io/<owner>/messenger` с тегами `main` и `sha-…` опубликован для двух архитектур; `gh attestation verify` проходит; курс открывается на Pages, навигация и ассеты работают под подпутём. Падения на настоящих раннерах чинятся отдельной задачей плана.
- Playwright на 375 и 1280 px, в тёмной и светлой теме, для уроков 1.15–1.20 (локально и на Pages): нет горизонтальной прокрутки, песочницы и симулятор реагируют, полноэкранный режим работает.
- Каждый урок проходит ревью на факты (man-страницы `namespaces(7)`, `cgroups(7)`, `pivot_root(2)`; документация ядра cgroup v2; OCI image и runtime spec; документация Docker/BuildKit и GitHub Actions; SLSA v1.0; Go 1.26 по исходникам) и подробность; внешний источник не нужен для понимания.
