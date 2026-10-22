-- Урок 2.2: аномалии вставки, обновления и удаления на «одной большой таблице» и их исчезновение
-- после нормализации до 3НФ; в конце — таблица в 3НФ, но не в НФБК.
-- Запуск из labs/: docker compose -f 2a/compose.yaml exec -T db sh -c 'psql -X -f /lab/anomalies.sql 2>&1'
-- Ошибки здесь намеренные: скрипт не останавливается на них (ON_ERROR_STOP не задан) и после каждой
-- печатает SQLSTATE — код, по которому ошибку различает программа.
\set QUIET on
SET client_min_messages = warning;
DROP SCHEMA IF EXISTS anomalies CASCADE;
CREATE SCHEMA anomalies;
SET search_path = anomalies;
RESET client_min_messages;
\set QUIET off
\set ECHO queries

\echo
\echo '=== 1. Одна большая таблица: сообщение вместе с данными автора и чата ==='
-- #region flat
CREATE TABLE messages_flat (
    message_id  bigint PRIMARY KEY,
    chat_id     bigint NOT NULL,
    chat_title  text   NOT NULL,
    author_id   bigint NOT NULL,
    author_name text   NOT NULL,
    text        text   NOT NULL
);

INSERT INTO messages_flat VALUES
    (1, 10, 'Бэкенд', 100, 'alice', 'Кто смотрел ревью?'),
    (2, 10, 'Бэкенд', 101, 'bob',   'Я, сейчас допишу'),
    (3, 10, 'Бэкенд', 100, 'alice', 'Спасибо!'),
    (4, 20, 'Обед',   100, 'alice', 'Идём в 13:00?');
-- #endregion flat

\echo
\echo '=== 2. Аномалия вставки: пользователя без сообщения сохранить негде ==='
-- #region insert-anomaly
-- carol зарегистрировалась, но ещё ничего не написала. Строки «пользователь» нет — есть только
-- строка «сообщение», и без сообщения её не вставить.
INSERT INTO messages_flat (message_id, chat_id, chat_title, author_id, author_name, text)
VALUES (NULL, NULL, NULL, 102, 'carol', NULL);
-- #endregion insert-anomaly
\echo SQLSTATE :SQLSTATE

\echo
\echo '=== 3. Аномалия обновления: имя хранится в стольких строках, сколько сообщений ==='
-- #region update-anomaly
-- alice переименовалась в alice_w. Правка «по одному сообщению» (как сделал бы код, который
-- обновляет только то, что сейчас показывает) оставляет у одного автора два имени.
UPDATE messages_flat SET author_name = 'alice_w' WHERE message_id = 1;

-- Функциональная зависимость author_id → author_name нарушена: у одного ключа два значения.
SELECT author_id, array_agg(DISTINCT author_name) AS names
FROM messages_flat
GROUP BY author_id
HAVING count(DISTINCT author_name) > 1;

-- Правильная правка трогает все сообщения автора — и чем больше он писал, тем она дороже.
UPDATE messages_flat SET author_name = 'alice_w' WHERE author_id = 100;
-- #endregion update-anomaly

\echo
\echo '=== 4. Аномалия удаления: с последним сообщением пропадает и пользователь ==='
-- #region delete-anomaly
-- bob написал одно сообщение. Удаляем его — и вместе с ним единственную запись о том,
-- что пользователь 101 зовётся bob.
DELETE FROM messages_flat WHERE message_id = 2;
SELECT count(*) AS rows_about_bob FROM messages_flat WHERE author_id = 101;
-- #endregion delete-anomaly

\echo
\echo '=== 5. 3НФ: каждый факт — в одной строке одной таблицы ==='
-- #region normalized
CREATE TABLE users (
    id   bigint PRIMARY KEY,
    name text   NOT NULL
);

CREATE TABLE chats (
    id    bigint PRIMARY KEY,
    title text   NOT NULL
);

CREATE TABLE messages (
    id        bigint PRIMARY KEY,
    chat_id   bigint NOT NULL REFERENCES chats (id),
    author_id bigint NOT NULL REFERENCES users (id),
    text      text   NOT NULL
);

