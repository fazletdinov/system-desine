-- Урок 2.7: антипаттерны Карвина на данных мессенджера. Для каждого — запрос, который ломается
-- (или ограничение, которое не выразить), и исправление.
-- Запуск из labs/: docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/antipatterns.sql 2>&1'
\set QUIET on
SET client_min_messages = warning;
DROP SCHEMA IF EXISTS antipatterns CASCADE;
CREATE SCHEMA antipatterns;
SET search_path = antipatterns;
RESET client_min_messages;
\set QUIET off
\set ECHO queries

CREATE TABLE users (id bigint PRIMARY KEY, username text NOT NULL UNIQUE);
INSERT INTO users VALUES (1, 'alice'), (2, 'bob'), (12, 'mallory');

\echo
\echo '=== 1. EAV: «гибкий» профиль без типов и ограничений ==='
-- #region eav
CREATE TABLE user_attrs (
    user_id bigint NOT NULL REFERENCES users (id),
    attr    text   NOT NULL,
    value   text,
    PRIMARY KEY (user_id, attr)
);

-- База принимает что угодно: дату, которая не дата, и опечатку в имени атрибута.
INSERT INTO user_attrs VALUES
    (1, 'birthday',     '1990-05-17'),
    (1, 'max_upload',   '9'),
    (2, 'birthday',     'вчера'),
    (2, 'max_upload',   '10'),
    (12, 'birth_day',   '1985-01-01');

-- Сортировка по «числу» — строковая: '10' < '9'.
SELECT user_id, value AS max_upload
FROM user_attrs
WHERE attr = 'max_upload'
ORDER BY value DESC;

-- Чтобы собрать строку «пользователь», нужен JOIN (или FILTER) на каждый атрибут, а приведение
-- типа падает на первом же мусорном значении.
SELECT u.username, b.value::date AS birthday
FROM users u
LEFT JOIN user_attrs b ON b.user_id = u.id AND b.attr = 'birthday';
-- #endregion eav
\echo SQLSTATE :SQLSTATE

-- #region eav-fix
-- Исправление: известные атрибуты — столбцы со своими типами и проверками.
CREATE TABLE profiles (
    user_id    bigint PRIMARY KEY REFERENCES users (id),
    birthday   date,
    max_upload int CHECK (max_upload > 0)
);

INSERT INTO profiles VALUES (2, 'вчера', 10);
-- #endregion eav-fix
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 2. Полиморфная связь: одна ссылка «то ли на сообщение, то ли на чат» ==='
-- #region polymorphic
CREATE TABLE chats    (id bigint PRIMARY KEY, title text NOT NULL);
CREATE TABLE messages (id bigint PRIMARY KEY, chat_id bigint NOT NULL REFERENCES chats (id), text text NOT NULL);
INSERT INTO chats    VALUES (10, 'Бэкенд');
INSERT INTO messages VALUES (100, 10, 'Привет');

-- target_id ссылается на разные таблицы в зависимости от target_type, поэтому FOREIGN KEY
-- объявить нельзя: внешний ключ знает только одну таблицу.
CREATE TABLE reports_poly (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    target_type text   NOT NULL CHECK (target_type IN ('message', 'chat')),
    target_id   bigint NOT NULL,
    reason      text   NOT NULL
);

-- Жалоба на сообщение, которого нет, — принята.
INSERT INTO reports_poly (target_type, target_id, reason) VALUES ('message', 999, 'спам');

-- Висячие ссылки приходится искать запросом — по одному на каждый тип цели.
SELECT r.id, r.target_type, r.target_id
FROM reports_poly r
WHERE r.target_type = 'message'
  AND NOT EXISTS (SELECT 1 FROM messages m WHERE m.id = r.target_id);
-- #endregion polymorphic

-- #region polymorphic-fix
-- Исправление: по столбцу-ссылке на каждый тип цели, у каждого — свой FOREIGN KEY;
-- CHECK требует ровно одну заполненную ссылку.
CREATE TABLE reports (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    message_id bigint REFERENCES messages (id),
    chat_id    bigint REFERENCES chats (id),
    reason     text   NOT NULL,
    CONSTRAINT reports_one_target CHECK (num_nonnulls(message_id, chat_id) = 1)
);

