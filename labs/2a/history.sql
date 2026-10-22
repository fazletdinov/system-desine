-- Урок 2.6: мягкое удаление (deleted_at), перенос в архив и настоящее удаление — что каждое делает с
-- UNIQUE, внешними ключами и правом на удаление; журнал аудита триггером и его цена.
-- Запуск из labs/: docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/history.sql 2>&1'
\set QUIET on
SET client_min_messages = warning;
DROP SCHEMA IF EXISTS history CASCADE;
CREATE SCHEMA history;
SET search_path = history;
RESET client_min_messages;
-- Параллельные планы выключены: от случайной выборки ANALYZE план то звал бы параллельного
-- исполнителя, то нет, и цитаты урока не воспроизводились бы.
SET max_parallel_workers_per_gather = 0;
\set QUIET off
\set ECHO queries

\echo
\echo '=== 1. Схема: пользователи с deleted_at, их сессии и сообщения, журнал аудита ==='
-- #region soft-schema
CREATE TABLE users (
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username     text        NOT NULL CONSTRAINT users_username_key UNIQUE,
    display_name text        NOT NULL,
    -- NULL — пользователь жив; время — когда его «удалили».
    deleted_at   timestamptz
);
-- Сессии принадлежат пользователю: при настоящем удалении база убрала бы их сама.
CREATE TABLE sessions (
    token   text   PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE
);
-- Сообщения переживают автора (NO ACTION): удалить автора, пока они есть, база не даст.
CREATE TABLE messages (
    id        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    author_id bigint NOT NULL REFERENCES users (id),
    text      text   NOT NULL
);
-- #endregion soft-schema

-- Журнал ведётся с первой строки: разбор — шаг 6, здесь он нужен, чтобы история alice была полной.
-- #region audit
CREATE TABLE users_audit (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    bigint      NOT NULL,
    op         text        NOT NULL CHECK (op IN ('INSERT', 'UPDATE', 'DELETE')),
    -- Кто из пользователей приложения: приложение кладёт его в настройку сеанса app.actor.
    -- Ручной SQL её не задаёт — тогда здесь NULL, и остаётся только роль базы (db_role).
    actor      text,
    db_role    text        NOT NULL DEFAULT current_user,
    changed_at timestamptz NOT NULL DEFAULT now(),
    old_row    jsonb,
    new_row    jsonb
);

CREATE FUNCTION users_audit_row() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO users_audit (user_id, op, actor, old_row, new_row)
    VALUES (coalesce(NEW.id, OLD.id), TG_OP,
            -- true: настройки нет — NULL, а не ошибка.
            current_setting('app.actor', true),
            CASE WHEN TG_OP <> 'INSERT' THEN to_jsonb(OLD) END,
            CASE WHEN TG_OP <> 'DELETE' THEN to_jsonb(NEW) END);
    RETURN NULL; -- AFTER-триггеру результат не нужен
END;
$$;

CREATE TRIGGER users_audit AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION users_audit_row();
-- #endregion audit

\echo
\echo '=== 2. Мягкое удаление при обычном UNIQUE: имя удалённого занято навсегда ==='
-- #region soft-delete
INSERT INTO users (username, display_name) VALUES ('alice', 'Алиса'), ('bob', 'Боб');
INSERT INTO sessions VALUES ('tok-alice', 1);
INSERT INTO messages (author_id, text) VALUES (1, 'привет'), (2, 'привет, Алиса');

UPDATE users SET deleted_at = now() WHERE username = 'alice';
-- ON DELETE CASCADE срабатывает на DELETE, а это UPDATE: сессия «удалённой» alice жива.
SELECT token, user_id FROM sessions;
-- Через год кто-то другой хочет имя alice.
INSERT INTO users (username, display_name) VALUES ('alice', 'Другая Алиса');
-- #endregion soft-delete
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 3. Частичный уникальный индекс: уникальны только живые ==='
-- #region partial-unique
ALTER TABLE users DROP CONSTRAINT users_username_key;
CREATE UNIQUE INDEX users_username_active_key ON users (username) WHERE deleted_at IS NULL;

