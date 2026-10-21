-- Урок 2.3: типы, которые выбирают «по привычке», и что из этого выходит. Ошибки намеренные, после
-- каждой печатается SQLSTATE.
-- Запуск из labs/: docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/types.sql 2>&1'
\set QUIET on
SET client_min_messages = warning;
DROP SCHEMA IF EXISTS types CASCADE;
CREATE SCHEMA types;
SET search_path = types, public;
-- citext — расширение из contrib, в официальном образе оно есть.
CREATE EXTENSION IF NOT EXISTS citext SCHEMA public;
RESET client_min_messages;
\set QUIET off
\set ECHO queries

\echo
\echo '=== 1. timestamp против timestamptz: одна и та же вставка, две сессии в разных часовых поясах ==='
-- #region timestamptz
CREATE TABLE events (
    at_naive timestamp,   -- без зоны: хранит «цифры на часах»
    at_tz    timestamptz  -- с зоной: хранит момент времени (внутри — UTC)
);

-- Сервер приложения в Москве записывает момент 12:00 UTC в оба столбца.
SET timezone = 'Europe/Moscow';
INSERT INTO events VALUES ('2026-10-21 12:00:00+00', '2026-10-21 12:00:00+00');
SELECT at_naive, at_tz FROM events;

-- Тот же ряд читает сессия в UTC (другой сервер, psql админа, задача аналитики).
SET timezone = 'UTC';
SELECT at_naive, at_tz FROM events;

-- Один ли это момент? Ответ для timestamp зависит от того, кто спрашивает.
SELECT at_naive::timestamptz = at_tz AS same_moment FROM events;
SET timezone = 'Europe/Moscow';
SELECT at_naive::timestamptz = at_tz AS same_moment FROM events;
SET timezone = 'UTC';

SELECT pg_column_size(at_naive) AS naive_bytes, pg_column_size(at_tz) AS tz_bytes FROM events;
-- #endregion timestamptz

\echo
\echo '=== 2. varchar(n) против text + CHECK: обе отвергают длинную строку, но по-разному ==='
-- #region varchar
CREATE TABLE titles_varchar (title varchar(5));
INSERT INTO titles_varchar VALUES ('слишком длинно');
-- #endregion varchar
\echo SQLSTATE :SQLSTATE
-- #region text-check
CREATE TABLE titles_text (
    title text CONSTRAINT titles_text_title_length CHECK (char_length(title) <= 5)
);
INSERT INTO titles_text VALUES ('слишком длинно');
-- #endregion text-check
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 3. enum против text + CHECK: в ошибке enum нет имени ограничения ==='
-- #region enum
CREATE TYPE chat_kind AS ENUM ('direct', 'group');
CREATE TABLE chats_enum (kind chat_kind NOT NULL);
INSERT INTO chats_enum VALUES ('channel');
-- #endregion enum
\echo SQLSTATE :SQLSTATE
-- #region kind-check
CREATE TABLE chats_check (
    kind text NOT NULL CONSTRAINT chats_check_kind CHECK (kind IN ('direct', 'group'))
);
INSERT INTO chats_check VALUES ('channel');
-- #endregion kind-check
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 4. Что лежит внутри UUID: версия, время создания, размер ==='
-- #region uuid-inside
-- UUIDv7 из примера документации PostgreSQL 18 (функции UUID). Время создания читает любой,
-- у кого есть id: ни ключа, ни доступа к базе для этого не нужно.
SELECT uuid_extract_version('019535d9-3df7-79fb-b466-fa907fa17f9e') AS version,
       uuid_extract_timestamp('019535d9-3df7-79fb-b466-fa907fa17f9e') AS created_at;
SELECT uuid_extract_version(gen_random_uuid()) AS v4_version,
       uuid_extract_timestamp(gen_random_uuid()) AS v4_created_at;

-- Тип uuid — 16 байт; тот же id текстом — 36 символов и заголовок.
SELECT pg_column_size('019535d9-3df7-79fb-b466-fa907fa17f9e'::uuid) AS uuid_bytes,
       pg_column_size('019535d9-3df7-79fb-b466-fa907fa17f9e'::text) AS text_bytes,
       pg_column_size(1::bigint) AS bigint_bytes;
-- #endregion uuid-inside

\echo
\echo '=== 5. Имя без учёта регистра: citext ==='
-- #region citext
CREATE TABLE users_citext (username citext CONSTRAINT users_citext_username_key UNIQUE);
INSERT INTO users_citext VALUES ('Alice');
INSERT INTO users_citext VALUES ('alice');
-- #endregion citext
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 6. Имя без учёта регистра: недетерминированная ICU-коллация ==='
-- #region icu-collation
-- und-u-ks-level2: сравнение на втором уровне силы ICU — регистр не различается.
CREATE COLLATION case_insensitive (provider = icu, locale = 'und-u-ks-level2', deterministic = false);
CREATE TABLE users_ci (
    username text COLLATE case_insensitive CONSTRAINT users_ci_username_key UNIQUE
);
INSERT INTO users_ci VALUES ('Alice');
INSERT INTO users_ci VALUES ('alice');
-- #endregion icu-collation
\echo SQLSTATE :SQLSTATE
-- #region icu-like
-- Равенство и LIKE (с PostgreSQL 18) работают без учёта регистра.
SELECT username FROM users_ci WHERE username = 'ALICE';
SELECT username FROM users_ci WHERE username LIKE 'ali%';
-- #endregion icu-like
-- #region icu-limits
-- А регулярные выражения и ILIKE с такой коллацией не работают вовсе. Коварнее всего — в CHECK:
-- таблица создаётся без ошибок, а падает каждая вставка.
CREATE TABLE users_ci_checked (
    username text COLLATE case_insensitive
        CONSTRAINT users_ci_checked_username_check CHECK (username ~ '^[a-z0-9_]+$')
);
INSERT INTO users_ci_checked VALUES ('alice');
-- #endregion icu-limits
\echo SQLSTATE :SQLSTATE
SELECT username FROM users_ci WHERE username ILIKE 'ali%';
\echo SQLSTATE :SQLSTATE
-- #region icu-fk
-- PostgreSQL 18: внешний ключ требует у обоих концов одну и ту же коллацию, если хоть одна
-- из них недетерминированная.
CREATE TABLE mentions (username text REFERENCES users_ci (username));
-- #endregion icu-fk
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 7. lower против casefold (PostgreSQL 18) ==='
-- #region casefold
-- База лабы создана с провайдером libc (en_US.utf8); pg_unicode_fast — встроенная коллация 18.
SELECT lower('Straße') AS lower_libc,
       lower('STRASSE') AS lower_upper,
       casefold('Straße') AS casefold_libc,
       casefold('Straße' COLLATE pg_unicode_fast) AS casefold_unicode,
       casefold('Straße' COLLATE pg_unicode_fast) = casefold('STRASSE' COLLATE pg_unicode_fast) AS same_name;
-- #endregion casefold
-- #region casefold-unique
-- Уникальный индекс по casefold — более точная замена индекса по lower(): ß и SS совпадут.
CREATE TABLE users_folded (username text NOT NULL);
CREATE UNIQUE INDEX users_folded_username_key ON users_folded (casefold(username COLLATE pg_unicode_fast));
INSERT INTO users_folded VALUES ('Straße');
INSERT INTO users_folded VALUES ('STRASSE');
-- #endregion casefold-unique
\echo SQLSTATE :SQLSTATE
