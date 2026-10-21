-- Начальная схема мессенджера: по схеме PostgreSQL на контекст (users, auth, chat) и
-- platform для технических таблиц. Таблицу видно по имени (chat.messages), права можно
-- выдавать на схему, а выделение контекста в сервис начинается с переноса одной схемы.
--
-- Имена ограничений заданы явно: адаптеры переводят ошибку 23505/23503/23514 в доменную
-- по pgconn.PgError.ConstraintName, и имя — часть контракта, а не то, что Postgres
-- придумает сам. Длины в CHECK — те же, что константы домена (user/domain, chat/domain):
-- база держит инвариант и тогда, когда строку пишет не наш код (ручной SQL, скрипт).
--
-- goose выполняет файл в одной транзакции вместе с записью версии в goose_db_version:
-- DDL в PostgreSQL транзакционен, и сбой посередине не оставит половины схемы.

-- +goose Up
-- #region schemas
CREATE SCHEMA users;
CREATE SCHEMA auth;
CREATE SCHEMA chat;
CREATE SCHEMA platform;
-- #endregion schemas

-- #region users
-- Имя уже приходит в нижнем регистре (user/domain.NewUsername пускает только a–z, 0–9
-- и «_»), поэтому обычного UNIQUE достаточно для уникальности без учёта регистра:
-- citext или индекс по lower(username) здесь не нужны (урок 2.3).
CREATE TABLE users.users (
    id           uuid        NOT NULL,
    username     text        NOT NULL,
    display_name text        NOT NULL,
    created_at   timestamptz NOT NULL,
    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT users_username_key UNIQUE (username),
    CONSTRAINT users_username_check
        CHECK (username ~ '^[a-z0-9_]+$' AND char_length(username) BETWEEN 3 AND 32),
    CONSTRAINT users_display_name_check
        CHECK (char_length(display_name) BETWEEN 1 AND 64)
);
-- #endregion users

-- #region credentials
-- Учётные данные — 1:1 с пользователем: первичный ключ и есть внешний. ON DELETE CASCADE:
-- без пользователя учётным данным незачем жить, а удалять их отдельным шагом — шанс
-- забыть. created_at домен не несёт (auth/domain.Credentials — только id и хеш), это
-- служебная отметка для разбора инцидентов, её ставит база.
CREATE TABLE auth.credentials (
    user_id       uuid        NOT NULL,
    password_hash text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT credentials_pkey PRIMARY KEY (user_id),
    CONSTRAINT credentials_user_id_fkey FOREIGN KEY (user_id)
        REFERENCES users.users (id) ON DELETE CASCADE,
    CONSTRAINT credentials_password_hash_check CHECK (password_hash <> '')
);
-- #endregion credentials

-- #region chats
-- Личный и групповой чат — одна таблица с kind: сообщения, участники и лента у них общие.
-- Разница — в парных CHECK: у личного чата есть direct_key и нет названия, у группы
-- наоборот. direct_key — отсортированная пара id участников (chat/domain.DirectKeyFor),
-- UNIQUE не даёт завести второй личный чат той же пары даже двум запросам наперегонки.
-- У групп direct_key NULL, а NULL в UNIQUE не конфликтуют друг с другом.
CREATE TABLE chat.chats (
    id               uuid        NOT NULL,
    kind             text        NOT NULL,
    title            text,
    direct_key       text,
    -- last_seq — номер последнего сообщения; новый выдаёт
    -- UPDATE … SET last_seq = last_seq + 1, version = version + 1 … WHERE id = $1 AND version = $2
    -- в одной транзакции с INSERT сообщения (урок 2.2).
    last_seq         bigint      NOT NULL DEFAULT 0,
    version          bigint      NOT NULL,
    created_at       timestamptz NOT NULL,
    last_activity_at timestamptz NOT NULL,
    CONSTRAINT chats_pkey PRIMARY KEY (id),
    CONSTRAINT chats_direct_key_key UNIQUE (direct_key),
    CONSTRAINT chats_kind_check CHECK (kind IN ('direct', 'group')),
    CONSTRAINT chats_direct_key_check CHECK ((kind = 'direct') = (direct_key IS NOT NULL)),
    CONSTRAINT chats_title_kind_check CHECK ((kind = 'group') = (title IS NOT NULL)),
    CONSTRAINT chats_title_check CHECK (char_length(title) BETWEEN 1 AND 128),
    CONSTRAINT chats_last_seq_check CHECK (last_seq >= 0),
    CONSTRAINT chats_version_check CHECK (version >= 0),
    -- До первого сообщения last_activity_at = created_at, дальше только растёт.
    CONSTRAINT chats_last_activity_check CHECK (last_activity_at >= created_at)
);
-- #endregion chats

-- #region members
-- Связь «многие ко многим» чатов и пользователей. Первичный ключ (chat_id, user_id)
-- запрещает повтор участника и сам служит индексом «участники чата» и «участник ли».
-- Внешний ключ на users.users — межсхемный: цена одной базы на все контексты (ADR-0014).
CREATE TABLE chat.members (
    chat_id uuid NOT NULL,
    user_id uuid NOT NULL,
    CONSTRAINT members_pkey PRIMARY KEY (chat_id, user_id),
    CONSTRAINT members_chat_id_fkey FOREIGN KEY (chat_id)
        REFERENCES chat.chats (id) ON DELETE CASCADE,
    CONSTRAINT members_user_id_fkey FOREIGN KEY (user_id)
        REFERENCES users.users (id)
);
-- #endregion members

