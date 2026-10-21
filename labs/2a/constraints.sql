-- Урок 2.4: каждое ограничение нарушается намеренно, после каждой ошибки печатается SQLSTATE —
-- код, по которому адаптер переводит ошибку базы в доменную (pgconn.PgError.Code), а не по тексту.
-- Запуск из labs/: docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/constraints.sql 2>&1'
\set QUIET on
SET client_min_messages = warning;
DROP SCHEMA IF EXISTS constraints CASCADE;
CREATE SCHEMA constraints;
SET search_path = constraints, public;
-- EXCLUDE с «=» по bigint требует класса операторов GiST для обычных типов — его даёт btree_gist.
CREATE EXTENSION IF NOT EXISTS btree_gist SCHEMA public;
RESET client_min_messages;
\set QUIET off
\set ECHO queries

\echo
\echo '=== 1. Схема: имена ограничений заданы явно — по ним адаптер узнаёт, что нарушено ==='
-- #region schema
CREATE TABLE users (
    id       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username text NOT NULL
        CONSTRAINT users_username_key UNIQUE
        CONSTRAINT users_username_check
            CHECK (username ~ '^[a-z0-9_]+$' AND length(username) BETWEEN 3 AND 32)
);

CREATE TABLE chats (
    id    bigint PRIMARY KEY,
    title text   NOT NULL
);

CREATE TABLE members (
    chat_id bigint NOT NULL
        CONSTRAINT members_chat_id_fkey REFERENCES chats (id) ON DELETE CASCADE,
    user_id bigint NOT NULL
        CONSTRAINT members_user_id_fkey REFERENCES users (id),
    PRIMARY KEY (chat_id, user_id)
);
-- #endregion schema

INSERT INTO users (username) VALUES ('alice'), ('bob');
INSERT INTO chats VALUES (1, 'Бэкенд'), (2, 'Обед');
INSERT INTO members VALUES (1, 1), (1, 2), (2, 1);

\echo
\echo '=== 2. NOT NULL — 23502 not_null_violation ==='
-- #region not-null
INSERT INTO users (username) VALUES (NULL);
-- #endregion not-null
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 3. CHECK — 23514 check_violation ==='
-- #region check
INSERT INTO users (username) VALUES ('Alice!');
-- #endregion check
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 4. UNIQUE — 23505 unique_violation; VERBOSITY verbose показывает поля ошибки ==='
-- #region unique
-- Подробный режим psql печатает то же, что pgx кладёт в pgconn.PgError: Code, SchemaName,
-- TableName, ConstraintName. Адаптер сверяет Code и ConstraintName — текст сообщения не трогает.
\set VERBOSITY verbose
INSERT INTO users (username) VALUES ('alice');
\set VERBOSITY default
-- #endregion unique
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 5. FOREIGN KEY — 23503 foreign_key_violation, с обеих сторон связи ==='
-- #region foreign-key
-- Ссылка на несуществующий чат.
INSERT INTO members VALUES (99, 1);
-- #endregion foreign-key
\echo SQLSTATE :SQLSTATE
-- #region foreign-key-delete
-- Удаление пользователя, на которого ссылаются участники: действие по умолчанию NO ACTION
-- запрещает удаление, пока ссылки есть.
DELETE FROM users WHERE username = 'bob';
-- #endregion foreign-key-delete
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 6. ON DELETE CASCADE: удаление чата уносит его участников ==='
-- #region cascade
SELECT chat_id, count(*) AS members FROM members GROUP BY chat_id ORDER BY chat_id;
DELETE FROM chats WHERE id = 1;
SELECT chat_id, count(*) AS members FROM members GROUP BY chat_id ORDER BY chat_id;
-- #endregion cascade

\echo
\echo '=== 7. Частичный UNIQUE по выражению: адрес уникален без учёта регистра среди неудалённых ==='
-- #region partial-unique
CREATE TABLE emails (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    address    text NOT NULL,
    deleted_at timestamptz
);
-- Выражение lower(address) — регистр не различается; WHERE deleted_at IS NULL — удалённые
-- (soft delete) адреса в уникальности не участвуют, их можно занять заново.
CREATE UNIQUE INDEX emails_address_active_key ON emails (lower(address)) WHERE deleted_at IS NULL;

