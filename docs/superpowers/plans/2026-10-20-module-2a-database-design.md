# Модуль 2, часть 2a «Проектирование базы данных» — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Уроки 2.1–2.9 о проектировании базы данных, лаба `labs/2a` на Postgres 18.6, три песочницы и три плеера; мессенджер переходит с in-memory хранилищ на Postgres через `pgx`/`pgxpool` за теми же портами, с миграциями (goose), транзакциями на уровне use case (`Transactor`) и интеграционными тестами `storetest` на testcontainers.

**Architecture:**
- Слои и порты модулей 1 и 1.5 не меняются; use case'ы получают один новый узкий порт `Transactor` (в `auth/usecase` и `chat/usecase`). Домен не меняется.
- Новое: `platform/pgtx` (транзакция в `ctx`, `Querier`), `platform/pgtest` (testcontainers + snapshot/restore для тестов адаптеров), `*/adapters/postgres` для `user`, `auth`, `chat`, `platform/idempotency`, пакет `messenger/migrations` (встроенные SQL-миграции goose), подкоманда `messenger migrate`.
- In-memory адаптеры остаются тестовыми фейками и эталоном `storetest`; бинарь работает только с Postgres.
- Тег `m02-lNN` ставится на последний коммит задачи урока. Глоссарий, лаба, визуализации (задачи 1–4) входят в `m02-l01`; код мессенджера (задачи 8–11) — в `m02-l04`.

**Tech Stack:** курс — как в 1.5b (VitePress 1.6.4, Vue 3.5.43, TypeScript 5.9.3, Vitest 5.0.3, mermaid 12.1.0, Node 26); Go 1.27.2; **PostgreSQL 18.6** (образ `postgres:18.6-trixie`, digest сверяет задача 2); `github.com/jackc/pgx/v5` **v5.11.0** (как в обвязке `check:go`); `github.com/pressly/goose/v3` **v3.29.0** (2026-10-11 — моложе двух недель на 2026-10-20: задача 8 берёт v3.29.0 только если на дату исполнения ему ≥ 14 дней, иначе предыдущий выпуск); `github.com/testcontainers/testcontainers-go` и `modules/postgres` **v0.44.0** (2026-08-07).

**Spec:** `docs/superpowers/specs/2026-10-20-module-2a-database-design-design.md` — главный; спецификации 1a–1c, 1.5a/b и `2026-10-09-course-platform-design.md` (§1, §2.2–2.5) — действующие правила.

**Отступления от стандарта плана (как в 1.5):** Go-код — сигнатурами и тестами; уроки, рендереры, сцены, ADR — контрактом.

**Факты, проверенные при подготовке (2026-10-20):**
- PostgreSQL 18.6 вышел 2026-08-13; 19 — RC1 (2026-10-15). В 18 есть встроенная `uuidv7()`.
- goose: `goose.NewProvider(goose.DialectPostgres, *sql.DB, fs.FS, opts...)`; `*sql.DB` из `github.com/jackc/pgx/v5/stdlib` (`stdlib.OpenDB(*pgx.ConnConfig)` или `sql.Open("pgx", dsn)`).
- testcontainers-go `modules/postgres`: `Run(ctx, image, opts...)`, `Snapshot(ctx, opts...)`, `Restore(ctx, opts...)`; снимок требует базу не `postgres`; `postgres.WithSQLDriver("pgx")` ускоряет снимки.
- `DEFERRABLE` бывает только у `UNIQUE`, `PRIMARY KEY`, `FOREIGN KEY`, `EXCLUDE`. SQLSTATE: 23505 unique_violation, 23503 foreign_key_violation, 23514 check_violation, 23P01 exclusion_violation.
- `pgxpool`: `MaxConns` по умолчанию `max(4, runtime.NumCPU())` (v5.11.0).
- В мессенджере: `user/domain.NewUsername` допускает только `a–z0–9_` (имя уже в нижнем регистре — `citext`/коллация не нужны; урок 2.3 объясняет альтернативы); ошибки `user/domain.ErrUsernameTaken`, `auth/domain.ErrCredentialsExist`, `chat/domain.{ErrChatNotFound,ErrConcurrentUpdate,ErrDirectChatExists,ErrDuplicateMessage,ErrClientMessageIDReused}`; порты — `*/usecase/ports.go`; батареи `storetest.Run` в `user`, `auth`, `chat`, `platform/idempotency` (+ `RunSweeper`); `idempotency.Store.Reserve(ctx, scope, key, rec, now) (Record, bool, error)`, `Complete`, `Release`; `health.New(log, checks ...Check)` с портом `health.Checker`; подкоманды в `cmd/messenger/cli.go` (`version`, `healthcheck`); стадии `lifecycle.Run(ctx, log, Stages{Front, Back})`.

