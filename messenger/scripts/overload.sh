#!/bin/bash
# Наплыв регистраций (урок 1.5.6): N одновременных POST /v1/users и сводка по кодам
# ответа. Каждая регистрация — хеш Argon2id (19 МиБ, ≈ 17 мс), поэтому так проверяется
# ограничитель хешей: сверх MESSENGER_HASH_CONCURRENCY + MESSENGER_HASH_QUEUE сервер
# должен сразу отвечать 503 overloaded с Retry-After, а не копить горутины и память.
# Запуск из каталога messenger при поднятом compose: scripts/overload.sh [N] [адрес]
set -euo pipefail

n=${1:-50}
url=${2:-http://127.0.0.1:8080}
# Приставка имён — секунды запуска: повторный прогон не упрётся в 409 «имя занято».
run=$(date +%s)
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
export url run dir

# xargs -P n запускает все n curl сразу: это и есть «одновременно». Каждый curl пишет
# заголовки, тело и строку «код время» в свои файлы — общий вывод перемешался бы.
# Оборванное соединение (процесс убит) curl отмечает кодом 000; «|| true» — чтобы
# такой запрос попал в сводку, а не остановил скрипт через set -e.
seq 1 "$n" | xargs -P "$n" -I{} bash -c '
	curl -s -o "$dir/{}.body" -D "$dir/{}.head" -w "%{http_code} %{time_total}\n" \
		-X POST "$url/v1/users" -H "Content-Type: application/json" \
		-d "{\"username\":\"load${run}_{}\",\"display_name\":\"Нагрузка\",\"password\":\"достаточно длинный пароль\"}" \
		>"$dir/{}.code" || true'

echo "запросов: $n"
# Сводка: код ответа, сколько таких, время ответа (мин–макс), а для ошибок — поле code
# из problem+json и заголовок Retry-After.
for i in $(seq 1 "$n"); do
	read -r status secs <"$dir/$i.code"
	code=$(grep -so '"code":"[a-z_]*"' "$dir/$i.body" | cut -d'"' -f4 || true)
	retry=$(grep -si '^retry-after:' "$dir/$i.head" | tr -d '\r' | cut -d' ' -f2 || true)
	echo "$status ${code:--} ${retry:--} $secs"
done | sort | awk '
	{ k = $1 " code=" $2 " Retry-After=" $3; c[k]++
	  ms = $4 * 1000
	  if (!(k in lo) || ms < lo[k]) lo[k] = ms
	  if (ms > hi[k]) hi[k] = ms }
	END { for (k in c) printf "%-45s %3d шт., %4.0f–%4.0f мс\n", k, c[k], lo[k], hi[k] }' | sort
