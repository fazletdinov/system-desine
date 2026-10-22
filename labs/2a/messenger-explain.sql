-- Урок 2.5: планы главных запросов мессенджера на схеме из его миграции и на данных
-- правдоподобной формы. По этим планам выбирался индекс списка чатов (задача 10 плана 2a).
-- Запуск из labs/: docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/messenger-explain.sql 2>&1'
--
-- Схема — не копия, а сама миграция messenger/migrations/00001_init.sql (каталог смонтирован
-- в контейнер как /messenger-migrations): из файла берётся часть Up, без Down. Запросы —
-- те же, что в chat/adapters/postgres/store.go (listChatsFirst, listChatsAfter,
-- listMessagesBefore), с параметрами через PREPARE, как их отправляет pgx.
\set QUIET on
-- Намеренных ошибок здесь нет: любая — сбой подготовки, и планы после неё были бы о пустых таблицах.
\set ON_ERROR_STOP on
SET client_min_messages = warning;
DROP SCHEMA IF EXISTS platform, chat, auth, users CASCADE;
\! sed -n '/^-- +goose Up/,/^-- +goose Down/p' /messenger-migrations/00001_init.sql > /tmp/messenger-up.sql
\i /tmp/messenger-up.sql
RESET client_min_messages;
-- Одинаковые «случайные» данные при каждом запуске — и одинаковые планы.
SELECT setseed(0.42) \g /dev/null
-- Без параллельных планов: ANALYZE берёт случайную выборку строк, и от прогона к прогону оценка
-- «сколько чатов у пользователя» колеблется настолько, что планировщик то зовёт параллельного
-- исполнителя (Gather Merge), то нет. Индексы в плане от этого не меняются, а планы, которые
-- цитирует урок, должны совпадать при каждом запуске. Параллельные планы — часть 2b.
SET max_parallel_workers_per_gather = 0;
\set QUIET off
\set ECHO queries

\echo
\echo '=== 1. Данные: 100 000 пользователей, 1 000 000 чатов; у «тяжёлого» пользователя 1 000 чатов, у «лёгкого» — 20 ==='
-- id — по номеру, как в storetest: пользователь 1 — тяжёлый, 2 — лёгкий.
CREATE FUNCTION pg_temp.uid(n bigint) RETURNS uuid
    LANGUAGE sql IMMUTABLE RETURN format('00000000-0000-7000-8000-%s', lpad(n::text, 12, '0'))::uuid;
CREATE FUNCTION pg_temp.cid(n bigint) RETURNS uuid
    LANGUAGE sql IMMUTABLE RETURN format('00000000-0000-7000-9000-%s', lpad(n::text, 12, '0'))::uuid;

INSERT INTO users.users (id, username, display_name, created_at)
SELECT pg_temp.uid(n), 'user_' || n, 'Пользователь ' || n, timestamptz '2026-01-01 00:00Z'
FROM generate_series(1, 100000) AS n;

-- Группы, а не личные чаты: так не нужен уникальный ключ пары, а форма данных для списка
-- та же. Активность — случайный момент за 30 дней после создания.
INSERT INTO chat.chats (id, kind, title, last_seq, version, created_at, last_activity_at)
SELECT pg_temp.cid(n), 'group', 'чат ' || n, 0, 0, timestamptz '2026-09-01 00:00Z',
       timestamptz '2026-09-01 00:00Z' + random() * interval '30 days'
FROM generate_series(1, 1000000) AS n;

-- По два участника из 3…100000 в каждом чате (в среднем 20 чатов на пользователя), плюс
-- тяжёлый в чатах 1–1000 и лёгкий в 1001–1020.
INSERT INTO chat.members (chat_id, user_id)
SELECT pg_temp.cid(n), pg_temp.uid(3 + n % 99998) FROM generate_series(1, 1000000) AS n
UNION
SELECT pg_temp.cid(n), pg_temp.uid(3 + (n::bigint * 7919) % 99998) FROM generate_series(1, 1000000) AS n
UNION
SELECT pg_temp.cid(n), pg_temp.uid(1) FROM generate_series(1, 1000) AS n
UNION
SELECT pg_temp.cid(n), pg_temp.uid(2) FROM generate_series(1001, 1020) AS n;

-- История одного чата: 10 000 сообщений. id — UUIDv7, как у idgen мессенджера (растёт со временем);
-- client_message_id присылает клиент, и он случаен — UUIDv4.
INSERT INTO chat.messages (chat_id, seq, id, author_id, client_message_id, text, sent_at)
SELECT pg_temp.cid(1), s, uuidv7(), pg_temp.uid(1), gen_random_uuid(), 'сообщение ' || s,
       timestamptz '2026-09-01 00:00Z' + s * interval '1 minute'
FROM generate_series(1, 10000) AS s;
UPDATE chat.chats SET last_seq = 10000, version = 10000 WHERE id = pg_temp.cid(1);

VACUUM ANALYZE users.users, chat.chats, chat.members, chat.messages;