**Решения плана, уточняющие спеку (рулинги до исполнения):**
1. **Схема таблиц** (миграция `00001_init.sql`; точные длины — константы домена):
   - `users.users(id uuid PK, username text NOT NULL UNIQUE CHECK (username ~ '^[a-z0-9_]+$' AND length BETWEEN …), display_name text NOT NULL CHECK (length …), created_at timestamptz NOT NULL)`;
   - `auth.credentials(user_id uuid PK REFERENCES users.users(id) ON DELETE CASCADE, password_hash text NOT NULL, created_at timestamptz NOT NULL)`;
   - `chat.chats(id uuid PK, kind text NOT NULL CHECK (kind IN ('direct','group')), title text, last_seq bigint NOT NULL DEFAULT 0 CHECK (last_seq >= 0), version bigint NOT NULL CHECK (version >= 0), created_at timestamptz NOT NULL, last_activity_at timestamptz NOT NULL, direct_key text UNIQUE)` — `direct_key` = отсортированная пара id для личного чата (NULL у групп), `CHECK ((kind = 'direct') = (direct_key IS NOT NULL))`;
   - `chat.members(chat_id uuid REFERENCES chat.chats ON DELETE CASCADE, user_id uuid REFERENCES users.users(id), PRIMARY KEY (chat_id, user_id))` + индекс `(user_id)`;
   - `chat.messages(chat_id uuid REFERENCES chat.chats ON DELETE CASCADE, seq bigint CHECK (seq > 0), id uuid NOT NULL UNIQUE, author_id uuid NOT NULL, client_message_id text NOT NULL, text text NOT NULL CHECK (…), sent_at timestamptz NOT NULL, PRIMARY KEY (chat_id, seq), UNIQUE (chat_id, author_id, client_message_id))`;
   - индекс списка чатов: `chat.chats (last_activity_at DESC, id DESC)` используется через `members` — точный индекс выбирает задача 10 по `EXPLAIN` (урок 2.5 цитирует);
   - `platform.idempotency_keys(scope text, key text, state text CHECK (state IN ('in_progress','done')), owner …, response…, reserved_until timestamptz NOT NULL, expires_at timestamptz NOT NULL, PRIMARY KEY (scope, key))` + индекс `(expires_at)`; поля ответа — по `idempotency.Record`.
