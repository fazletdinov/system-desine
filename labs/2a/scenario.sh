#!/bin/bash
# Сценарный прогон лабы 2a: поднимает PostgreSQL из 2a/compose.yaml, прогоняет SQL-скрипты и keys
# в коротком режиме (10⁵ строк) и проверяет ключевые строки вывода. Падает на первом расхождении;
# check:labs запускает его из каталога labs/. Числа зависят от машины, поэтому keys проверяется только
# относительно: UUIDv4 против UUIDv7 и bigint, а не абсолютными размерами и временем.
# Проверки написаны как «условие || fail»: fail завершает скрипт, так что «A && B || C» тут ветвление.
# shellcheck disable=SC2015
set -euo pipefail
# Вывод psql и keys — по-русски: без UTF-8-локали sed и grep режут кириллицу по байтам.
export LC_ALL=C.UTF-8

cd "$(dirname "$0")/.."

fail() {
	echo "ПРОВАЛ: $*" >&2
	exit 1
}

ok() {
	echo "ОК: $*"
}

# Своё имя проекта и свой порт: сценарий не трогает базу, которую ученик поднял руками (порт 55432).
export LAB2A_PORT=${LAB2A_SCENARIO_PORT:-55433}
compose=(docker compose -p lab2a-scenario -f 2a/compose.yaml)
bin=$(mktemp -d)
cleanup() {
	"${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
	rm -rf "$bin"
}
trap cleanup EXIT

# first_number <текст> <регулярка с одной группой (…)> — число из первого совпадения.
first_number() {
	printf '%s\n' "$1" | sed -nE "s/^.*$2.*$/\1/p" | head -n 1
}

# less <a> <b> — a < b для чисел с точкой.
less() { awk -v a="$1" -v b="$2" 'BEGIN { exit !(a < b) }'; }

# sql <файл> — прогнать скрипт лабы psql внутри контейнера; stderr слит в stdout там же,
# иначе ошибки и вывод запросов перемешались бы по пути через docker.
sql() {
	"${compose[@]}" exec -T db sh -c "psql -X -f /lab/$1 2>&1"
}

# has <вывод> <имя> <строка…> — каждая строка встречается в выводе как есть.
has() {
	local out=$1 name=$2
	shift 2
	for want in "$@"; do
		printf '%s\n' "$out" | grep -qF -- "$want" || fail "$name: в выводе нет «$want»:
$out"
	done
}

# errors_expected <вывод> <имя> <сколько> — ошибок ровно столько, сколько задумано, и после каждой
# скрипт напечатал её SQLSTATE; SQLSTATE 00000 значит, что задуманная ошибка не случилась.
errors_expected() {
	local out=$1 name=$2 want=$3 errors states
	errors=$(printf '%s\n' "$out" | grep -c 'ERROR:' || true)
	states=$(printf '%s\n' "$out" | grep -cE '^SQLSTATE [0-9A-Z]{5}$' || true)
	[ "$errors" -eq "$want" ] && [ "$states" -eq "$want" ] || fail "$name: ошибок $errors, строк SQLSTATE $states, ждали по $want:
$out"
	! printf '%s\n' "$out" | grep -q '^SQLSTATE 00000$' || fail "$name: задуманная ошибка не случилась (SQLSTATE 00000):
$out"
}

# --- 0. База ---
# --wait ждёт healthcheck (pg_isready); 60 с — с запасом на первый запуск initdb на раннере.
"${compose[@]}" up -d --wait --wait-timeout 60 >/dev/null 2>&1 || fail "PostgreSQL не поднялся:
$("${compose[@]}" logs 2>&1 | tail -n 30)"
version=$("${compose[@]}" exec -T db psql -X -tAc 'SHOW server_version')
case "$version" in
18.6*) ok "PostgreSQL $version" ;;
*) fail "ждали PostgreSQL 18.6, получили $version" ;;
esac

# --- 1. anomalies.sql: три аномалии и их исчезновение (урок 2.2) ---
out=$(sql anomalies.sql)
errors_expected "$out" anomalies.sql 1
has "$out" anomalies.sql 'SQLSTATE 23502' '{alice,alice_w}' 'UPDATE 3' '{доступ,оплата}' '{dan,fay}'
# После нормализации переименование — одна строка, а bob переживает удаление своего сообщения.
printf '%s\n' "$out" | sed -n '/=== 6/,/=== 7/p' | grep -qE '^ 101 \| bob *$' || fail "anomalies.sql: после нормализации bob пропал:
$out"
ok "anomalies.sql: вставка — 23502, обновление — два имени у автора, удаление — bob пропал; после 3НФ всё на месте"

