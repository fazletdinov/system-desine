-- Урок 2.7: мультиарендность в общей схеме — столбец tenant_id и защита на уровне строк (RLS).
-- Два арендатора (acme и globex) в одних таблицах; роль приложения видит только строки того
-- арендатора, которого запрос назвал в настройке app.tenant_id.
-- Запуск из labs/: docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/tenancy.sql 2>&1'
\set QUIET on
SET client_min_messages = warning;
DROP SCHEMA IF EXISTS tenancy CASCADE;
-- Роли живут в кластере, а не в базе, и схема их не удаляет. Удаляем после схемы: пока на роль
-- ссылаются права и владение объектами, DROP ROLE не пройдёт.
DROP ROLE IF EXISTS tenancy_app;
DROP ROLE IF EXISTS tenancy_owner;
CREATE SCHEMA tenancy;
SET search_path = tenancy;
RESET client_min_messages;
\set QUIET off
\set ECHO queries

\echo
\echo '=== 1. Общие таблицы: tenant_id в каждой строке и в каждом ключе ==='
-- #region tenancy-schema
CREATE TABLE tenants (id bigint PRIMARY KEY, name text NOT NULL UNIQUE);

-- tenant_id — первый столбец первичного ключа: запросы арендатора читают свой диапазон индекса.
CREATE TABLE chats (
    tenant_id bigint NOT NULL REFERENCES tenants (id),
    id        bigint NOT NULL,
    title     text   NOT NULL,
    PRIMARY KEY (tenant_id, id)
);

-- Внешний ключ включает tenant_id: сообщение может ссылаться только на чат своего арендатора.
CREATE TABLE messages (
    tenant_id bigint NOT NULL,
    chat_id   bigint NOT NULL,
    seq       bigint NOT NULL,
    text      text   NOT NULL,
    PRIMARY KEY (tenant_id, chat_id, seq),
    FOREIGN KEY (tenant_id, chat_id) REFERENCES chats (tenant_id, id)
);

INSERT INTO tenants VALUES (1, 'acme'), (2, 'globex');
INSERT INTO chats VALUES
    (1, 10, 'acme: бэкенд'),
    (1, 11, 'acme: обед'),
    (2, 20, 'globex: релиз'),
    (2, 21, 'globex: зарплаты');
-- #endregion tenancy-schema

-- #region tenancy-policy
-- Роль, от которой работает приложение: не владелец таблиц и не суперпользователь.
CREATE ROLE tenancy_app NOLOGIN;
GRANT USAGE ON SCHEMA tenancy TO tenancy_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON chats, messages TO tenancy_app;

ALTER TABLE chats ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;

-- Политика без WITH CHECK применяет USING и к чтению, и к записи: видеть, менять и вставлять
-- можно только строки арендатора из настройки. Нет настройки — ошибка, а не «все строки».
CREATE POLICY tenant_isolation ON chats
    USING (tenant_id = current_setting('app.tenant_id')::bigint);
CREATE POLICY tenant_isolation ON messages
    USING (tenant_id = current_setting('app.tenant_id')::bigint);
-- #endregion tenancy-policy

\echo
\echo '=== 2. Суперпользователь политик не замечает ==='
-- #region tenancy-superuser
SELECT rolname, rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user;
SELECT tenant_id, id, title FROM chats ORDER BY tenant_id, id;
-- #endregion tenancy-superuser

\echo
\echo '=== 3. Роль приложения без арендатора ==='
-- #region tenancy-no-tenant
SET ROLE tenancy_app;
SELECT id, title FROM chats;
-- #endregion tenancy-no-tenant
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 4. Запрос арендатора: настройка живёт до конца транзакции ==='
-- #region tenancy-request
BEGIN;
-- Третий аргумент true — «только до конца транзакции», как SET LOCAL.
SELECT set_config('app.tenant_id', '1', true);

