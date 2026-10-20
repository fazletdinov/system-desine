#!/bin/bash
# Нагрузка чтением (урок 1.5.10): N параллельных клиентов заданное время читают историю
# одного чата (GET /v1/chats/{id}/messages) и в конце печатают сводку по кодам ответа.
# Регистрация упирается в Argon2id (scripts/overload.sh), а чтение — в JSON, JWT, замки
# хранилища и аллокации: вместе они дают профиль, где видно не только хеш пароля.
# Пользователей, токен, чат и сообщения скрипт создаёт сам через API.
# Запуск из каталога messenger при поднятом compose:
#   scripts/read-load.sh [N] [секунд] [адрес]
set -euo pipefail

n=${1:-8}
secs=${2:-30}
url=${3:-http://127.0.0.1:8080}
# Сообщений в чате — одна полная страница (limit=50 — умолчание API): каждый ответ
# сериализует 50 сообщений, а не пустой список.
messages=50
# Запросов на один вызов curl (одно соединение): процесс curl на каждый запрос сам
# по себе дороже ответа сервера и упёрся бы в fork, а не в мессенджер.
batch=100
password='достаточно длинный пароль'
# Приставка имён — 12 случайных шестнадцатеричных знаков: повторный прогон, даже в ту же
# секунду, не упрётся в 409 «имя занято» (имя — до 32 знаков a–z, 0–9 и _).
run=$(od -An -N6 -tx1 /dev/urandom | tr -d ' \n')
# mktemp -d создаёт каталог с правами 0700: файл с токеном в нём не читают другие пользователи.
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
# Заголовок с токеном curl читает из файла (-H @файл): в аргументах процесса токен
# увидел бы любой пользователь машины через ps.
auth="$dir/auth"

# call МЕТОД ПУТЬ [ТЕЛО] — запрос к API, печатает тело ответа. Подготовка идёт параллельно
# с overload.sh, и регистрация или вход могут получить 503 overloaded: ограничитель
# хешей просит повторить позже — повторяем раз в секунду, а не падаем.
call() {
	local method=$1 path=$2 status
	local args=(-s -o "$dir/resp" -w '%{http_code}' -X "$method" -H 'Content-Type: application/json')
	[ -n "$token" ] && args+=(-H @"$auth")
	[ $# -ge 3 ] && args+=(-d "$3")
	for _ in $(seq 1 30); do
		status=$(curl "${args[@]}" "$url$path") || status=000
		case $status in
		2??) cat "$dir/resp"; return 0 ;;
		503 | 000) sleep 1 ;;
		*) echo "read-load: $method $path — $status: $(cat "$dir/resp")" >&2; return 1 ;;
		esac
	done
	echo "read-load: $method $path — 30 попыток подряд $status" >&2
	return 1
}
# field ИМЯ — значение строкового поля из JSON на stdin (без jq: скрипту хватает curl).
# Поля нет — сообщение и ошибка: иначе set -e молча оборвал бы скрипт на пустом значении.
field() {
	local value
	value=$(grep -o "\"$1\":\"[^\"]*\"" | head -n1 | cut -d'"' -f4) || true
	[ -n "$value" ] || { echo "read-load: в ответе нет поля $1" >&2; return 1; }
	printf '%s\n' "$value"
}
uuid() { cat /proc/sys/kernel/random/uuid 2>/dev/null || uuidgen | tr '[:upper:]' '[:lower:]'; }

token=
signup() { call POST /v1/users "{\"username\":\"read${run}_$1\",\"display_name\":\"Чтение\",\"password\":\"$password\"}" | field id; }
reader=$(signup a)
peer=$(signup b)
token=$(call POST /v1/tokens "{\"username\":\"read${run}_a\",\"password\":\"$password\"}" | field access_token)
printf 'Authorization: Bearer %s\n' "$token" >"$auth"
chat=$(call POST /v1/chats "{\"kind\":\"direct\",\"member_ids\":[\"$peer\"]}" | field id)
for i in $(seq 1 "$messages"); do
	call POST "/v1/chats/$chat/messages" "{\"client_message_id\":\"$(uuid)\",\"text\":\"Сообщение $i для нагрузки чтением\"}" >/dev/null
done
echo "пользователь $reader, чат $chat, сообщений $messages; читаю: $n клиентов × $secs с"

export url auth chat secs dir batch
# xargs -P n запускает n клиентов сразу; каждый крутит curl до конца срока и пишет
# строки «код время» в свой файл — общий вывод перемешался бы. Один вызов curl делает
# batch запросов по одному keep-alive соединению: новое соединение на каждый запрос за
# 30 с исчерпало бы локальные порты (≈ 28 тыс., каждый закрытый ещё 60 с в TIME_WAIT),
# и хвост прогона вместо ответов сервера дал бы ошибки соединения. Оборванное соединение
# curl отмечает кодом 000; «|| true» — чтобы такой ответ попал в сводку, а не
# остановил клиента через set -e.
started=$(date +%s.%N)
seq 1 "$n" | xargs -P "$n" -I{} bash -c '
	urls=()
	for _ in $(seq 1 "$batch"); do urls+=(-o /dev/null "$url/v1/chats/$chat/messages"); done
	end=$((SECONDS + secs))
	while [ "$SECONDS" -lt "$end" ]; do
		curl -s -w "%{http_code} %{time_total}\n" -H @"$auth" "${urls[@]}" || true
	done >"$dir/{}.codes"'
# Клиент заканчивает только после последней пачки, поэтому прогон длится дольше заказанного:
# запросы в секунду считаем по фактическому времени, а не по secs.
elapsed=$(awk -v a="$started" -v b="$(date +%s.%N)" 'BEGIN { print b - a }')

# Сводка: всего запросов и в секунду (по фактическому времени прогона), затем по кодам ответа — сколько и время ответа
# (медиана, p99, максимум). sort раскладывает строки по коду и времени, поэтому i-я
# строка кода — i-е по возрастанию время.
total=$(cat "$dir"/*.codes | wc -l)
awk -v total="$total" -v secs="$elapsed" 'BEGIN { printf "запросов: %d за %.1f с, %.0f в секунду\n", total, secs, total / secs }'
cat "$dir"/*.codes | sort -k1,1 -k2,2n | awk '
	{ c[$1]++; t[$1, c[$1]] = $2 * 1000 }
	END {
		for (k in c) {
			i99 = int(c[k] * 0.99 + 0.5); if (i99 < 1) i99 = 1
			printf "%s %7d шт., медиана %6.1f мс, p99 %6.1f мс, максимум %6.1f мс\n", k, c[k], t[k, int((c[k] + 1) / 2)], t[k, i99], t[k, c[k]]
		}
	}' | sort