INSERT INTO users (username, display_name) VALUES ('alice', 'Другая Алиса');
-- Второй живой alice — по-прежнему ошибка.
INSERT INTO users (username, display_name) VALUES ('alice', 'Третья Алиса');
-- #endregion partial-unique
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 4. Запрос, забывший про deleted_at, и внешний ключ, который о нём не знает ==='
-- #region forgotten-filter
-- Вход по имени без условия: две строки alice, и какая из них «та», запрос не знает.
SELECT id, username, display_name, deleted_at IS NOT NULL AS deleted
FROM users WHERE username = 'alice' ORDER BY id;
-- Внешний ключ проверяет, что строка есть, а не что она жива: сообщение от удалённой alice принято.
INSERT INTO messages (author_id, text) VALUES (1, 'пишу после удаления');
-- #endregion forgotten-filter

-- #region active-view
-- Представление — одно место, где записано «живой»; запросы приложения читают из него.
CREATE VIEW active_users AS
SELECT id, username, display_name FROM users WHERE deleted_at IS NULL;

SELECT id, username, display_name FROM active_users WHERE username = 'alice';
-- #endregion active-view

-- #region partial-plan-setup
-- 100 000 пользователей, каждый десятый удалён.
INSERT INTO users (username, display_name, deleted_at)
SELECT 'user' || g, 'Пользователь ' || g, CASE WHEN g % 10 = 0 THEN now() END
FROM generate_series(1, 100000) AS g;
VACUUM ANALYZE users;
-- #endregion partial-plan-setup

-- #region partial-plan
-- Частичный индекс годится, только если из условия запроса следует его WHERE.
EXPLAIN (COSTS OFF) SELECT id FROM users WHERE username = 'user42';
EXPLAIN (COSTS OFF) SELECT id FROM users WHERE username = 'user42' AND deleted_at IS NULL;
EXPLAIN (COSTS OFF) SELECT id FROM active_users WHERE username = 'user42';
-- #endregion partial-plan

\echo
\echo '=== 5. Перенос в архив: строка уходит, а дети остаются ==='
-- #region archive
CREATE TABLE users_archive (
    id           bigint      PRIMARY KEY,
    username     text        NOT NULL,
    display_name text        NOT NULL,
    archived_at  timestamptz NOT NULL
);

-- bob — живой пользователь с сообщением. Перенос — DELETE … RETURNING и INSERT одной командой.
WITH moved AS (
    DELETE FROM users WHERE username = 'bob' RETURNING id, username, display_name
)
INSERT INTO users_archive SELECT id, username, display_name, now() FROM moved;
-- #endregion archive
\echo SQLSTATE :SQLSTATE
-- #region archive-ok
-- У user7 нет ни сообщений, ни сессий — перенос проходит.
WITH moved AS (
    DELETE FROM users WHERE username = 'user7' RETURNING id, username, display_name
)
INSERT INTO users_archive SELECT id, username, display_name, now() FROM moved;
SELECT username FROM users_archive;
-- #endregion archive-ok

\echo
\echo '=== 6. Журнал аудита: кто изменил строку ==='
-- #region audit-actor
-- Приложение: SET LOCAL живёт до конца транзакции и не протекает в чужие запросы пула.
BEGIN;
SET LOCAL app.actor = 'user:2';
UPDATE users SET display_name = 'Боб Б.' WHERE username = 'bob';
COMMIT;
-- Ручная правка из psql: триггер её поймал, но кто из людей это был — не знает.
UPDATE users SET display_name = 'Боб' WHERE username = 'bob';

SELECT op, actor, db_role, old_row ->> 'display_name' AS old_name, new_row ->> 'display_name' AS new_name
FROM users_audit WHERE user_id = 2 ORDER BY id;
-- #endregion audit-actor

\echo
\echo '=== 7. Право на удаление: alice просит стереть её данные ==='
-- #region erase-soft
-- Мягкое удаление ничего не стёрло: имя и отображаемое имя на месте.
SELECT username, display_name, deleted_at IS NOT NULL AS deleted FROM users WHERE id = 1;
-- Настоящее удаление упирается в сообщения alice в чужих чатах.
DELETE FROM users WHERE id = 1;
-- #endregion erase-soft
\echo SQLSTATE :SQLSTATE

-- #region erase
-- Обезличивание: строка остаётся ради ссылок из messages, персональных данных в ней нет.
-- Сессии удаляются явно: CASCADE на UPDATE не срабатывает.
BEGIN;
UPDATE users
SET username = 'deleted_' || id, display_name = 'Удалённый пользователь'
WHERE id = 1;
DELETE FROM sessions WHERE user_id = 1;
COMMIT;

