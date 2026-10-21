#!/bin/bash
# Миграции мессенджера вживую (урок 2.8): status и up на пустой базе, повторный up,
# отказ сервера стартовать на устаревшей схеме, база новее бинаря, два migrate up
# одновременно и сбой миграции посередине (lock_timeout) с откатом DDL.
#
# Запуск из каталога messenger (нужны Docker, Go и jq):
#   scripts/migrate-demo.sh > ../labs/2a/results/messenger-migrate.txt
# Бинарь собирается из текущего кода, база — одноразовый контейнер с тем же образом,
# что в compose.yaml; скрипт убирает его за собой.
set -euo pipefail
export LC_ALL=C.UTF-8

dir=$(mktemp -d "$PWD/.migrate-demo-XXXXXX")
db=messenger-migrate-demo-$$
cleanup() {
	docker rm -f "$db" >/dev/null 2>&1 || true
	rm -rf "$dir"
}
trap cleanup EXIT

fail() {
	echo "ПРОВАЛ: $*" >&2
	exit 1
}

pg_image=$(sed -n 's/^ *image: \(postgres:.*\)$/\1/p' compose.yaml)
[ -n "$pg_image" ] || fail "в compose.yaml нет образа postgres"

echo "# Миграции мессенджера: status, up, отказ старта, два up сразу, сбой посередине (урок 2.8)"
echo "# $(date -u +%Y-%m-%dT%H:%MZ), $(go version | cut -d' ' -f3), Docker $(docker version -f '{{.Server.Version}}'), Linux $(uname -r)"
echo "# образ базы: $pg_image"
echo "# Воспроизвести: cd messenger && scripts/migrate-demo.sh > ../labs/2a/results/messenger-migrate.txt"
echo

go build -o "$dir/messenger" ./cmd/messenger
go run ./cmd/keygen -out "$dir/jwt.pem" >/dev/null
password=$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')

docker run -d --name "$db" -p 127.0.0.1::5432 \
	-e POSTGRES_USER=messenger -e POSTGRES_DB=messenger -e POSTGRES_PASSWORD="$password" \
	"$pg_image" >/dev/null
# По TCP, как healthcheck в compose: сокет отвечает ещё во время init-скриптов.
for _ in $(seq 1 60); do
	docker exec "$db" pg_isready -q -h 127.0.0.1 -U messenger -d messenger && break
	sleep 1
done
docker exec "$db" pg_isready -q -h 127.0.0.1 -U messenger -d messenger || fail "Postgres не поднялся"
port=$(docker port "$db" 5432/tcp | head -n 1 | sed 's/.*://')

# use_db NAME — строка подключения к базе NAME в файле, как секрет db_url в compose.
use_db() {
	printf 'postgres://messenger:%s@127.0.0.1:%s/%s?sslmode=disable\n' "$password" "$port" "$1" >"$dir/db_url"
}
export MESSENGER_DATABASE_URL_FILE=$dir/db_url
export MESSENGER_JWT_KEY_FILE=$dir/jwt.pem
export MESSENGER_ADDR=127.0.0.1:18081
export MESSENGER_DEBUG_ADDR=

psql() { docker exec -i "$db" psql -X -q -U messenger "$@"; }

# run CMD... — команда бинаря: печатает её, stdout и stderr вместе и код выхода.
run() {
	echo "\$ messenger $*"
	local code=0
	"$dir/messenger" "$@" 2>&1 || code=$?
	echo "код выхода: $code"
	echo
}