INSERT INTO emails (address) VALUES ('Alice@Example.com');
INSERT INTO emails (address) VALUES ('alice@example.com');
-- #endregion partial-unique
\echo SQLSTATE :SQLSTATE
-- #region partial-unique-reuse
UPDATE emails SET deleted_at = now() WHERE address = 'Alice@Example.com';
INSERT INTO emails (address) VALUES ('alice@example.com');
SELECT id, address, deleted_at IS NOT NULL AS deleted FROM emails ORDER BY id;
-- #endregion partial-unique-reuse

\echo
\echo '=== 8. EXCLUDE — 23P01 exclusion_violation: периоды тишины одного участника не пересекаются ==='
-- #region exclude
CREATE TABLE mutes (
    chat_id bigint    NOT NULL,
    user_id bigint    NOT NULL,
    during  tstzrange NOT NULL,
    CONSTRAINT mutes_no_overlap EXCLUDE USING gist (
        chat_id WITH =,
        user_id WITH =,
        during  WITH &&
    )
);

INSERT INTO mutes VALUES (2, 1, '[2026-10-20 10:00+00, 2026-10-20 12:00+00)');
-- Другой участник в то же время — можно.
INSERT INTO mutes VALUES (2, 2, '[2026-10-20 11:00+00, 2026-10-20 13:00+00)');
-- Встык: полуоткрытый диапазон [a, b) заканчивается до b — пересечения нет.
INSERT INTO mutes VALUES (2, 1, '[2026-10-20 12:00+00, 2026-10-20 14:00+00)');
-- Тот же участник, пересечение с 10:00–12:00.
INSERT INTO mutes VALUES (2, 1, '[2026-10-20 11:00+00, 2026-10-20 11:30+00)');
-- #endregion exclude
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 9. Отложенный FOREIGN KEY: проверка в COMMIT, а не в каждой команде ==='
-- #region deferrable-fk-immediate
ALTER TABLE members ALTER CONSTRAINT members_chat_id_fkey DEFERRABLE INITIALLY IMMEDIATE;

-- Без SET CONSTRAINTS ограничение проверяется сразу: участник раньше чата — ошибка.
BEGIN;
INSERT INTO members VALUES (3, 1);
-- #endregion deferrable-fk-immediate
\echo SQLSTATE :SQLSTATE
ROLLBACK;

-- #region deferrable-fk
-- Отложили до COMMIT: порядок команд внутри транзакции больше не важен.
BEGIN;
SET CONSTRAINTS members_chat_id_fkey DEFERRED;
INSERT INTO members VALUES (3, 1);
INSERT INTO chats VALUES (3, 'Релиз');
COMMIT;

-- Чат так и не появился: та же ошибка, но в COMMIT, и откатывается вся транзакция.
BEGIN;
SET CONSTRAINTS members_chat_id_fkey DEFERRED;
INSERT INTO members VALUES (4, 1);
COMMIT;
-- #endregion deferrable-fk
\echo SQLSTATE :SQLSTATE
SELECT chat_id, user_id FROM members ORDER BY chat_id, user_id;

\echo
\echo '=== 10. Отложенный UNIQUE: сдвиг позиций одной командой ==='
-- #region deferrable-unique
CREATE TABLE pins (
    chat_id  bigint NOT NULL,
    position int    NOT NULL,
    note     text   NOT NULL,
    CONSTRAINT pins_position_key UNIQUE (chat_id, position)
);
INSERT INTO pins VALUES (2, 1, 'правила'), (2, 2, 'расписание');

-- Новый закреп — первым: остальные сдвигаются на одну позицию. Неотложенный UNIQUE проверяется
-- после каждой строки, и на первой же строке позиция 2 ещё занята.
UPDATE pins SET position = position + 1 WHERE chat_id = 2;
-- #endregion deferrable-unique
\echo SQLSTATE :SQLSTATE
-- #region deferrable-unique-fix
-- DEFERRABLE (даже INITIALLY IMMEDIATE) переносит проверку на конец команды.
ALTER TABLE pins DROP CONSTRAINT pins_position_key;
ALTER TABLE pins ADD CONSTRAINT pins_position_key
    UNIQUE (chat_id, position) DEFERRABLE INITIALLY IMMEDIATE;
UPDATE pins SET position = position + 1 WHERE chat_id = 2;
INSERT INTO pins VALUES (2, 1, 'объявление');
SELECT position, note FROM pins WHERE chat_id = 2 ORDER BY position;
-- #endregion deferrable-unique-fix