-- #region messages
-- Первичный ключ (chat_id, seq), а не id: главный паттерн доступа — лента чата
-- «WHERE chat_id = $1 AND seq < $2 ORDER BY seq DESC», и она читается одним диапазоном
-- индекса (урок 2.5). id остаётся уникальным — по нему сообщение ищут напрямую.
-- client_message_id — естественный ключ повтора: клиент присваивает его до первой
-- попытки, и UNIQUE превращает повторную отправку в 23505, а не в дубль. Тип uuid, а не
-- text: домен и так требует UUID, база проверяет формат на границе, и 16 байт вместо ~37.
CREATE TABLE chat.messages (
    chat_id           uuid        NOT NULL,
    seq               bigint      NOT NULL,
    id                uuid        NOT NULL,
    author_id         uuid        NOT NULL,
    client_message_id uuid        NOT NULL,
    text              text        NOT NULL,
    sent_at           timestamptz NOT NULL,
    CONSTRAINT messages_pkey PRIMARY KEY (chat_id, seq),
    CONSTRAINT messages_id_key UNIQUE (id),
    CONSTRAINT messages_client_id_key UNIQUE (chat_id, author_id, client_message_id),
    CONSTRAINT messages_chat_id_fkey FOREIGN KEY (chat_id)
        REFERENCES chat.chats (id) ON DELETE CASCADE,
    CONSTRAINT messages_seq_check CHECK (seq > 0),
    CONSTRAINT messages_text_check CHECK (char_length(text) BETWEEN 1 AND 4096)
);
-- #endregion messages

-- #region members-user-index
-- Первичный ключ (chat_id, user_id) не помогает запросу «чаты пользователя»
-- (WHERE user_id = $1): user_id — второй столбец, и без своего индекса это полный
-- просмотр chat.members на каждом открытии приложения. Он же обслуживает список чатов
-- по активности: чаты пользователя — отсюда, их строки — по chats_pkey, порядок
-- (last_activity_at, id) — сортировкой его чатов. Индекса по chat.chats (last_activity_at)
-- нет намеренно: EXPLAIN (labs/2a/results/messenger-explain.txt) показал, что планировщик
-- его для списка не берёт, а индекс по столбцу, который меняет каждое сообщение, лишает
-- UPDATE строки чата HOT (урок 2.2).
CREATE INDEX members_user_id_idx ON chat.members (user_id);
-- #endregion members-user-index

-- #region idempotency-keys
-- Ключи идемпотентности (урок 1.5.4). Запись «в работе» (in_progress) держит
-- reserved_until: после него повтор перехватывает зависшую резервацию, а не получает
-- 409 до конца суток. owner — токен того, кто держит резервацию; новый при каждом
-- захвате. Complete и Release меняют строку только со своим owner: иначе запоздавший
-- прежний владелец стёр бы или перезаписал резервацию того, кто её перехватил.
-- Ответ (status, header, body) есть только у готовой записи (done).
CREATE TABLE platform.idempotency_keys (
    scope          text        NOT NULL,
    key            text        NOT NULL,
    fingerprint    bytea       NOT NULL,
    state          text        NOT NULL,
    owner          uuid        NOT NULL,
    status         integer,
    header         jsonb,
    body           bytea,
    reserved_until timestamptz NOT NULL,
    expires_at     timestamptz NOT NULL,
    CONSTRAINT idempotency_keys_pkey PRIMARY KEY (scope, key),
    -- 255 — idempotency.KeyMaxLen; отпечаток — SHA-256 запроса, ровно 32 байта.
    CONSTRAINT idempotency_keys_key_check CHECK (char_length(key) BETWEEN 1 AND 255),
    CONSTRAINT idempotency_keys_fingerprint_check CHECK (octet_length(fingerprint) = 32),
    CONSTRAINT idempotency_keys_state_check CHECK (state IN ('in_progress', 'done')),
    CONSTRAINT idempotency_keys_response_check CHECK ((state = 'done') = (status IS NOT NULL)),
    CONSTRAINT idempotency_keys_status_check CHECK (status BETWEEN 100 AND 599)
);
-- #endregion idempotency-keys

-- #region idempotency-expires-index
-- Фоновая очистка удаляет просроченные ключи порциями (WHERE expires_at <= $1); без
-- индекса каждый проход читал бы всю таблицу.
CREATE INDEX idempotency_keys_expires_at_idx ON platform.idempotency_keys (expires_at);
-- #endregion idempotency-expires-index

-- #region down
-- Откат — в обратном порядке зависимостей. Явные DROP TABLE, а не DROP SCHEMA … CASCADE:
-- CASCADE молча снёс бы и то, что в эти схемы добавили позже не этой миграцией.
-- +goose Down
DROP TABLE platform.idempotency_keys;
DROP TABLE chat.messages;
DROP TABLE chat.members;
DROP TABLE chat.chats;
DROP TABLE auth.credentials;
DROP TABLE users.users;
DROP SCHEMA platform;
DROP SCHEMA chat;
DROP SCHEMA auth;
DROP SCHEMA users;
-- #endregion down