# serve — запуск сервера: если он поднялся (GET /readyz → 200), останавливаем SIGTERM;
# если вышел сам — печатаем его ошибку из журнала (JSON slog: поля msg и err).
serve() {
	echo "\$ messenger   # сервер"
	"$dir/messenger" >"$dir/server.log" 2>&1 &
	local pid=$! code=0 ready=""
	for _ in $(seq 1 50); do
		kill -0 "$pid" 2>/dev/null || break
		if [ "$(curl -s -o /dev/null -w '%{http_code}' "http://$MESSENGER_ADDR/readyz" || true)" = 200 ]; then
			ready=1
			break
		fi
		sleep 0.1
	done
	if [ -n "$ready" ]; then
		echo "GET /readyz → 200"
		kill -TERM "$pid"
	fi
	wait "$pid" || code=$?
	jq -r 'select(.level == "ERROR") | "\(.level) \(.msg): \(.err)"' "$dir/server.log"
	echo "код выхода: $code"
	echo
}

echo "## 1. Пустая база: сервер без миграций, status, up, повторный up"
use_db messenger
# region refuse
serve
run migrate status
run migrate up
run migrate up
run migrate status
# endregion refuse
echo "\$ psql -c 'SELECT id, version_id, is_applied FROM goose_db_version ORDER BY id'"
psql -c 'SELECT id, version_id, is_applied FROM goose_db_version ORDER BY id'
echo "\$ psql -c 'SELECT schemaname, count(*) AS tables FROM pg_tables …'"
psql -c "SELECT schemaname, count(*) AS tables FROM pg_tables WHERE schemaname IN ('users', 'auth', 'chat', 'platform') GROUP BY 1 ORDER BY 1"
serve

echo "## 2. База новее бинаря: применена миграция 2, которой бинарь не знает"
# Так выглядит база во время выкладки: migrate нового выпуска уже прошёл, а старые
# экземпляры ещё работают. Строку версии кладём руками — миграции 2 в мессенджере нет.
psql -c 'INSERT INTO goose_db_version (version_id, is_applied) VALUES (2, true)'
run migrate status
serve
psql -c 'DELETE FROM goose_db_version WHERE version_id = 2'

echo "## 3. Два migrate up одновременно на пустой базе"
psql -c 'CREATE DATABASE race'
use_db race
# timed NAME — migrate up в фоне; вывод и время работы (мс) — в $dir/NAME.log.
timed() {
	local start code=0
	start=$(date +%s%N)
	"$dir/messenger" migrate up >"$dir/$1.out" 2>&1 || code=$?
	{
		echo "--- процесс $1: код выхода $code, $((($(date +%s%N) - start) / 1000000)) мс"
		cat "$dir/$1.out"
	} >"$dir/$1.log"
}
# region race
timed a &
timed b &
wait
# endregion race
cat "$dir/a.log" "$dir/b.log"
psql -d race -c 'SELECT version_id, is_applied FROM goose_db_version ORDER BY id'

echo "## 3б. migrate up, пока блокировку goose держит другой сеанс"
# Сеанс psql берёт ту же advisory-блокировку, что goose (lock.DefaultLockID в v3.28.0),
# на 3 с — так выглядит для второго процесса долгая миграция первого.
psql -c 'CREATE DATABASE locked'
use_db locked
goose_lock_id=4097083626
psql -d locked -c "SELECT pg_advisory_lock($goose_lock_id)" -c 'SELECT pg_sleep(3)' \
	-c "SELECT pg_advisory_unlock($goose_lock_id)" >/dev/null &
holder=$!
for _ in $(seq 1 50); do
	held=$(psql -d locked -tA -c "SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND granted")
	[ "$held" != 0 ] && break
	sleep 0.1
done
[ "$held" != 0 ] || fail "сеанс не взял блокировку"
echo "другой сеанс держит блокировку goose $goose_lock_id"
timed c &
waiter=$!
sleep 1.5
exists() { psql -d locked -tA -c "SELECT CASE WHEN to_regclass('$1') IS NULL THEN 'нет' ELSE 'есть' END"; }
echo "через 1,5 с: таблица goose_db_version — $(exists goose_db_version), users.users — $(exists users.users)"
wait "$waiter"
wait "$holder"
cat "$dir/c.log"
echo

echo "## 4. Сбой миграции посередине: lock_timeout и откат DDL"
# Миграция — та же транзакция, которую goose открывает на файл: операторы файла, затем
# запись версии, затем COMMIT. Здесь её операторы выполняет psql, чтобы показать сбой на
# живой схеме мессенджера, не добавляя в него миграцию 00002.
# region lock-holder
# Долгий отчёт: обычный SELECT держит ACCESS SHARE на chat.messages до конца транзакции.
psql -d messenger -c 'BEGIN' -c 'SELECT count(*) AS report FROM chat.messages' \
	-c 'SELECT pg_sleep(6)' -c 'COMMIT' >"$dir/report.log" 2>&1 &
report=$!
# endregion lock-holder
for _ in $(seq 1 50); do
	held=$(psql -d messenger -tA -c "SELECT count(*) FROM pg_locks WHERE relation = 'chat.messages'::regclass AND mode = 'AccessShareLock' AND granted")
	[ "$held" != 0 ] && break
	sleep 0.1
done
[ "$held" != 0 ] || fail "отчёт не взял блокировку"
echo "отчёт держит ACCESS SHARE на chat.messages"
# region migration-tx
migration_tx() {
	docker exec -i "$db" psql -X -U messenger -d messenger -e <<'SQL'
BEGIN;
SET LOCAL lock_timeout = '2s';
CREATE TABLE chat.message_edits (message_id uuid NOT NULL, edited_at timestamptz NOT NULL);
ALTER TABLE chat.messages ADD COLUMN edited_at timestamptz;
\echo SQLSTATE :SQLSTATE
INSERT INTO goose_db_version (version_id, is_applied) VALUES (2, true);
COMMIT;
SQL
}
# endregion migration-tx
migration_tx >"$dir/tx.log" 2>&1 &
tx=$!
# Пока ALTER ждёт блокировку, приходит обычное чтение ленты — и встаёт в очередь за ним.
sleep 0.5
# region reader
# read_feed — чтение ленты одного чата; печатает, сколько миллисекунд оно заняло.
read_feed() {
	local start
	start=$(date +%s%N)
	psql -d messenger -tA -c "SELECT count(*) FROM chat.messages WHERE chat_id = '00000000-0000-0000-0000-000000000001'" >/dev/null
	echo $((($(date +%s%N) - start) / 1000000))
}
during=$(read_feed)
# endregion reader
wait "$tx" || true
cat "$dir/tx.log"
echo "чтение ленты, начатое через 0,5 с после ALTER: $during мс; то же чтение после отката: $(read_feed) мс"
echo
echo "после сбоя:"
psql -d messenger -c "SELECT to_regclass('chat.message_edits') AS edits_table,
	(SELECT count(*) FROM information_schema.columns
	  WHERE table_schema = 'chat' AND table_name = 'messages' AND column_name = 'edited_at') AS edited_at_columns,
	(SELECT max(version_id) FROM goose_db_version WHERE is_applied) AS schema_version"
wait "$report"
echo "отчёт закончился; повтор той же миграции:"
migration_tx 2>&1
psql -d messenger -c "SELECT to_regclass('chat.message_edits') AS edits_table,
	(SELECT max(version_id) FROM goose_db_version WHERE is_applied) AS schema_version"
echo "migrate-demo: отказ старта на пустой базе, up один раз, база новее бинаря допустима, из двух up сразу применяет один, up под чужой блокировкой ждёт, сбой откатывает DDL целиком"
