#!/bin/bash
# Smoke-тест мессенджера на Postgres: контейнер становится healthy, проходит сценарий
# API (регистрация, вход, чат, сообщение, лента) и останавливается по SIGTERM с кодом 0
# в правильном порядке: HTTP → фоновые компоненты → пул базы.
#
# Запуск из каталога messenger:
#   scripts/smoke.sh <образ>   — образ с одноразовым Postgres рядом (check:images, CI-джоба image);
#   scripts/smoke.sh --compose — стек compose.yaml: up --build, сценарий, restart (данные
#                                на месте), stop. Нужны секреты из README.md в корне («В Docker»);
#                                стек остаётся остановленным, том базы — с данными.
set -euo pipefail

port=${SMOKE_PORT:-18080}
grace=30
# Каталог — внутри проекта: Docker Desktop не видит /tmp хоста.
dir=$(mktemp -d "$PWD/.smoke-XXXXXX")
# Имена уникальны на каждый прогон: в томе compose остаются пользователи прошлых.
suffix=$(date +%s)$$
alice=smoke_a_$suffix
bob=smoke_b_$suffix
password="достаточно длинный пароль"
text="привет из smoke $suffix"

fail() {
	echo "ПРОВАЛ: $*" >&2
	exit 1
}

uuid() { cat /proc/sys/kernel/random/uuid 2>/dev/null || uuidgen | tr '[:upper:]' '[:lower:]'; }

