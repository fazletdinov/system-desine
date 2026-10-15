#!/bin/bash
# Smoke-тест образа: контейнер становится healthy, отвечает на пробы и API,
# корректно останавливается по SIGTERM. Используют check:images и CI (джоба image).
# Запуск из каталога messenger: scripts/smoke.sh <образ>
set -euo pipefail

image=${1:?нужно: scripts/smoke.sh <образ>}
port=${SMOKE_PORT:-18080}
name=messenger-smoke-$$
grace=30
# Ключ — во временном каталоге внутри проекта: Docker Desktop не видит /tmp хоста.
dir=$(mktemp -d "$PWD/.smoke-XXXXXX")
cleanup() {
	docker rm -f "$name" >/dev/null 2>&1 || true
	rm -rf "$dir"
}
trap cleanup EXIT

go run ./cmd/keygen -out "$dir/jwt.pem"
# Одноразовый ключ smoke-теста: на «голом» Docker Engine bind mount сохраняет владельца
# хоста, и процесс с UID 65532 не прочитал бы файл 0600 (урок 1.17).
chmod 0444 "$dir/jwt.pem"

docker run -d --name "$name" -p "127.0.0.1:$port:8080" \
	-v "$dir/jwt.pem:/run/secrets/jwt_key:ro" \
	-e MESSENGER_JWT_KEY_FILE=/run/secrets/jwt_key \
	--read-only --cap-drop ALL --security-opt no-new-privileges:true \
	"$image" >/dev/null

status=starting
for _ in $(seq 1 30); do
	status=$(docker inspect -f '{{.State.Health.Status}}' "$name")
	[ "$status" = healthy ] && break
	sleep 1
done
if [ "$status" != healthy ]; then
	docker logs "$name" >&2
	echo "ПРОВАЛ: контейнер не стал healthy (статус $status)" >&2
	exit 1
fi

curl -fsS "http://127.0.0.1:$port/readyz" >/dev/null
code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:$port/v1/users" \
	-H 'Content-Type: application/json' \
	-d '{"username":"smoke","display_name":"Smoke","password":"достаточно длинный пароль"}')
[ "$code" = 201 ] || { echo "ПРОВАЛ: POST /v1/users — $code" >&2; exit 1; }

# docker stop шлёт SIGTERM, ждёт и добивает SIGKILL. Срок задаём явно — 30 с, как
# terminationGrace в app.go (Kubernetes и наш compose): умолчание движка на него не
# похоже (Docker Desktop 29.7 добивал через ~3 с, на середине дренажа в 5 с).
docker stop -t "$grace" "$name" >/dev/null
exit_code=$(docker inspect -f '{{.State.ExitCode}}' "$name")
[ "$exit_code" = 0 ] || { docker logs "$name" >&2; echo "ПРОВАЛ: код выхода $exit_code" >&2; exit 1; }
docker logs "$name" 2>&1 | grep -q 'сервер остановлен' || { echo "ПРОВАЛ: нет записи об остановке" >&2; exit 1; }
echo "smoke: healthy, readyz, POST /v1/users 201, остановка по SIGTERM с кодом 0"