# --- 2. constraints.sql: каждое ограничение и его SQLSTATE (урок 2.4) ---
out=$(sql constraints.sql)
errors_expected "$out" constraints.sql 12
has "$out" constraints.sql 'SQLSTATE 23502' 'SQLSTATE 23514' 'SQLSTATE 23503' 'SQLSTATE 23P01' 'SQLSTATE 0A000' \
	'ERROR:  23505: duplicate key value violates unique constraint "users_username_key"' \
	'CONSTRAINT NAME:  users_username_key' \
	'violates unique constraint "emails_address_active_key"' \
	'violates exclusion constraint "mutes_no_overlap"' \
	'CHECK constraints cannot be marked DEFERRABLE'
# Отложенный FK: чат 3 вставлен после участника в той же транзакции — COMMIT прошёл.
printf '%s\n' "$out" | grep -qE '^ +3 \| +1$' || fail "constraints.sql: отложенный FK не пропустил участника чата 3:
$out"
ok "constraints.sql: 23502, 23514, 23505, 23503, 23P01 и 0A000; отложенный FK проверяется в COMMIT"

# --- 3. antipatterns.sql: запрос, который ломается, и исправление (урок 2.7) ---
out=$(sql antipatterns.sql)
errors_expected "$out" antipatterns.sql 5
has "$out" antipatterns.sql 'SQLSTATE 22007' 'SQLSTATE 23503' 'SQLSTATE 23514' ' 20 | Обед' '(4 rows)'
# Строка результата WITH RECURSIVE (id 4 на глубине 3) — в эхо запросов её нет, только в выводе.
printf '%s\n' "$out" | grep -qE '^ +4 \| +3 \| +В пятницу некому чинить$' || fail "antipatterns.sql: рекурсивный запрос не дошёл до третьего уровня:
$out"
printf '%s\n' "$out" | grep -qE '^ f +\| t +\|' || fail "antipatterns.sql: float и numeric сравнились одинаково:
$out"
ok "antipatterns.sql: EAV, полиморфная связь, список через запятую, наивное дерево и float — сломались и исправлены"

# --- 4. jsonb/: настройки в JSONB и GIN (урок 2.6) ---
out=$(sql jsonb/settings.sql)
errors_expected "$out" jsonb/settings.sql 5
has "$out" jsonb/settings.sql 'SQLSTATE 22P02' 'settings_theme_valid' 'settings_known_keys' \
	'Failing row contains (6, {"theme": ["dark", "light"]})' '{"theme": "system", "notify": {"mentions": false}, "language": "ru"}'
out=$(sql jsonb/gin.sql)
errors_expected "$out" jsonb/gin.sql 0
has "$out" jsonb/gin.sql 'Bitmap Index Scan on settings_gin_path' 'Filter: ((settings ->> '\''language'\''::text) = '\''kk'\''::text)'
row_columns=$(first_number "$out" ' columns +\| [^|]+\| +([0-9]+)')
row_jsonb=$(first_number "$out" ' jsonb +\| [^|]+\| +([0-9]+)')
[ -n "$row_columns" ] && [ -n "$row_jsonb" ] && less "$row_columns" "$row_jsonb" || fail "jsonb/gin.sql: строка столбцами $row_columns байт, в JSONB $row_jsonb, ждали JSONB больше:
$out"
ok "jsonb: столбец boolean отвергает строку (22P02), CHECK ловит число, массив и опечатку в ключе; строка $row_columns байт столбцами против $row_jsonb в JSONB; @> идёт по GIN"

# --- 5. keys: bigint, UUIDv4, UUIDv7 (урок 2.3) ---
go build -o "$bin/keys" ./2a/keys
out=$("$bin/keys" -n 100000 -dsn "postgres://lab:lab@127.0.0.1:$LAB2A_PORT/lab?sslmode=disable") || fail "keys упал:
$out"
v4_v7_index=$(first_number "$out" 'uuidv4 к uuidv7: индекс ×([0-9.]+)')
v4_v7_wal=$(first_number "$out" 'uuidv4 к uuidv7: .*WAL дозагрузки ×([0-9.]+)')
v7_bigint_index=$(first_number "$out" 'uuidv7 к bigint: индекс ×([0-9.]+)')
# Относительные проверки: v4 разбрасывает вставки по всему индексу — листья заполнены на ~2/3 и
# после контрольной точки почти каждый лист пишет в WAL полный образ; v7 и bigint пишут в правый край.
[ -n "$v4_v7_index" ] && less 1.1 "$v4_v7_index" || fail "keys: индекс v4 к v7 ×$v4_v7_index, ждали больше 1.1:
$out"
[ -n "$v4_v7_wal" ] && less 1.5 "$v4_v7_wal" || fail "keys: WAL дозагрузки v4 к v7 ×$v4_v7_wal, ждали больше 1.5:
$out"
[ -n "$v7_bigint_index" ] && less 1 "$v7_bigint_index" || fail "keys: индекс v7 к bigint ×$v7_bigint_index, ждали больше 1 (ключ вдвое шире):
$out"
ok "keys: индекс v4 больше v7 в $v4_v7_index раза, WAL дозагрузки — в $v4_v7_wal; индекс v7 больше bigint в $v7_bigint_index"

echo "Лаба 2a: все проверки прошли."