# api METHOD PATH TOKEN [BODY] — запрос к API; код ответа — в $code, тело — в $dir/body.
api() {
	local args=(-sS -o "$dir/body" -w '%{http_code}' -X "$1" "$base$2")
	[ -n "$3" ] && args+=(-H "Authorization: Bearer $3")
	[ $# -ge 4 ] && args+=(-H 'Content-Type: application/json' -d "$4")
	code=$(curl "${args[@]}")
}

# expect CODE WHAT — код последнего ответа; печатает шаг сценария.
expect() {
	[ "$code" = "$1" ] || fail "$2: $code, ждали $1; тело $(cat "$dir/body")"
	echo "  $2 → $code"
}

field() { jq -er "$1" "$dir/body"; }

# wait_healthy CONTAINER — ждёт HEALTHCHECK образа (он стучится в /readyz, а тот — в базу).
wait_healthy() {
	local status=starting
	for _ in $(seq 1 60); do
		status=$(docker inspect -f '{{.State.Health.Status}}' "$1")
		[ "$status" = healthy ] && return 0
		sleep 1
	done
	docker logs "$1" >&2
	fail "контейнер не стал healthy (статус $status)"
}

# region scenario
# scenario — запись: два пользователя, вход, личный чат, сообщение, лента глазами Боба.
scenario() {
	echo "сценарий:"
	api GET /readyz ""
	expect 200 "GET /readyz"
	api POST /v1/users "" '{"username":"'"$alice"'","display_name":"Алиса","password":"'"$password"'"}'
	expect 201 "POST /v1/users (алиса)"
	api POST /v1/users "" '{"username":"'"$bob"'","display_name":"Боб","password":"'"$password"'"}'
	expect 201 "POST /v1/users (боб)"
	bob_id=$(field .id)
	login "$alice"
	alice_token=$token
	api POST /v1/chats "$alice_token" '{"kind":"direct","member_ids":["'"$bob_id"'"]}'
	expect 201 "POST /v1/chats (личный)"
	chat_id=$(field .id)
	api POST "/v1/chats/$chat_id/messages" "$alice_token" '{"client_message_id":"'"$(uuid)"'","text":"'"$text"'"}'
	expect 201 "POST /v1/chats/{id}/messages"
	login "$bob"
	check_feed "$token"
	api GET /v1/chats "$token"
	expect 200 "GET /v1/chats (боб)"
	[ "$(field '.items[0].id')" = "$chat_id" ] || fail "в списке чатов Боба нет чата $chat_id"
}

# login USER — вход; токен — в $token.
login() {
	api POST /v1/tokens "" '{"username":"'"$1"'","password":"'"$password"'"}'
	expect 200 "POST /v1/tokens ($1)"
	token=$(field .access_token)
}

# check_feed TOKEN — в ленте чата ровно одно сообщение, то самое.
check_feed() {
	api GET "/v1/chats/$chat_id/messages" "$1"
	expect 200 "GET /v1/chats/{id}/messages"
	if [ "$(field '.items | length')" != 1 ] || [ "$(field '.items[0].text')" != "$text" ]; then
		fail "лента: ждали одно сообщение «$text», получили $(cat "$dir/body")"
	fi
	echo "  лента: 1 сообщение, seq $(field '.items[0].seq'), «$(field '.items[0].text')»"
}

# endregion scenario

# check_stop CONTAINER — код выхода 0 и порядок остановки в журнале.
check_stop() {
	local exit_code
	exit_code=$(docker inspect -f '{{.State.ExitCode}}' "$1")
	[ "$exit_code" = 0 ] || { docker logs "$1" >&2; fail "код выхода $exit_code"; }
	# Только последний запуск: после restart в журнале контейнера два запуска и две остановки.
	docker logs "$1" 2>&1 | jq -Rr 'fromjson? | .msg' |
		awk '/^слушаю$/ { buf = "" } { buf = buf $0 "\n" } END { printf "%s", buf }' >"$dir/stop.log"
	local prev=0 n
	echo "остановка (код выхода $exit_code), журнал по порядку:"
	for line in "сервер остановлен" "фоновые компоненты остановлены" "пул закрыт"; do
		n=$(grep -n -m1 -F "$line" "$dir/stop.log" | cut -d: -f1)
		if [ -z "$n" ] || [ "$n" -le "$prev" ]; then
			fail "в журнале нет «$line» после предыдущего шага остановки"
		fi
		echo "  $line"
		prev=$n
	done
}

standalone() {
	local image=$1
	# Не local: их читает trap на выходе из скрипта, когда функции уже нет.
	name=messenger-smoke-$$
	net=$name
	db=$name-db
	cleanup() {
		docker rm -f "$name" "$db" >/dev/null 2>&1 || true
		docker network rm "$net" >/dev/null 2>&1 || true
		rm -rf "$dir"
	}
	trap cleanup EXIT
	# Образ базы — тот же, что в compose.yaml (одна строка на весь проект).
	local pg_image
	pg_image=$(sed -n 's/^ *image: \(postgres:.*\)$/\1/p' compose.yaml)
	[ -n "$pg_image" ] || fail "в compose.yaml нет образа postgres"

	go run ./cmd/keygen -out "$dir/jwt.pem"
	head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n' >"$dir/db_password"
	printf 'postgres://messenger:%s@%s:5432/messenger?sslmode=disable\n' "$(cat "$dir/db_password")" "$db" >"$dir/db_url"
	# Одноразовые секреты smoke-теста: на «голом» Docker Engine bind mount сохраняет
	# владельца хоста, и процесс с UID 65532 не прочитал бы файл 0600 (урок 1.17).
	chmod 0444 "$dir/jwt.pem" "$dir/db_password" "$dir/db_url"

	docker network create "$net" >/dev/null
	docker run -d --name "$db" --network "$net" \
		-v "$dir/db_password:/run/secrets/db_password:ro" \
		-e POSTGRES_USER=messenger -e POSTGRES_DB=messenger \
		-e POSTGRES_PASSWORD_FILE=/run/secrets/db_password \
		"$pg_image" >/dev/null
	# По TCP, как healthcheck в compose: сокет отвечает ещё во время init-скриптов.
	for _ in $(seq 1 60); do
		docker exec "$db" pg_isready -q -h 127.0.0.1 -U messenger -d messenger && break
		sleep 1
	done
	docker exec "$db" pg_isready -q -h 127.0.0.1 -U messenger -d messenger || fail "Postgres не поднялся"

	local secrets=(-v "$dir/db_url:/run/secrets/db_url:ro" -e MESSENGER_DATABASE_URL_FILE=/run/secrets/db_url)
	local hardening=(--read-only --cap-drop ALL --security-opt no-new-privileges:true)
	docker run --rm --network "$net" "${secrets[@]}" "${hardening[@]}" "$image" migrate up >/dev/null ||
		fail "migrate up"
	docker run -d --name "$name" --network "$net" -p "127.0.0.1:$port:8080" "${secrets[@]}" \
		-v "$dir/jwt.pem:/run/secrets/jwt_key:ro" -e MESSENGER_JWT_KEY_FILE=/run/secrets/jwt_key \
		"${hardening[@]}" "$image" >/dev/null
	wait_healthy "$name"
	base=http://127.0.0.1:$port
	scenario

	# docker stop шлёт SIGTERM, ждёт и добивает SIGKILL. Срок задаём явно — 30 с, как
	# terminationGrace в app.go (Kubernetes и наш compose): умолчание движка на него не
	# похоже (Docker Desktop 29.7 добивал через ~3 с, на середине дренажа в 5 с).
	docker stop -t "$grace" "$name" >/dev/null
	check_stop "$name"
	echo "smoke: healthy, сценарий API на Postgres, остановка по SIGTERM с кодом 0 и в порядке HTTP → фон → пул"
}

with_compose() {
	trap 'rm -rf "$dir"' EXIT
	for f in jwt.pem secrets/db_password secrets/db_url; do
		[ -f "$f" ] || fail "нет $f — создайте секреты по README.md в корне репозитория, раздел «В Docker»"
	done
	echo "\$ docker compose up -d --build"
	docker compose up -d --build --quiet-pull 2>&1 | tail -n 3
	local id
	id=$(docker compose ps -q messenger)
	wait_healthy "$id"
	echo "миграции: $(docker compose logs --no-log-prefix migrate | tail -n 1)"
	base=http://127.0.0.1:8080
	scenario

	echo "\$ docker compose restart messenger"
	docker compose restart messenger 2>&1 | tail -n 1
	wait_healthy "$id"
	echo "после перезапуска:"
	login "$alice"
	check_feed "$token"

	echo "\$ docker compose stop"
	docker compose stop 2>&1
	check_stop "$id"
	echo "smoke (compose): сценарий API, данные пережили перезапуск, остановка с кодом 0 в порядке HTTP → фон → пул"
}

case ${1:?нужно: scripts/smoke.sh <образ> | --compose} in
--compose) with_compose ;;
*) standalone "$1" ;;
esac