INSERT INTO reports (message_id, reason) VALUES (999, 'спам');
-- #endregion polymorphic-fix
\echo SQLSTATE :SQLSTATE
-- #region polymorphic-fix-check
INSERT INTO reports (message_id, chat_id, reason) VALUES (100, 10, 'спам');
-- #endregion polymorphic-fix-check
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 3. Список через запятую: участники чата в одной строке ==='
-- #region jaywalking
CREATE TABLE chats_csv (
    id         bigint PRIMARY KEY,
    title      text   NOT NULL,
    member_ids text   NOT NULL
);
INSERT INTO chats_csv VALUES
    (10, 'Бэкенд', '1,2'),
    (20, 'Обед',   '12'),
    (30, 'Архив',  '1,404');

-- «Чаты пользователя 2» через LIKE: ловит и 12 — чужой чат «Обед» попал в выдачу.
SELECT id, title FROM chats_csv WHERE member_ids LIKE '%2%' ORDER BY id;

-- Пользователя 404 не существует, а список его принял: ссылочной целостности у строки нет.
SELECT c.id, m.member_id
FROM chats_csv c, unnest(string_to_array(c.member_ids, ',')::bigint[]) AS m (member_id)
WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = m.member_id);
-- #endregion jaywalking

-- #region jaywalking-fix
-- Исправление: таблица-связка — строка на пару (чат, участник), FOREIGN KEY с обеих сторон.
CREATE TABLE members (
    chat_id bigint NOT NULL REFERENCES chats_csv (id),
    user_id bigint NOT NULL REFERENCES users (id),
    PRIMARY KEY (chat_id, user_id)
);
INSERT INTO members VALUES (10, 1), (10, 2), (20, 12), (30, 1);

SELECT chat_id FROM members WHERE user_id = 2;

INSERT INTO members VALUES (30, 404);
-- #endregion jaywalking-fix
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 4. Наивное дерево: ответы на сообщения через самосоединения ==='
-- #region naive-tree
CREATE TABLE replies (
    id        bigint PRIMARY KEY,
    parent_id bigint REFERENCES replies (id),
    text      text   NOT NULL
);
INSERT INTO replies VALUES
    (1, NULL, 'Релиз в пятницу?'),
    (2, 1,    'Лучше в четверг'),
    (3, 2,    'Почему?'),
    (4, 3,    'В пятницу некому чинить'),
    (5, 1,    'Согласен');

-- Ветка сообщения 1 «через два JOIN»: глубина зашита в запрос, ответ 4 (третий уровень) потерян.
SELECT r1.id AS level1, r2.id AS level2
FROM replies r1
LEFT JOIN replies r2 ON r2.parent_id = r1.id
WHERE r1.parent_id = 1
ORDER BY 1, 2;
-- #endregion naive-tree

-- #region naive-tree-fix
-- Исправление в PostgreSQL: та же таблица «родитель — потомок» и рекурсивный запрос — глубина
-- любая, и запрос от неё не зависит.
WITH RECURSIVE thread AS (
    SELECT id, parent_id, text, 1 AS depth
    FROM replies
    WHERE parent_id = 1
    UNION ALL
    SELECT r.id, r.parent_id, r.text, t.depth + 1
    FROM replies r
    JOIN thread t ON r.parent_id = t.id
)
SELECT id, depth, repeat('  ', depth - 1) || text AS text
FROM thread
ORDER BY depth, id;
-- #endregion naive-tree-fix

\echo
\echo '=== 5. Деньги во float ==='
-- #region float-money
-- double precision хранит двоичную дробь: 0,1 и 0,2 в ней не представимы точно.
SELECT 0.1::float8 + 0.2::float8 = 0.3::float8 AS float_equal,
       0.1::numeric + 0.2::numeric = 0.3::numeric AS numeric_equal,
       sum(0.1::float8) AS float_sum_of_ten,
       sum(0.1::numeric) AS numeric_sum_of_ten
FROM generate_series(1, 10);
-- #endregion float-money