SELECT u.username, m.text
FROM messages m JOIN users u ON u.id = m.author_id
WHERE m.author_id = 1 ORDER BY m.id;
-- #endregion erase

-- #region audit-erase
-- В таблице имени больше нет — а журнал хранит его в каждой строке истории alice.
SELECT op, old_row ->> 'username' AS old_name, new_row ->> 'username' AS new_name
FROM users_audit WHERE user_id = 1 ORDER BY id;
-- #endregion audit-erase

-- #region audit-scrub
-- Процедура удаления проходит и по журналу: из строк истории убираются поля с персональными
-- данными, а сами строки — кто, когда, какая операция — остаются.
UPDATE users_audit
SET old_row = old_row - ARRAY['username', 'display_name'],
    new_row = new_row - ARRAY['username', 'display_name']
WHERE user_id = 1;
SELECT count(*) AS audit_rows_with_name FROM users_audit
WHERE user_id = 1 AND (old_row ? 'username' OR new_row ? 'username');
-- #endregion audit-scrub

\echo
\echo '=== 8. Цена триггера: 100 000 обновлений с журналом и без ==='
-- #region audit-cost
-- Две одинаковые таблицы; у второй — тот же триггер аудита.
CREATE TABLE plain (LIKE users INCLUDING ALL);
CREATE TABLE audited (LIKE users INCLUDING ALL);
INSERT INTO plain (username, display_name) SELECT 'u' || g, 'Имя ' || g FROM generate_series(1, 100000) AS g;
INSERT INTO audited (username, display_name) SELECT 'u' || g, 'Имя ' || g FROM generate_series(1, 100000) AS g;
VACUUM ANALYZE plain, audited;
CREATE TRIGGER audited_audit AFTER INSERT OR UPDATE OR DELETE ON audited
FOR EACH ROW EXECUTE FUNCTION users_audit_row();
-- Отметки журнала до и после каждого UPDATE; CHECKPOINT перед каждым — равные условия: после
-- контрольной точки первое изменение страницы пишет её полный образ в обоих опытах.
CREATE TEMP TABLE wal_marks (mark text PRIMARY KEY, lsn pg_lsn NOT NULL);
-- Журнал от прежних шагов (в нём уже 10⁵ вставок шага 4) не мешает: считаем только прирост.
CREATE TEMP TABLE audit_before AS
SELECT count(*) AS n, pg_table_size('users_audit') AS bytes FROM users_audit;
-- #endregion audit-cost
CHECKPOINT;
INSERT INTO wal_marks VALUES ('plain_start', pg_current_wal_lsn());
-- #region audit-cost-plain
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF)
UPDATE plain SET display_name = display_name || '!';
-- #endregion audit-cost-plain
INSERT INTO wal_marks VALUES ('plain_end', pg_current_wal_lsn());
CHECKPOINT;
INSERT INTO wal_marks VALUES ('audited_start', pg_current_wal_lsn());
-- #region audit-cost-audited
EXPLAIN (ANALYZE, COSTS OFF, TIMING OFF)
UPDATE audited SET display_name = display_name || '!';
-- #endregion audit-cost-audited
INSERT INTO wal_marks VALUES ('audited_end', pg_current_wal_lsn());
-- #region audit-cost-wal
SELECT t.tbl,
       pg_size_pretty(pg_wal_lsn_diff(e.lsn, b.lsn)) AS wal,
       pg_wal_lsn_diff(e.lsn, b.lsn)                 AS wal_bytes
FROM (VALUES ('plain'), ('audited')) AS t (tbl)
JOIN wal_marks b ON b.mark = t.tbl || '_start'
JOIN wal_marks e ON e.mark = t.tbl || '_end'
ORDER BY wal_bytes;
-- Журнал только дописывается, поэтому его прирост — это строки этого опыта.
SELECT pg_size_pretty(pg_table_size('plain'))   AS plain_size,
       pg_size_pretty(pg_table_size('audited')) AS audited_size,
       (SELECT count(*) FROM users_audit) - b.n  AS new_audit_rows,
       pg_size_pretty(pg_table_size('users_audit') - b.bytes) AS audit_growth
FROM audit_before b;
-- #endregion audit-cost-wal