-- #region list-chats-prepare
PREPARE list_chats_first(uuid, int) AS
SELECT c.id, c.kind, c.title, c.last_seq, c.version, c.created_at, c.last_activity_at,
       ARRAY(SELECT m.user_id FROM chat.members m WHERE m.chat_id = c.id ORDER BY m.user_id)
FROM chat.chats c
JOIN chat.members me ON me.chat_id = c.id
WHERE me.user_id = $1
ORDER BY c.last_activity_at DESC, c.id DESC
LIMIT $2;

PREPARE list_chats_after(uuid, int, timestamptz, uuid) AS
SELECT c.id, c.kind, c.title, c.last_seq, c.version, c.created_at, c.last_activity_at,
       ARRAY(SELECT m.user_id FROM chat.members m WHERE m.chat_id = c.id ORDER BY m.user_id)
FROM chat.chats c
JOIN chat.members me ON me.chat_id = c.id
WHERE me.user_id = $1 AND (c.last_activity_at, c.id) < ($3, $4)
ORDER BY c.last_activity_at DESC, c.id DESC
LIMIT $2;
-- #endregion list-chats-prepare

-- Курсор после 500-го чата тяжёлого пользователя — середина его списка.
SELECT c.last_activity_at AS cursor_at, c.id AS cursor_id
FROM chat.chats c JOIN chat.members me ON me.chat_id = c.id
WHERE me.user_id = pg_temp.uid(1)
ORDER BY c.last_activity_at DESC, c.id DESC
OFFSET 499 LIMIT 1 \gset

\echo
\echo '=== 2. Первая страница списка чатов (listChatsFirst, limit 51 = 50 + «есть ли дальше») ==='
\echo '--- тяжёлый пользователь (1 000 чатов)'
-- #region plan-heavy
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF)
EXECUTE list_chats_first(pg_temp.uid(1), 51);
-- #endregion plan-heavy

\echo '--- лёгкий пользователь (20 чатов)'
-- #region plan-light
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF)
EXECUTE list_chats_first(pg_temp.uid(2), 51);
-- #endregion plan-light

\echo
\echo '=== 3. Следующая страница (listChatsAfter): курсор (last_activity_at, id) после 500-го чата тяжёлого ==='
-- #region plan-after
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF)
EXECUTE list_chats_after(pg_temp.uid(1), 51, :'cursor_at', :'cursor_id');
-- #endregion plan-after

\echo
\echo '=== 4. Индекс chat.chats (last_activity_at DESC, id DESC), которого в схеме нет: план его не берёт, а сообщения теряют HOT ==='
-- Запись сообщения — тот же оператор, что AppendMessage в адаптере.
-- #region append-prepare
PREPARE append_message(uuid, bigint, timestamptz, uuid, uuid, uuid, text, timestamptz) AS
WITH chat AS (
    UPDATE chat.chats
    SET last_seq = last_seq + 1, version = version + 1, last_activity_at = $3
    WHERE id = $1 AND version = $2
    RETURNING last_seq
)
INSERT INTO chat.messages (chat_id, seq, id, author_id, client_message_id, text, sent_at)
SELECT $1, chat.last_seq, $4, $5, $6, $7, $8 FROM chat
RETURNING seq;
-- #endregion append-prepare

-- 200 сообщений в чат 2, каждое — своей транзакцией, как в мессенджере. Перед замером счётчики
-- статистики сбрасываются в общую память (pg_stat_force_next_flush), иначе прочитали бы старые.
\set QUIET on
CREATE FUNCTION pg_temp.send_batch(from_version int) RETURNS SETOF text LANGUAGE sql AS $$
SELECT format('EXECUTE append_message(%L, %s, %L, uuidv7(), %L, gen_random_uuid(), ''сообщение'', %L)',
           pg_temp.cid(2), v, timestamptz '2026-10-01 00:00Z' + v * interval '1 second', pg_temp.uid(3),
           timestamptz '2026-10-01 00:00Z' + v * interval '1 second')
FROM generate_series(from_version, from_version + 199) AS v
$$;
CREATE FUNCTION pg_temp.chat_updates() RETURNS TABLE (n_tup_upd bigint, n_tup_hot_upd bigint)
    LANGUAGE sql RETURN (SELECT ROW(n_tup_upd, n_tup_hot_upd)
                         FROM pg_stat_user_tables WHERE relid = 'chat.chats'::regclass);
\set QUIET off

\echo '--- без индекса (схема мессенджера): 200 сообщений'
SELECT pg_stat_force_next_flush() \g /dev/null
SELECT n_tup_upd AS upd0, n_tup_hot_upd AS hot0 FROM pg_temp.chat_updates() \gset
\set ECHO none
\o /dev/null
SELECT pg_temp.send_batch(0) \gexec
\o
\set ECHO queries
SELECT pg_stat_force_next_flush() \g /dev/null
SELECT n_tup_upd - :upd0 AS updates, n_tup_hot_upd - :hot0 AS hot_updates FROM pg_temp.chat_updates();