-- Ни одного WHERE tenant_id — политика подставила его сама.
SELECT id, title FROM chats ORDER BY id;

-- Чужой чат по id не найти и не изменить: для этой транзакции его нет.
SELECT count(*) AS globex_chat_visible FROM chats WHERE id = 20;
UPDATE chats SET title = 'взломано' WHERE id = 20;

EXPLAIN (COSTS OFF) SELECT id, title FROM chats WHERE id = 10;
COMMIT;
-- #endregion tenancy-request

\echo
\echo '=== 5. Запись в чужого арендатора ==='
-- #region tenancy-with-check
BEGIN;
SELECT set_config('app.tenant_id', '1', true);
INSERT INTO chats VALUES (2, 22, 'acme подбрасывает чат в globex');
-- #endregion tenancy-with-check
\echo SQLSTATE :SQLSTATE
ROLLBACK;

-- #region tenancy-composite-fk
BEGIN;
SELECT set_config('app.tenant_id', '1', true);
-- Строка своя (tenant_id = 1), а чат 20 — чужой: пары (1, 20) в chats нет.
INSERT INTO messages VALUES (1, 20, 1, 'acme пишет в чат globex');
-- #endregion tenancy-composite-fk
\echo SQLSTATE :SQLSTATE
ROLLBACK;

\echo
\echo '=== 6. После COMMIT настройки нет ==='
-- #region tenancy-after-commit
-- Параметр остался известен сеансу, но пуст: приведение '' к bigint падает — снова не «все строки».
SELECT current_setting('app.tenant_id') = '' AS tenant_is_empty;
SELECT id, title FROM chats;
-- #endregion tenancy-after-commit
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 7. SET без LOCAL: арендатор переживает запрос ==='
-- #region tenancy-session-leak
-- Запрос globex выставил арендатора на весь сеанс и вернул соединение в пул.
SET app.tenant_id = '2';
SELECT count(*) AS globex_chats FROM chats;

-- Следующий запрос на том же соединении — от acme, и он забыл выставить своего арендатора.
-- Ошибки нет: он видит чаты globex.
SELECT string_agg(title, ', ' ORDER BY id) AS next_request_sees FROM chats;
RESET app.tenant_id;
-- #endregion tenancy-session-leak

\echo
\echo '=== 8. Параметр в SET не передать, в set_config — можно ==='
-- \bind отправляет запрос расширенным протоколом с параметром $1 — так же, как pgx.
-- #region tenancy-bind
BEGIN;
SET LOCAL app.tenant_id = $1 \bind 1 \g
-- #endregion tenancy-bind
\echo SQLSTATE :SQLSTATE
ROLLBACK;

-- #region tenancy-bind-fix
BEGIN;
SELECT set_config('app.tenant_id', $1, true) \bind 1 \g
SELECT count(*) AS acme_chats FROM chats;
COMMIT;
-- #endregion tenancy-bind-fix

\echo
\echo '=== 9. Владелец таблицы обходит политики, пока не FORCE ==='
-- #region tenancy-owner
RESET ROLE;
CREATE ROLE tenancy_owner NOLOGIN;
GRANT USAGE ON SCHEMA tenancy TO tenancy_owner;
ALTER TABLE chats OWNER TO tenancy_owner;

SET ROLE tenancy_owner;
BEGIN;
SELECT set_config('app.tenant_id', '1', true);
SELECT count(*) AS owner_sees FROM chats;
COMMIT;

RESET ROLE;
ALTER TABLE chats FORCE ROW LEVEL SECURITY;

SET ROLE tenancy_owner;
BEGIN;
SELECT set_config('app.tenant_id', '1', true);
SELECT count(*) AS owner_sees_forced FROM chats;
COMMIT;

-- Суперпользователь обходит RLS и с FORCE.
RESET ROLE;
SELECT count(*) AS superuser_sees_forced FROM chats;
-- #endregion tenancy-owner
