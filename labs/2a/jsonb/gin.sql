-- Урок 2.6: размер настроек в JSONB против столбцов и индекс GIN для поиска по содержимому.
-- Подробно об индексах — в 2b; здесь только то, что нужно для выбора «столбец или JSONB».
-- Запуск из labs/: docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/jsonb/gin.sql 2>&1'
\set QUIET on
SET client_min_messages = warning;
DROP SCHEMA IF EXISTS jsonb_gin CASCADE;
CREATE SCHEMA jsonb_gin;
SET search_path = jsonb_gin;
RESET client_min_messages;
-- Одинаковые «случайные» данные при каждом запуске — и одинаковые планы.
SELECT setseed(0.42) \g /dev/null
\set QUIET off
\set ECHO queries

\echo
\echo '=== 1. 100 000 пользователей: одни и те же настройки столбцами и в JSONB ==='
-- #region data
CREATE TABLE settings_columns (
    user_id         bigint  PRIMARY KEY,
    theme           text    NOT NULL,
    notify_mentions boolean NOT NULL,
    language        text    NOT NULL,
    beta            text[]  NOT NULL
);

-- Темы и языки — случайные; бета new_composer включена примерно у 1 % пользователей.
INSERT INTO settings_columns
SELECT g,
       (ARRAY['light', 'dark', 'system'])[1 + floor(random() * 3)::int],
       random() < 0.8,
       (ARRAY['ru', 'en', 'de', 'kk'])[1 + floor(random() * 4)::int],
       CASE WHEN random() < 0.01 THEN ARRAY['new_composer'] ELSE ARRAY[]::text[] END
FROM generate_series(1, 100000) AS g;

CREATE TABLE settings_jsonb AS
SELECT user_id,
       jsonb_build_object(
           'theme', theme,
           'notify', jsonb_build_object('mentions', notify_mentions),
           'language', language,
           'beta', to_jsonb(beta)) AS settings
FROM settings_columns;
ALTER TABLE settings_jsonb ADD PRIMARY KEY (user_id);
VACUUM ANALYZE settings_columns, settings_jsonb;
-- #endregion data

\echo
\echo '=== 2. Цена хранения: JSONB хранит имена ключей в каждой строке ==='
-- #region size
SELECT 'columns' AS layout,
       pg_size_pretty(pg_table_size('settings_columns')) AS table_size,
       round(avg(pg_column_size(c.*)))                   AS avg_row_bytes
FROM settings_columns c
UNION ALL
SELECT 'jsonb',
       pg_size_pretty(pg_table_size('settings_jsonb')),
       round(avg(pg_column_size(j.*)))
FROM settings_jsonb j;
-- #endregion size

\echo
\echo '=== 3. Поиск по содержимому без индекса — полный просмотр ==='
-- #region gin-before
EXPLAIN (COSTS OFF)
SELECT count(*) FROM settings_jsonb WHERE settings @> '{"beta": ["new_composer"]}';
-- #endregion gin-before

\echo
\echo '=== 4. GIN по JSONB: два класса операторов ==='
-- #region gin
-- jsonb_ops (по умолчанию) индексирует каждый ключ и значение — обслуживает @>, ?, ?|, ?&.
-- jsonb_path_ops индексирует хеши путей до значений — только @> (и jsonpath), зато меньше.
CREATE INDEX settings_gin ON settings_jsonb USING gin (settings);
CREATE INDEX settings_gin_path ON settings_jsonb USING gin (settings jsonb_path_ops);
ANALYZE settings_jsonb;

SELECT indexrelid::regclass AS index, pg_size_pretty(pg_relation_size(indexrelid)) AS size
FROM pg_index
WHERE indrelid = 'settings_jsonb'::regclass
ORDER BY indexrelid::regclass::text;

EXPLAIN (COSTS OFF)
SELECT count(*) FROM settings_jsonb WHERE settings @> '{"beta": ["new_composer"]}';

SELECT count(*) AS beta_users FROM settings_jsonb WHERE settings @> '{"beta": ["new_composer"]}';
-- #endregion gin

\echo
\echo '=== 5. ->> с «=» — не то, что умеет GIN ==='
-- #region gin-not-used
-- GIN знает операторы @>, ?, а не сравнение текста, извлечённого ->>. Такой запрос либо
-- переписывают через @>, либо строят B-tree по выражению (settings ->> 'language').
EXPLAIN (COSTS OFF)
SELECT count(*) FROM settings_jsonb WHERE settings ->> 'language' = 'kk';
-- #endregion gin-not-used