INSERT INTO users VALUES (100, 'alice'), (101, 'bob');
INSERT INTO chats VALUES (10, 'Бэкенд'), (20, 'Обед');
INSERT INTO messages VALUES
    (1, 10, 100, 'Кто смотрел ревью?'),
    (2, 10, 101, 'Я, сейчас допишу'),
    (3, 10, 100, 'Спасибо!'),
    (4, 20, 100, 'Идём в 13:00?');
-- #endregion normalized

\echo
\echo '=== 6. Те же три правки после нормализации ==='
-- #region anomalies-gone
-- Вставка: пользователь существует сам по себе.
INSERT INTO users VALUES (102, 'carol');

-- Обновление: имя — одна строка, сколько бы сообщений ни было.
UPDATE users SET name = 'alice_w' WHERE id = 100;

-- Удаление: сообщение уходит, пользователь остаётся.
DELETE FROM messages WHERE id = 2;
SELECT id, name FROM users ORDER BY id;

-- «Большая таблица» никуда не делась — это запрос, а не хранилище.
SELECT m.id AS message_id, c.title AS chat_title, u.name AS author_name, m.text
FROM messages m
JOIN chats c ON c.id = m.chat_id
JOIN users u ON u.id = m.author_id
ORDER BY m.id;
-- #endregion anomalies-gone

\echo
\echo '=== 7. 3НФ, но не НФБК: дежурные поддержки ==='
-- #region bcnf
-- Правила: у клиента по каждой теме один дежурный — (client_id, topic) → agent;
-- дежурный ведёт ровно одну тему — agent → topic.
-- Ключи: (client_id, topic) и (client_id, agent). Все столбцы входят в какой-нибудь ключ,
-- поэтому 3НФ выполнена. НФБК — нет: agent определяет topic, но agent — не ключ.
CREATE TABLE support_duty (
    client_id bigint NOT NULL,
    topic     text   NOT NULL,
    agent     text   NOT NULL,
    PRIMARY KEY (client_id, topic),
    UNIQUE (client_id, agent)
);

INSERT INTO support_duty VALUES
    (1, 'оплата', 'dan'),
    (2, 'оплата', 'dan'),
    (1, 'доступ', 'eve');

-- Аномалия обновления: dan переходит на тему «доступ», а правят одну строку.
UPDATE support_duty SET topic = 'доступ' WHERE client_id = 2 AND agent = 'dan';

-- Зависимость agent → topic нарушена: у dan две темы.
SELECT agent, array_agg(DISTINCT topic ORDER BY topic) AS topics
FROM support_duty
GROUP BY agent
HAVING count(DISTINCT topic) > 1;
-- #endregion bcnf

\echo
\echo '=== 8. Разбиение до НФБК и его цена ==='
-- #region bcnf-split
CREATE TABLE agents (
    agent text PRIMARY KEY,
    topic text NOT NULL
);

CREATE TABLE client_agents (
    client_id bigint NOT NULL,
    agent     text   NOT NULL REFERENCES agents (agent),
    PRIMARY KEY (client_id, agent)
);

INSERT INTO agents VALUES ('dan', 'оплата'), ('eve', 'доступ'), ('fay', 'оплата');
INSERT INTO client_agents VALUES (1, 'dan'), (2, 'dan'), (1, 'eve');

-- Тема дежурного теперь одна строка: противоречие из шага 7 записать нельзя.
-- Цена: правило «у клиента по теме один дежурный» живёт в двух таблицах, и UNIQUE его
-- не выразит. Вторая дежурная по оплате у клиента 1 вставляется без ошибки:
INSERT INTO client_agents VALUES (1, 'fay');

SELECT ca.client_id, a.topic, array_agg(ca.agent ORDER BY ca.agent) AS agents
FROM client_agents ca
JOIN agents a USING (agent)
GROUP BY ca.client_id, a.topic
HAVING count(*) > 1;
-- #endregion bcnf-split