\echo '--- с индексом: план первой страницы и ещё 200 сообщений'
CREATE INDEX chats_last_activity_idx ON chat.chats (last_activity_at DESC, id DESC);
ANALYZE chat.chats;
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF)
EXECUTE list_chats_first(pg_temp.uid(1), 51);
SELECT pg_stat_force_next_flush() \g /dev/null
SELECT n_tup_upd AS upd0, n_tup_hot_upd AS hot0 FROM pg_temp.chat_updates() \gset
\set ECHO none
\o /dev/null
SELECT pg_temp.send_batch(200) \gexec
\o
\set ECHO queries
SELECT pg_stat_force_next_flush() \g /dev/null
SELECT n_tup_upd - :upd0 AS updates, n_tup_hot_upd - :hot0 AS hot_updates FROM pg_temp.chat_updates();
DROP INDEX chat.chats_last_activity_idx;

\echo
\echo '=== 5. members по (user_id, chat_id) вместо (user_id): chat_id — из индекса, без чтения таблицы (с откатом) ==='
BEGIN;
DROP INDEX chat.members_user_id_idx;
CREATE INDEX members_user_id_chat_id_idx ON chat.members (user_id, chat_id);
ANALYZE chat.members;
-- #region plan-covering
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF)
EXECUTE list_chats_first(pg_temp.uid(1), 51);
-- #endregion plan-covering
SELECT relname AS index, pg_size_pretty(pg_relation_size(oid)) AS size
FROM pg_class WHERE relname IN ('members_user_id_idx', 'members_user_id_chat_id_idx', 'members_pkey');
ROLLBACK;
SELECT relname AS index, pg_size_pretty(pg_relation_size(oid)) AS size
FROM pg_class WHERE relname IN ('members_user_id_idx', 'members_pkey');

\echo
\echo '=== 6. Страница истории чата (listMessagesBefore): диапазон первичного ключа (chat_id, seq) ==='
PREPARE list_messages_before(uuid, bigint, int) AS
SELECT chat_id, seq, id, author_id, client_message_id, text, sent_at FROM chat.messages
WHERE chat_id = $1 AND seq < $2
ORDER BY seq DESC
LIMIT $3;

-- #region plan-history
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF)
EXECUTE list_messages_before(pg_temp.cid(1), 5000, 51);
-- #endregion plan-history

\echo
\echo '=== 7. OFFSET против keyset: та же страница по номеру позиции ==='
-- Страница истории из раздела 6 (seq 4999…4949), заданная не ключом, а смещением: от новейшего
-- seq 10000 до 4999 — 5001 строка. Индекс знает порядок строк, но не их номера, поэтому
-- пропущенные строки всё равно читаются.
-- #region offset-history
PREPARE list_messages_offset(uuid, int, int) AS
SELECT chat_id, seq, id, author_id, client_message_id, text, sent_at FROM chat.messages
WHERE chat_id = $1
ORDER BY seq DESC
OFFSET $2 LIMIT $3;
-- #endregion offset-history

SELECT max(seq) AS first_seq, min(seq) AS last_seq
FROM (SELECT seq FROM chat.messages WHERE chat_id = pg_temp.cid(1) ORDER BY seq DESC OFFSET 5001 LIMIT 51) AS page;

-- #region plan-offset-history
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF)
EXECUTE list_messages_offset(pg_temp.cid(1), 5001, 51);
-- #endregion plan-offset-history

\echo '--- список чатов тяжёлого пользователя: OFFSET 500 — та же страница, что курсор из раздела 3'
PREPARE list_chats_offset(uuid, int, int) AS
SELECT c.id, c.kind, c.title, c.last_seq, c.version, c.created_at, c.last_activity_at,
       ARRAY(SELECT m.user_id FROM chat.members m WHERE m.chat_id = c.id ORDER BY m.user_id)
FROM chat.chats c
JOIN chat.members me ON me.chat_id = c.id
WHERE me.user_id = $1
ORDER BY c.last_activity_at DESC, c.id DESC
OFFSET $2 LIMIT $3;

-- #region plan-offset-chats
EXPLAIN (ANALYZE, BUFFERS, COSTS OFF, TIMING OFF)
EXECUTE list_chats_offset(pg_temp.uid(1), 500, 51);
-- #endregion plan-offset-chats

\echo
\echo '=== 8. Цена ключей chat.messages: таблица и три её индекса на 10 000 сообщений чата 1 и 400 — чата 2 ==='
-- Каждый INSERT сообщения пишет строку таблицы и по записи в каждый из трёх индексов: первичный
-- ключ (chat_id, seq) — для ленты, UNIQUE (id) — для поиска по id, UNIQUE (chat_id, author_id,
-- client_message_id) — для защиты от повторной отправки.
SELECT c.relname AS relation, pg_relation_size(c.oid) AS bytes, pg_size_pretty(pg_relation_size(c.oid)) AS size
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'chat' AND c.relname IN ('messages', 'messages_pkey', 'messages_id_key', 'messages_client_id_key')
ORDER BY bytes DESC;