\echo
\echo '=== 11. CHECK и NOT NULL отложить нельзя — 0A000 feature_not_supported ==='
-- #region check-not-deferrable
-- DEFERRABLE бывает только у UNIQUE, PRIMARY KEY, FOREIGN KEY и EXCLUDE.
ALTER TABLE users ADD CONSTRAINT users_username_short CHECK (length(username) <= 16) DEFERRABLE;
-- #endregion check-not-deferrable
\echo SQLSTATE :SQLSTATE
-- #region not-null-not-deferrable
ALTER TABLE chats ADD CONSTRAINT chats_title_not_null NOT NULL title DEFERRABLE;
-- #endregion not-null-not-deferrable
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 12. Ошибка ограничения обрывает транзакцию — 25P02 in_failed_sql_transaction ==='
-- #region aborted-transaction
BEGIN;
INSERT INTO users (username) VALUES ('alice');
-- #endregion aborted-transaction
\echo SQLSTATE :SQLSTATE
-- #region aborted-transaction-next
-- Тот же запрос, который до ошибки прошёл бы, — в оборванной транзакции база его не выполняет.
SELECT username FROM users WHERE username = 'alice';
-- #endregion aborted-transaction-next
\echo SQLSTATE :SQLSTATE
-- #region aborted-transaction-rollback
ROLLBACK;
SELECT username FROM users WHERE username = 'alice';
-- #endregion aborted-transaction-rollback

\echo
\echo '=== 13. Внешний ключ без индекса на ссылающемся столбце: удаление родителя ищет детей ==='
-- 1001 чат, 10⁶ сообщений в первых 1000; чат 1001 без сообщений — удаляется без единой ошибки,
-- но проверка внешнего ключа всё равно ищет его детей. Время — строка «Trigger for constraint».
\set QUIET on
-- #region fk-index-setup
CREATE TABLE big_chats (id bigint PRIMARY KEY);
CREATE TABLE big_messages (
    id      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    chat_id bigint NOT NULL CONSTRAINT big_messages_chat_id_fkey REFERENCES big_chats (id)
);
INSERT INTO big_chats SELECT g FROM generate_series(1, 1001) AS g;
INSERT INTO big_messages (chat_id) SELECT 1 + g % 1000 FROM generate_series(1, 1000000) AS g;
VACUUM ANALYZE big_chats, big_messages;
-- #endregion fk-index-setup
\set QUIET off
-- #region fk-without-index
EXPLAIN (ANALYZE, COSTS OFF, BUFFERS OFF) DELETE FROM big_chats WHERE id = 1001;
-- #endregion fk-without-index
-- #region fk-with-index
CREATE INDEX big_messages_chat_id_idx ON big_messages (chat_id);
INSERT INTO big_chats VALUES (1001);
EXPLAIN (ANALYZE, COSTS OFF, BUFFERS OFF) DELETE FROM big_chats WHERE id = 1001;
-- #endregion fk-with-index

\echo
\echo '=== 14. Отложенное ограничение не годится в арбитры ON CONFLICT — 55000 ==='
-- pins_position_key с шага 10 — DEFERRABLE INITIALLY IMMEDIATE.
-- #region deferrable-on-conflict
INSERT INTO pins VALUES (2, 1, 'дубль')
    ON CONFLICT (chat_id, position) DO UPDATE SET note = EXCLUDED.note;
-- #endregion deferrable-on-conflict
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 15. WITHOUT OVERLAPS (PostgreSQL 18): то же исключающее ограничение короче — 23P01 ==='
-- #region without-overlaps
CREATE TABLE mutes_wo (
    chat_id bigint    NOT NULL,
    user_id bigint    NOT NULL,
    during  tstzrange NOT NULL,
    CONSTRAINT mutes_wo_key UNIQUE (chat_id, user_id, during WITHOUT OVERLAPS)
);
INSERT INTO mutes_wo VALUES (2, 1, '[2026-10-20 10:00+00, 2026-10-20 12:00+00)');
INSERT INTO mutes_wo VALUES (2, 1, '[2026-10-20 11:00+00, 2026-10-20 11:30+00)');
-- #endregion without-overlaps
\echo SQLSTATE :SQLSTATE