2. **Захват зависшей резервации** (бэклог «409 на 24 ч»): запись `in_progress` хранит `reserved_until = now + reservationTTL` (`reservationTTL` — константа идемпотентности, по умолчанию `RequestTimeout + 5s`, передаётся в конструктор store'ов); `Reserve` поверх `in_progress` с `reserved_until <= now` перехватывает её (возвращает `true`), иначе — как сейчас. Одна семантика для memory и Postgres; новый тест в `storetest.Run`: `TestReserveTakesOverStaleInProgress` и `TestReserveKeepsFreshInProgress`. Гонка двух перехватчиков — атомарным `UPDATE … WHERE reserved_until <= $now RETURNING` (Postgres) и замком (memory).
3. **`Transactor`** объявлен в `auth/usecase` и `chat/usecase` (узкий: `WithinTx(ctx context.Context, fn func(ctx context.Context) error) error`). Используют: `SignUp.Execute` (регистрация профиля + сохранение учётных данных), `SendMessage` (append), `CreateChat` (чат + участники, если сейчас это два шага). Вложенный `WithinTx` переиспользует внешнюю транзакцию (без SAVEPOINT; комментарий «почему»).
4. **Изоляция тестов адаптеров** — `pgtest`: один контейнер на пакет, миграции → `Snapshot`, перед каждым тестом `Restore`; если `Restore` дольше 300 мс на тест — `TRUNCATE` всех таблиц (решение и замер в отчёте задачи 8).
5. **Пул**: `MESSENGER_DB_MAX_CONNS`, по умолчанию 10 (обоснование законом Литтла в уроке 2.9: RPS × время удержания соединения из профиля 1.5.10, с запасом; `NumCPU` не используется — квота контейнера). `MinConns` 0, `MaxConnLifetime` 1h, `HealthCheckPeriod` 1m (умолчания pgxpool, названы константами).
6. **Порядок остановки**: пул закрывается `defer` в `run` после `lifecycle.Run` (после фоновых компонентов). `/readyz` — проверка `pool.Ping` с таймаутом `dbPingTimeout = 1 * time.Second`; пробы переносятся под `middleware.Recover` (бэклог 1c).
7. **Миграции**: `messenger/migrations/{embed.go, 00001_init.sql}`; подкоманда `messenger migrate up|status` открывает `*sql.DB` через `pgx/v5/stdlib` с `MaxOpenConns(1)`; в compose — сервис `migrate` (тот же образ, `command: ["migrate","up"]`), `messenger` зависит от него с `condition: service_completed_successfully`; Postgres — сервис `db` с томом и healthcheck `pg_isready`.
8. **Секрет строки подключения**: `MESSENGER_DATABASE_URL_FILE` → файл `db_url` через compose `secrets` (как `jwt_key`); пароль базы — секрет `db_password` для образа Postgres (`POSTGRES_PASSWORD_FILE`).
9. **CI**: новая джоба `messenger-integration` (`go test -race -run . ./...` с `MESSENGER_REQUIRE_DOCKER=1`), входит в `ci-ok`; обычная `messenger-test` без Docker пропускает интеграционные тесты (сообщение `t.Skip`).
10. **Числа в уроках** снимает исполнитель; текстовые выводы — в `labs/2a/results/` (стандарт 1.5b).

## Global Constraints

- Команды npm — из `course/`, Go мессенджера — из `messenger/`, лаб — из `labs/`. Go 1.27.2: `export PATH=/home/idel/sdk/go1.27.2/bin:$PATH`. `golangci-lint` — в `~/go/bin`.
- Ветка `feat/module-2a`. `main` защищён ruleset (PR + `ci-ok`). `git push` — только с согласия пользователя.
- Postgres — `pgx`/`pgxpool`. `database/sql` в коде — только в подкоманде `migrate` и `platform/pgtest` (через `pgx/v5/stdlib`); в уроках — только как контраст.
- Язык контента — русский; термин при первом появлении — `<T id="…">`; английский оригинал в скобках.
- Уроки — по основной спецификации §1, §2.2 (10 правил), §2.3, §2.4; структура как в 1.5b: «Проблема» → теория → «Визуализация» → «Код» → «Что изменилось в мессенджере» (2.4, 2.5, 2.8, 2.9) или «Что появилось в лабе» → «Типичные ошибки» → «Компромисс» → «Самопроверка» (5–8) → «Источники». Заголовок `title: "2.N. …"`, папка `course/modules/02-database/`, `index.md` модуля с `order: 2`.
- Визуализации: чистые TS без `vue`/DOM; DOM в `onMounted`/обработчиках; слушатели снимаются; цвета — `--viz-*`; обе темы, контраст ≥ 4,5:1, `prefers-reduced-motion`, 375 px без прокрутки, `VizFrame`, `SegmentedToggle`, подписи с `termId`; клавиатура не перехватывает сочетания с Alt/Ctrl/Meta.
- Go-код — по `~/.claude/CLAUDE.md` и §2.5: порт у потребителя, узкие интерфейсы; `NewX(deps…) (*X, error)`; `ctx` первым; `%w`; сообщения по-русски со строчной; у горутины владелец; константы с «почему»; SQL — параметризованный, никакой склейки строк с данными; ошибки ограничений — по `pgconn.PgError.Code` и `ConstraintName`, не по тексту.
- Новые тесты с временем — `synctest`; без `time.Sleep`. Интеграционные тесты не используют `synctest` (сеть к контейнеру реальная).
- Код в уроках — `<<< @/../<путь>#<регион>{lang}`; SQL-фрагменты — из файлов миграций/лабы регионами (`-- #region name` / `-- #endregion`), иллюстративные Go-фрагменты — `<!-- go-check -->` (обвязка `pgx-pool` уже есть).
- Вывод команд — реальный, с машиной; числа воспроизводимы из `labs/2a` или команд урока; гипотезы помечены.
- `labs/2a/scenario.sh`: `LC_ALL=C.UTF-8`, без кириллических диапазонов в регулярках, относительные пороги, ≤ 2 мин на раннере, `trap` убирает контейнеры.
- Коммиты на русском, последняя строка `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Факты — по документации PostgreSQL 18, исходникам pgx v5.11.0, goose, testcontainers-go; книги: Hernandez, Karwin, Fontaine.

## Review Focus

1. **Два одновременных `SendMessage` в один чат.** Ожидается: оба сохранены с `seq` 1 и 2 без пропуска или второй получает `ErrConcurrentUpdate` (по контракту порта), никогда — два сообщения с одним `seq`. Покрывает `TestAppendConcurrentWriters` в `chat/usecase/storetest` (задача 10).
2. **Сбой посередине регистрации** (учётные данные не сохранились). Ожидается: пользователь тоже не сохранён, повторная регистрация с тем же именем проходит. Покрывает `TestSignUpRollsBackProfileOnCredentialsFailure` (задача 9, на Postgres).
3. **Зависшая резервация идемпотентности** (процесс упал между `Reserve` и `Complete`). Ожидается: после `reservationTTL` повтор проходит, а не 409 сутки. Покрывают `TestReserveTakesOverStaleInProgress`/`TestReserveKeepsFreshInProgress` (задача 11).
4. **База недоступна при старте или во время работы.** Ожидается: старт — ошибка с понятным текстом и код 1 (не паника); во время работы — `/readyz` 503 с причиной, запросы — 503, процесс жив. Покрывают `TestReadyzFailsWhenDBDown` и `TestNewAppFailsWithoutDB` (задача 11).
5. **Миграции применены не полностью или база новее бинаря.** Ожидается: `messenger migrate status` показывает расхождение; сервер при старте проверяет, что версия схемы ≥ требуемой бинарю, иначе выход с кодом 1 и сообщением «примените миграции». Покрывает `TestStartRefusesOutdatedSchema` (задача 8).

---

## Структура файлов

```
messenger/
├── migrations/{embed.go,00001_init.sql}                         # задача 8
├── internal/platform/pgtx/{doc.go,pgtx.go,pgtx_test.go}         # задача 9
├── internal/platform/pgtest/{doc.go,pgtest.go}                  # задача 8
├── internal/{user,auth,chat}/adapters/postgres/{doc.go,store.go,errors.go,store_test.go}   # задача 10
├── internal/platform/idempotency/postgres/{doc.go,store.go,store_test.go}                 # задача 11
├── internal/{auth,chat}/usecase/{ports.go,*.go,*_test.go}       # задача 9 (Transactor)
├── internal/platform/idempotency/{idempotency.go,memory/store.go,storetest/storetest.go}  # задача 11
├── cmd/messenger/{app.go,main.go,cli.go,migrate.go,*_test.go}   # задачи 8, 11
├── compose.yaml, .golangci.yml, go.mod                          # задачи 8, 11
└── docs/adr/{0014-схема-и-контексты,0015-транзакции,0016-миграции}.md   # задачи 12, 17, 16
labs/2a/{compose.yaml,scenario.sh,README.md,anomalies.sql,constraints.sql,antipatterns.sql,keys/,jsonb/,results/}   # задача 2
course/
├── glossary/terms.yaml                                          # задача 1
├── modules/02-database/{index.md,01-…09-*.md}                   # задачи 1, 5–7, 12–17
├── viz/simulations/{normal-forms,key-locality,access-patterns,er-build,constraint-race,migration-run}.ts (+ .test.ts)
├── viz/renderers/*View.vue, viz/scenes/*.vue                    # задачи 3–4
└── scripts/check-labs.ts                                        # задача 2
.github/{workflows/ci.yml,dependabot.yml}                        # задача 11
```

---

### Task 1: Глоссарий и страница модуля 2

**Files:** Modify: `course/glossary/terms.yaml`; Create: `course/modules/02-database/index.md`; Modify (если нужно): `course/components/lesson-kicker.ts` (+ тест)

- [ ] **Step 1:** Термины спеки §6, которых нет (проверить `grep -n "^- id:"`; существующие — переиспользовать): определение 1–3 предложения с примером из мессенджера, ссылки `[текст](#id)` в `long` на связанные.
- [ ] **Step 2:** `index.md`: `order: 2`, вступление о модуле 2 (части 2a и 2b), список уроков 2.1–2.9 текстом без ссылок (каждый урок добавит свою ссылку).
- [ ] **Step 3:** `lesson-kicker` даёт «Модуль 2 · Урок N» для `02-database/NN-…` — тест `['modules/02-database/03-keys-types.md', 'Модуль 2 · Урок 3']`; если уже работает — только тест.
- [ ] **Step 4: Проверить** — `npm test && npm run check:refs && npm run build`; Expected: PASS.
- [ ] **Step 5: Commit** — `docs(course): глоссарий и страница модуля 2`.

---

### Task 2: Лаба `labs/2a`

**Files:** Create: `labs/2a/{compose.yaml,scenario.sh,README.md,anomalies.sql,constraints.sql,antipatterns.sql,keys/main.go,keys/main_test.go,jsonb/*.sql,results/}`; Modify: `course/scripts/check-labs.ts`, `.github/workflows/ci.yml` (джоба `labs` — путь `labs/2a`)

**Interfaces:** Produces: `labs/2a/results/*.txt` (реальные выводы с машиной) для уроков 2.2–2.7.

- [ ] **Step 1: compose** — `postgres:18.6-trixie@sha256:<digest>` (digest — `docker buildx imagetools inspect`, записать в отчёт), порт только на `127.0.0.1`, `tmpfs` для данных (лаба одноразовая), healthcheck `pg_isready`.
- [ ] **Step 2: SQL-лабы** — `anomalies.sql` (ненормализованная таблица «сообщения с данными автора и чата»: аномалии вставки/обновления/удаления, затем 3НФ), `constraints.sql` (нарушение каждого вида ограничения с SQLSTATE: 23502, 23505, 23503, 23514, 23P01; отложенный FK), `antipatterns.sql` (EAV, полиморфная связь, список через запятую, наивное дерево — запрос, который ломается, и исправление), `jsonb/` (настройки пользователя в JSONB против столбцов: запросы, индекс GIN — кратко, подробно в 2b). Регионы `-- #region …` для уроков.
- [ ] **Step 3: `keys/`** — Go-программа на `pgx`: таблицы с PK `bigint generated always as identity`, `uuid` v4 (`gen_random_uuid()`), `uuid` v7 (`uuidv7()`), N вставок пачками; печать размера индекса PK (`pg_relation_size`), WAL (`pg_wal_lsn_diff` до/после), времени, доли листовых страниц после `pgstattuple`/`pageinspect` (если расширение есть в образе — иначе только размер). `-n` по умолчанию 10⁶; тест на разбор вывода.
- [ ] **Step 4: `scenario.sh`** — поднять compose, прогнать SQL-лабы с проверкой ключевых строк (`ERROR:  duplicate key`, SQLSTATE), `keys -n 100000` с относительной проверкой «индекс v4 больше индекса v7», `trap` с `docker compose down -v`; `check-labs.ts` подключает; время ≤ 2 мин на 2 ядрах (`taskset -c 0,1`).
- [ ] **Step 5: Полные прогоны** — `keys -n 1000000` (3 повтора), все SQL-лабы; выводы с машиной — в `labs/2a/results/`.
- [ ] **Step 6: Проверить** — из `labs/`: `gofmt -l . && go vet ./... && go test -race ./... && LC_ALL=C bash 2a/scenario.sh`; `cd ../course && npm run check:labs`; Expected: PASS.
- [ ] **Step 7: Commit** — `feat(labs): лаба 2a — нормализация, ключи, ограничения, антипаттерны`.

---

### Task 3: Песочницы `normal-forms`, `key-locality`, `access-patterns`

**Files:** Create: `course/viz/simulations/{normal-forms,key-locality,access-patterns}.ts` (+ `.test.ts`), renderers и scenes.

- [ ] **Step 1: `normal-forms`** — модель: таблица строк (ненормализованная «сообщение+автор+чат» и её 3НФ-разложение); действия «вставить», «переименовать автора», «удалить последнее сообщение автора»; вывод: какие строки изменились, где возникла аномалия (несогласованность, потеря факта, невозможность вставки). Тесты: переименование в ненормализованной меняет N строк и при частичном обновлении даёт противоречие; в 3НФ — одна строка; удаление последнего сообщения теряет автора только в ненормализованной; вставка автора без сообщения невозможна только в ненормализованной.
- [ ] **Step 2: `key-locality`** — модель листьев B-tree (ёмкость страницы K, правило разделения 50/50 и «правое» разделение для монотонных ключей, как в Postgres — `fillfactor` и `rightmost split` сверить в документации nbtree); генераторы ключей bigint, v4 (случайные), v7 (монотонные с шумом в пределах миллисекунды); вывод: число страниц, средняя заполненность, число разделений, «горячие» страницы. Тесты: монотонные ключи дают заполненность ≈ fillfactor и разделения только справа; случайные — заполненность ≈ 0,5–0,75 и разделения по всему дереву; при одинаковом N страниц у v4 больше, чем у v7. В подписи — ссылка на реальные числа лабы.
- [ ] **Step 3: `access-patterns`** — каталог запросов мессенджера (из урока 2.5) × варианты схемы/ключей; вывод: каким ключом/индексом обслуживается, сколько строк читается в модели (диапазон против полного прохода), что будет без индекса. Тесты: лента чата по PK `(chat_id, seq)` — диапазон; поиск сообщения по `id` — через `UNIQUE(id)`; список чатов участника без индекса по `members(user_id)` — полный проход.
- [ ] **Step 4: Проверить** — `npm test && npm run typecheck`; Expected: PASS.
- [ ] **Step 5: Commit** — `feat(viz): песочницы normal-forms, key-locality, access-patterns`.

---

### Task 4: Плееры `er-build`, `constraint-race`, `migration-run`

**Files:** Create: `course/viz/simulations/{er-build,constraint-race,migration-run}.ts` (+ тесты), renderers, scenes.

- [ ] **Step 1: `er-build`** — шаги: требование → сущность/связь появляется на ER-диаграмме (пользователь, учётные данные 1:1, чат, участники M:N, сообщения 1:N, ключи идемпотентности) → граница контекста (схема). Тест: на последнем шаге 6 сущностей и связи с верной кардинальностью.
- [ ] **Step 2: `constraint-race`** — два экземпляра сервиса регистрируют одно имя: вариант «проверка в коде» (оба SELECT → оба INSERT → два пользователя), вариант «`UNIQUE`» (второй INSERT → 23505 → `ErrUsernameTaken` → 409). Тесты: в первом варианте в финале два пользователя с одним именем; во втором — один и ошибка 23505.
- [ ] **Step 3: `migration-run`** — `migrate up`: таблица версий goose, транзакционная миграция, сбой посередине → откат DDL, повтор; запуск сервера при неприменённой миграции → отказ старта. Тест: после сбоя версия не меняется и таблиц из миграции нет.
- [ ] **Step 4: Проверить** — `npm test && npm run typecheck`; Expected: PASS.
- [ ] **Step 5: Commit** — `feat(viz): плееры er-build, constraint-race, migration-run`.

---

### Task 5: Урок 2.1 «Модель данных» (тег `m02-l01`)

**Files:** Create: `course/modules/02-database/01-modeling.md`; Modify: `index.md` (ссылка)

- [ ] **Step 1: Урок (контракт)** — строка 2.1 спеки. Обязательно: требования мессенджера из 0.6 → сущности и связи; нотации ER; модель мессенджера mermaid ER (классы палитры); границы контекстов → схемы Postgres и почему межконтекстные FK только к `users.users` (компромисс — ADR-0014 появится в задаче 12, здесь ссылка текстом); сцена `ErBuild`. Типичные ошибки: модель «от экранов», а не от требований; M:N без связующей таблицы; смешение контекстов в одной таблице; «у каждой таблицы id и больше ничего».
- [ ] **Step 2: Проверить** — `npm run build && npm run check:refs && npm run check:go && npm test`; Expected: PASS.
- [ ] **Step 3: Commit и тег** — `docs(course): урок 2.1 — модель данных`; `git tag m02-l01`.

### Task 6: Урок 2.2 «Нормализация и денормализация» (тег `m02-l02`)

**Files:** Create: `02-normalization.md`; Modify: `index.md`

- [ ] **Step 1: Урок (контракт)** — строка 2.2 спеки: аномалии на реальном выводе `labs/2a/results` (anomalies), функциональные зависимости, 1НФ–НФБК с примером, где 3НФ ≠ НФБК, денормализация `last_seq`/`last_activity_at` — кто поддерживает инвариант (транзакция отправки сообщения, задача 9); сцена `NormalFormsSandbox`. Типичные ошибки: «нормализовать до 5НФ всегда», денормализация без владельца инварианта, JSON-массив вместо связи.
- [ ] **Step 2: Проверить; Step 3: Commit и тег** `m02-l02`.

### Task 7: Урок 2.3 «Ключи и типы» (тег `m02-l03`)

**Files:** Create: `03-keys-types.md`; Modify: `index.md`

- [ ] **Step 1: Урок (контракт)** — строка 2.3 спеки: числа `keys` из `labs/2a/results` (размер индекса и WAL на 10⁶ — с машиной), почему UUIDv7 из приложения, `uuidv7()` в 18, утечка времени в v7 и когда это важно; типы (`timestamptz`, `text`+`CHECK`, `numeric`, `bigint`, enum против справочника); имя пользователя: домен уже нормализует к нижнему регистру — поэтому простой `UNIQUE`; альтернативы `citext`/`lower()`-индекс/недетерминированная ICU-коллация (что поддерживает 18 — по release notes, с цитатой); сцена `KeyLocalitySandbox`. Типичные ошибки: `timestamp` без зоны, `varchar(255)` «на всякий случай», float для денег, UUIDv4 как PK большой таблицы без понимания цены, `serial` вместо `identity`.
- [ ] **Step 2: Проверить; Step 3: Commit и тег** `m02-l03`.

---

### Task 8: Мессенджер — миграции, `migrate`, compose с Postgres, `pgtest`

**Files:** Create: `messenger/migrations/{embed.go,00001_init.sql}`, `messenger/cmd/messenger/migrate.go` (+ тест), `messenger/internal/platform/pgtest/{doc.go,pgtest.go}`; Modify: `cmd/messenger/cli.go`, `compose.yaml`, `go.mod`, `README.md` (корневой), `.gitignore` (секреты)

**Interfaces:**
```go
// messenger/migrations
//go:embed *.sql
var FS embed.FS
const Required int64 = 1 // последняя миграция, без которой бинарь не стартует

// cmd/messenger (migrate.go)
func runMigrate(ctx context.Context, args []string, stdout, stderr io.Writer) int // up | status
func schemaVersion(ctx context.Context, pool *pgxpool.Pool) (int64, error)        // для проверки при старте сервера

// platform/pgtest
type DB struct{ /* container, dsn */ }
func Start(m *testing.M) (code int)          // для TestMain: контейнер, миграции, Snapshot; без Docker — помечает «пропуск»
func Pool(t *testing.T) *pgxpool.Pool         // Restore + новый пул на тест; t.Skip, если Docker нет и MESSENGER_REQUIRE_DOCKER != "1" (тогда t.Fatal)
```
- [ ] **Step 1: Миграция** `00001_init.sql` — схема по рулингу 1 (goose-аннотации `-- +goose Up/Down`, регионы для уроков 2.4, 2.5, 2.8). Имена ограничений — явные (`users_username_key`, …), константы с этими именами появятся в адаптерах.
- [ ] **Step 2: Тесты `migrate`** (на `pgtest`): `TestMigrateUpFromScratch` (status после up — все применены), `TestMigrateIdempotent` (второй up — без изменений), `TestStartRefusesOutdatedSchema` (версия < `Required` → ошибка с текстом «примените миграции»).
- [ ] **Step 3: Реализация** — `migrate up|status` (`goose.NewProvider`, `*sql.DB` через `stdlib` на одно соединение, DSN из `MESSENGER_DATABASE_URL_FILE`); `cli.go` вызывает; `pgtest` (testcontainers `postgres.Run` с образом из compose, `WithDatabase("messenger")`, `WithSQLDriver("pgx")`, применение миграций, `Snapshot`; `Restore` в `Pool`). Замер: время `Restore` на тест — в отчёт (рулинг 4).
- [ ] **Step 4: compose** — сервисы `db` (Postgres 18.6, том `db-data`, `POSTGRES_PASSWORD_FILE`, healthcheck), `migrate` (образ мессенджера, `command: ["migrate","up"]`, зависит от `db: service_healthy`), `messenger` (зависит от `migrate: service_completed_successfully`); секреты `db_password`, `db_url`; `mem_limit` для `db` с комментарием; README — как поднять, как создать секреты.
- [ ] **Step 5: Проверить** — `go test -race ./...` (с Docker), `golangci-lint run`, `docker compose up -d --build` → `docker compose logs migrate` (применено), `docker compose ps` (messenger healthy), `docker compose down`; Expected: PASS.
- [ ] **Step 6: Commit** — `feat(messenger): миграции goose, подкоманда migrate, Postgres в compose`.

---

### Task 9: Мессенджер — `pgtx` и порт `Transactor`

**Files:** Create: `internal/platform/pgtx/{doc.go,pgtx.go,pgtx_test.go}`; Modify: `internal/{auth,chat}/usecase/{ports.go,…}`, их тесты, `*/adapters/memory` (фейковый `Transactor` — в тестовом пакете или `memory.Tx`)

**Interfaces:**
```go
// auth/usecase и chat/usecase (каждый объявляет свой)
type Transactor interface {
	// WithinTx выполняет fn в одной транзакции: ошибка fn — откат. fn не запускает
	// горутины с переданным ctx: транзакция pgx не для одновременного использования.
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
// platform/pgtx
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
type Transactor struct{ /* pool */ }
func New(pool *pgxpool.Pool) (*Transactor, error)
func (t *Transactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
func Q(ctx context.Context, pool *pgxpool.Pool) Querier // транзакция из ctx или пул
```
- [ ] **Step 1: Тесты `pgtx`** (`pgtest`): `TestWithinTxCommits`, `TestWithinTxRollsBackOnError`, `TestWithinTxRollsBackOnPanic` (паника пробрасывается дальше после отката), `TestNestedWithinTxReusesOuter`, `TestQOutsideTxUsesPool`.
- [ ] **Step 2: Реализация `pgtx`.**
- [ ] **Step 3: Use case'ы** — `SignUp`, `SendMessage`, `CreateChat` (если двухшаговый) получают `Transactor` в конструкторе (nil — ошибка); тесты use case'ов с фейком; `TestSignUpRollsBackProfileOnCredentialsFailure` — интеграционный в `cmd/messenger` или `auth/adapters/postgres` (после задачи 10; здесь — фейк проверяет, что оба шага внутри `WithinTx`).
- [ ] **Step 4: Проверить** — `go test -race ./...`, `golangci-lint run`; Expected: PASS.
- [ ] **Step 5: Commit** — `feat(messenger): транзакции на уровне use case — порт Transactor и platform/pgtx`.

---

### Task 10: Мессенджер — Postgres-адаптеры `user`, `auth`, `chat`

**Files:** Create: `internal/{user,auth,chat}/adapters/postgres/{doc.go,store.go,errors.go,store_test.go}`; Modify: `internal/chat/usecase/storetest/storetest.go` (новые тесты), `internal/auth/usecase/storetest` при необходимости.

- [ ] **Step 1: Тесты** — каждый `store_test.go`: `TestMain` → `pgtest.Start`; `storetest.Run(t, func(t) Store { return postgres.New(pgtest.Pool(t)) })`. Новые тесты в батарее чата (проходят и memory): `TestAppendConcurrentWriters` (две горутины append с одной версией → одна успешна, вторая `ErrConcurrentUpdate`; `seq` без дублей), `TestListChatsByMemberCursorTies` (одинаковое `last_activity_at` на границе страницы). Тест отката на каждую пишущую операцию: `WithinTx` + ошибка → строки нет.
- [ ] **Step 2: Реализация** — SQL параметризованный, через `pgtx.Q(ctx, pool)`; маппинг строк ↔ домен через конструкторы домена (никаких тегов на доменных структурах); ошибки: 23505 по `ConstraintName` → `ErrUsernameTaken` / `ErrCredentialsExist` / `ErrDirectChatExists` / `ErrDuplicateMessage` (затем сверка тела для `ErrClientMessageIDReused` — по контракту порта); `pgx.ErrNoRows` → `ErrUserNotFound`/`ErrChatNotFound`/`ErrNoCredentials`; append — `UPDATE … WHERE id=$1 AND version=$2 RETURNING last_seq` (0 строк → `ErrConcurrentUpdate`) + `INSERT` в одной транзакции (`WithinTx` use case'а; адаптер не открывает свою). `MissingUsers` — один запрос `unnest($1::uuid[]) EXCEPT`. Индекс списка чатов — по `EXPLAIN` (вывод — в отчёт и в `labs/2a/results/messenger-explain.txt` для урока 2.5).
- [ ] **Step 3: Проверить** — `go test -race ./...` (с Docker), `-count=5` для `chat/adapters/postgres`, `golangci-lint run`; Expected: PASS.
- [ ] **Step 4: Commit** — `feat(messenger): Postgres-адаптеры пользователей, учётных данных и чатов`.

---

### Task 11: Мессенджер — идемпотентность в Postgres, пул, `/readyz`, сборка, CI

**Files:** Create: `internal/platform/idempotency/postgres/*`; Modify: `idempotency.go`, `memory/store.go`, `storetest/storetest.go`, `cmd/messenger/{app.go,main.go,app_test.go}`, `internal/platform/health/*`, `.golangci.yml`, `.github/workflows/ci.yml`, `.github/dependabot.yml`, `compose.yaml`

- [ ] **Step 1: Захват зависшей резервации** (рулинг 2): тесты в `storetest.Run` → проходят memory; затем Postgres-store (`Reserve` — `INSERT … ON CONFLICT DO NOTHING RETURNING`, иначе `UPDATE … WHERE reserved_until <= $now … RETURNING`, иначе чтение; `Sweep` — `DELETE … WHERE ctid IN (SELECT ctid … WHERE expires_at <= $1 LIMIT $2)` порциями `sweepBatch`).
- [ ] **Step 2: Сборка** — `app.go`: конфиг `MESSENGER_DATABASE_URL_FILE` (обязателен), `MESSENGER_DB_MAX_CONNS` (по умолчанию 10, 1–100); `pgxpool.NewWithConfig`, `Ping` при старте с `dbStartTimeout`; проверка версии схемы (`schemaVersion` ≥ `migrations.Required`); адаптеры Postgres вместо memory; `pgtx.New`; health-проверка `db` (`Ping` с `dbPingTimeout`); пробы под `Recover`; `main.run` — `defer pool.Close()` после `lifecycle.Run`. Тесты: `TestLoadDBConfig` (табличный), `TestNewAppFailsWithoutDB`, `TestReadyzFailsWhenDBDown` (контейнер `pgtest` останавливается — или пул на закрытый порт), `TestShutdownClosesPoolAfterBack` (порядок в журнале).
- [ ] **Step 3: depguard** — правила спеки §3.6; «красная» проверка: временный файл с импортом `pgx` в `chat/domain` → `golangci-lint` сообщает нарушение (вывод — в отчёт), файл удалён.
- [ ] **Step 4: CI** — джоба `messenger-integration` (рулинг 9) в `ci-ok`; фильтр путей; Dependabot — `gomod` для `course/scripts/go-harness` (+ `tools.go` с `//go:build tools`, импортирующий `pgxpool`, чтобы `go mod tidy` не убирал зависимость; комментарий «не запускать tidy» убрать), `docker` для `labs/2a/compose.yaml` при необходимости; `actionlint`.
- [ ] **Step 5: Сквозной прогон** — `docker compose up -d --build`; `scripts/smoke.sh` (регистрация, вход, чат, сообщение, лента) — зелёный; `docker compose restart messenger` → данные на месте; `docker compose stop` — порядок строк журнала (HTTP → фоновые → «пул закрыт»), код 0; вывод — в `labs/2a/results/messenger-smoke.txt`.
- [ ] **Step 6: Проверить** — `go test -race ./...`, `golangci-lint run`, `cd ../course && npm run check:messenger && npm run check:workflows`; Expected: PASS.
- [ ] **Step 7: Commit** — `feat(messenger): Postgres вместо памяти — идемпотентность, пул, пробы, CI`.

---

### Task 12: Урок 2.4 «Ограничения» и ADR-0014 (тег `m02-l04`)

**Files:** Create: `04-constraints.md`, `messenger/docs/adr/0014-схема-и-контексты.md`; Modify: `index.md`

- [ ] **Step 1: Урок (контракт)** — строка 2.4 спеки; выводы `constraints.sql` из `results`; регион миграции с ограничениями; регион `errors.go` адаптера (перевод по коду и имени ограничения); сцена `ConstraintRace`; отложенные ограничения — только UNIQUE/PK/FK/EXCLUDE. Типичные ошибки: проверка уникальности `SELECT`-ом перед `INSERT`, разбор текста ошибки, FK без индекса на ссылающемся столбце, `ON DELETE CASCADE` «везде».
- [ ] **Step 2: ADR-0014** — схема на контекст, межсхемные FK к `users.users` и цена, первый шаг при выделении сервиса, отвергнутые варианты (одна схема `public`, база на контекст).
- [ ] **Step 3: Проверить; Step 4: Commit и тег** `m02-l04`.

### Task 13: Урок 2.5 «Проектирование от паттернов доступа» (тег `m02-l05`)

- [ ] **Step 1: Урок (контракт)** — строка 2.5 спеки; `EXPLAIN` из `results/messenger-explain.txt` (без глубокого разбора — 2b); keyset-пагинация из 1.11 в SQL (регион адаптера); оптимистичная блокировка `version` и выдача `seq` (регион адаптера); сцена `AccessPatternsSandbox`. Типичные ошибки: `OFFSET`-пагинация, индекс «на всё», порядок столбцов составного ключа не от запроса, `SELECT FOR UPDATE` там, где хватает `version`.
- [ ] **Step 2: Проверить; Step 3: Commit и тег** `m02-l05`.

### Task 14: Урок 2.6 «JSONB, soft delete и история» (тег `m02-l06`)

- [ ] **Step 1: Урок (контракт)** — строка 2.6 спеки; лаба `jsonb/`; таблица «soft delete / архив / удаление» с последствиями для `UNIQUE`, FK и права на удаление; аудит (триггер против событий против temporal). Типичные ошибки: JSONB для всего, `deleted_at` без частичного уникального индекса, аудит в той же таблице.
- [ ] **Step 2: Проверить; Step 3: Commit и тег** `m02-l06`.

### Task 15: Урок 2.7 «Multi-tenancy, именование, антипаттерны» (тег `m02-l07`)

- [ ] **Step 1: Урок (контракт)** — строка 2.7 спеки; `antipatterns.sql` из `results`; RLS — пример политики (в лабе); соглашения об именовании, принятые в мессенджере (имена ограничений из миграции). Типичные ошибки — сами антипаттерны с исправлениями.
- [ ] **Step 2: Проверить; Step 3: Commit и тег** `m02-l07`.

### Task 16: Урок 2.8 «Миграции как код» и ADR-0016 (тег `m02-l08`)

**Files:** Create: `08-migrations.md`, `messenger/docs/adr/0016-миграции.md`

- [ ] **Step 1: Урок (контракт)** — строка 2.8 спеки; реальный вывод `migrate up`/`status` и отказ старта при устаревшей схеме; почему отдельный шаг; `database/sql` через `pgx/v5/stdlib` только здесь и почему; сцена `MigrationRun`; анонс expand/contract (2b). Типичные ошибки: миграции при старте нескольких экземпляров, правка применённой миграции, `Down`, который теряет данные, DDL вне миграций.
- [ ] **Step 2: ADR-0016** — goose, встроенные миграции, подкоманда, сервис compose; отвергнутые: tern, golang-migrate, Atlas, миграции при старте.
- [ ] **Step 3: Проверить; Step 4: Commit и тег** `m02-l08`.

### Task 17: Урок 2.9 «Мессенджер на Postgres» и ADR-0015 (тег `m02-l09`)

**Files:** Create: `09-postgres-adapter.md`, `messenger/docs/adr/0015-транзакции.md`; Modify: уроки 1.1 (обещание «в 2a заменим» → ссылка), 1.4/1.12 (`storetest` против Postgres), 0.4 (размер пула в коде мессенджера), ADR-0005 (очистка в базе)

- [ ] **Step 1: Урок (контракт)** — строка 2.9 спеки; `pgxpool` и `MaxConns` по закону Литтла (числа из профиля 1.5.10); транзакции (`pgtx`, регионы), диаграмма «запрос → use case → порт → pgx → строка» (mermaid); `storetest` на testcontainers (вывод прогона); `/readyz` с базой; порядок остановки (журнал из `results/messenger-smoke.txt`); раздел «Что изменилось в мессенджере» с деревом и явным «что не изменилось: домен и use case'ы, кроме `Transactor`» (`git diff --stat` по `internal/*/domain`). Типичные ошибки: транзакция внутри адаптера на несколько вызовов use case, горутины внутри `WithinTx`, пул на запрос, забытый `pgtx.Q`.
- [ ] **Step 2: ADR-0015** — транзакция в `ctx`, порт `Transactor`, сделать против купить (`go-transaction-manager`), риски и защита.
- [ ] **Step 3: Правки ранних уроков и ADR-0005.**
- [ ] **Step 4: Проверить; Step 5: Commit и тег** `m02-l09`.

### Task 18: Итоговая проверка, README, бэклог

- [ ] **Step 1:** Все проверки курса, лаб, мессенджера (включая интеграционные с Docker); на каждом теге `m02-l01…l09` — `go build`, `go test -race` (интеграционные — с Docker), `golangci-lint`.
- [ ] **Step 2:** Playwright на 375/1280 px, обе темы: уроки 2.1–2.9, все шесть сцен (прокрутка, реакция, полноэкранный режим, mermaid, контраст).
- [ ] **Step 3:** README (модуль 2a, запуск с Postgres, секреты), бэклог («Выполнено в 2a», «Остатки модуля 2a»), страница модуля.
- [ ] **Step 4: Commit** — `docs: итог части 2a — README, бэклог, страница модуля`. Затем — ревью ветки и PR (merge commit) с согласия пользователя.
