-- Урок 2.6: настройки пользователя — столбцами и в JSONB. Что умеют запросы к JSONB, чего база
-- в JSONB не проверяет и как вернуть часть проверок через CHECK.
-- Запуск из labs/: docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/jsonb/settings.sql 2>&1'
\set QUIET on
SET client_min_messages = warning;
DROP SCHEMA IF EXISTS jsonb_settings CASCADE;
CREATE SCHEMA jsonb_settings;
SET search_path = jsonb_settings;
RESET client_min_messages;
\set QUIET off
\set ECHO queries

\echo
\echo '=== 1. Настройки столбцами: тип, умолчание и допустимые значения держит база ==='
-- #region columns
CREATE TABLE settings_columns (
    user_id         bigint  PRIMARY KEY,
    theme           text    NOT NULL DEFAULT 'system'
        CHECK (theme IN ('light', 'dark', 'system')),
    notify_mentions boolean NOT NULL DEFAULT true,
    language        text    NOT NULL DEFAULT 'ru'
);

INSERT INTO settings_columns (user_id, theme) VALUES (1, 'dark');
INSERT INTO settings_columns (user_id, theme) VALUES (2, 42);
-- #endregion columns
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 2. Настройки в JSONB: одна колонка, любая форма ==='
-- #region jsonb
CREATE TABLE settings_jsonb (
    user_id  bigint PRIMARY KEY,
    settings jsonb  NOT NULL DEFAULT '{}'
);

INSERT INTO settings_jsonb VALUES
    (1, '{"theme": "dark", "notify": {"mentions": true}, "language": "ru"}'),
    (2, '{"theme": "light", "notify": {"mentions": false}, "beta": ["new_composer"]}'),
    (3, '{}');

-- ->  возвращает jsonb, ->> — текст; путь вглубь — цепочкой или #>>.
-- Отсутствующий ключ — это NULL, а не ошибка: умолчание приходится писать в каждом запросе.
SELECT user_id,
       settings ->> 'theme'                          AS theme,
       settings -> 'notify' ->> 'mentions'           AS mentions,
       coalesce(settings ->> 'language', 'ru')       AS language
FROM settings_jsonb
ORDER BY user_id;

-- @> — «содержит»: у кого включена бета new_composer. ? — «есть ключ верхнего уровня».
SELECT user_id FROM settings_jsonb WHERE settings @> '{"beta": ["new_composer"]}';
SELECT user_id FROM settings_jsonb WHERE settings ? 'notify' ORDER BY user_id;
-- #endregion jsonb

\echo
\echo '=== 3. Частичное обновление ==='
-- #region jsonb-update
-- || сливает объекты верхнего уровня, jsonb_set меняет значение по пути.
UPDATE settings_jsonb SET settings = settings || '{"theme": "system"}' WHERE user_id = 1;
UPDATE settings_jsonb SET settings = jsonb_set(settings, '{notify,mentions}', 'false') WHERE user_id = 1;
SELECT settings FROM settings_jsonb WHERE user_id = 1;
-- #endregion jsonb-update

\echo
\echo '=== 4. Чего JSONB не проверяет ==='
-- #region jsonb-no-checks
-- Число вместо строки и опечатка в ключе — обе строки приняты.
INSERT INTO settings_jsonb VALUES (4, '{"theme": 42}'), (5, '{"thme": "dark"}');

-- Опечатку видно только по последствиям: у пользователя 5 тема «не задана».
SELECT user_id, settings ->> 'theme' AS theme FROM settings_jsonb WHERE user_id IN (4, 5) ORDER BY user_id;
-- #endregion jsonb-no-checks

\echo
\echo '=== 5. Проверки возвращаются через CHECK ==='
-- #region jsonb-checks
DELETE FROM settings_jsonb WHERE user_id IN (4, 5);

-- Корень — объект; ключи — только известные (jsonb - text[] убирает перечисленные ключи, должно
-- остаться пустое); theme, если есть, — одна из трёх строк.
ALTER TABLE settings_jsonb
    ADD CONSTRAINT settings_is_object
        CHECK (jsonb_typeof(settings) = 'object'),
    ADD CONSTRAINT settings_known_keys
        CHECK (settings - ARRAY['theme', 'notify', 'language', 'beta'] = '{}'::jsonb),
    ADD CONSTRAINT settings_theme_valid
        CHECK (settings -> 'theme' IS NULL
               OR settings -> 'theme' <@ '["light", "dark", "system"]'::jsonb);

INSERT INTO settings_jsonb VALUES (4, '{"theme": 42}');
-- #endregion jsonb-checks
\echo SQLSTATE :SQLSTATE
-- #region jsonb-checks-typo
INSERT INTO settings_jsonb VALUES (5, '{"thme": "dark"}');
-- #endregion jsonb-checks-typo
\echo SQLSTATE :SQLSTATE
