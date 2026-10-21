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

# column_value <вывод> <столбец> — значение под заголовком столбца из однострочного результата.
column_value() {
	printf '%s\n' "$1" | sed -n "/^ $2 \$/{n;n;p;q;}" | tr -d ' '
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
errors_expected "$out" constraints.sql 16
has "$out" constraints.sql 'SQLSTATE 23502' 'SQLSTATE 23514' 'SQLSTATE 23503' 'SQLSTATE 23P01' 'SQLSTATE 0A000' 'SQLSTATE 25P02' 'SQLSTATE 55000' \
	'current transaction is aborted, commands ignored until end of transaction block' \
	'ERROR:  23505: duplicate key value violates unique constraint "users_username_key"' \
	'CONSTRAINT NAME:  users_username_key' \
	'violates unique constraint "emails_address_active_key"' \
	'violates exclusion constraint "mutes_no_overlap"' \
	'violates exclusion constraint "mutes_wo_key"' \
	'ON CONFLICT does not support deferrable unique constraints/exclusion constraints as arbiters' \
	'CHECK constraints cannot be marked DEFERRABLE'
# Отложенный FK: чат 3 вставлен после участника в той же транзакции — COMMIT прошёл.
printf '%s\n' "$out" | grep -qE '^ +3 \| +1$' || fail "constraints.sql: отложенный FK не пропустил участника чата 3:
$out"
# Внешний ключ без индекса: время проверки детей до и после CREATE INDEX. Абсолютные мс зависят от
# машины, поэтому только соотношение: полный просмотр 10⁶ строк против поиска по индексу.
fk_times=$(printf '%s\n' "$out" | sed -nE 's/^ Trigger for constraint big_messages_chat_id_fkey: time=([0-9.]+) .*$/\1/p')
fk_no_index=$(printf '%s\n' "$fk_times" | sed -n 1p)
fk_index=$(printf '%s\n' "$fk_times" | sed -n 2p)
[ -n "$fk_no_index" ] && [ -n "$fk_index" ] && less "$(awk -v b="$fk_index" 'BEGIN { print b * 10 }')" "$fk_no_index" || fail "constraints.sql: проверка FK без индекса $fk_no_index мс, с индексом $fk_index мс, ждали разницу больше чем в 10 раз:
$out"
ok "constraints.sql: 23502, 23514, 23505, 23503, 23P01 и 0A000; отложенный FK проверяется в COMMIT; после 23505 транзакция — 25P02; ON CONFLICT с отложенным — 55000; WITHOUT OVERLAPS — 23P01; FK без индекса $fk_no_index мс против $fk_index мс"

# --- 3. antipatterns.sql: запрос, который ломается, и исправление (урок 2.7) ---
out=$(sql antipatterns.sql)
errors_expected "$out" antipatterns.sql 9
has "$out" antipatterns.sql 'SQLSTATE 22007' 'SQLSTATE 23503' 'SQLSTATE 23514' 'SQLSTATE 42601' 'SQLSTATE 23505' ' 20 | Обед' '(4 rows)' \
	'INSERT has more expressions than target columns' ' chat_feed  | id, chat_id, text' \
	'violates unique constraint "members_pkey"' ' messages_nofk_chat_id_fkey | t' \
	'chat_invites_max_uses_check1  | CHECK ((max_uses < 1000))' 'chat_invites_expires_at_idx1' 'chat_invites_expires_at_not_null' \
	'notification_delivery_attempt_subscriber_installation_id_no_key |    63'
# Строка результата WITH RECURSIVE (id 4 на глубине 3) — в эхо запросов её нет, только в выводе.
printf '%s\n' "$out" | grep -qE '^ +4 \| +3 \| +В пятницу некому чинить$' || fail "antipatterns.sql: рекурсивный запрос не дошёл до третьего уровня:
$out"
printf '%s\n' "$out" | grep -qE '^ f +\| t +\|' || fail "antipatterns.sql: float и numeric сравнились одинаково:
$out"
# «ID для всего»: дубль участника принят — строка группировки с двумя строками на пару.
printf '%s\n' "$out" | grep -qE '^ +10 \| +1 \| +2$' || fail "antipatterns.sql: дубль участника при суррогатном ключе не появился:
$out"
ok "antipatterns.sql: EAV, полиморфная связь, список через запятую, наивное дерево, float, SELECT *, «ID для всего» и ключ без FOREIGN KEY — сломались и исправлены; имена по умолчанию — с суффиксом 1 и обрезкой до 63 байт"

# --- 3a. tenancy.sql: общая схема с tenant_id и RLS (урок 2.7) ---
out=$(sql tenancy.sql)
errors_expected "$out" tenancy.sql 5
# $1 в кавычках — текст ошибки PostgreSQL, а не переменная оболочки.
# shellcheck disable=SC2016
has "$out" tenancy.sql 'SQLSTATE 42704' 'SQLSTATE 42501' 'SQLSTATE 23503' 'SQLSTATE 22P02' 'SQLSTATE 42601' \
	'unrecognized configuration parameter "app.tenant_id"' \
	'new row violates row-level security policy for table "chats"' \
	'violates foreign key constraint "messages_tenant_id_chat_id_fkey"' \
	'invalid input syntax for type bigint: ""' \
	'syntax error at or near "$1"' \
	'Index Cond: ((tenant_id = (current_setting('\''app.tenant_id'\''::text))::bigint) AND (id = 10))' \
	' globex: релиз, globex: зарплаты'
# Запрос acme без WHERE tenant_id видит свои два чата и ни одного чата globex.
request=$(printf '%s\n' "$out" | sed -n '/=== 4/,/=== 5/p')
has "$request" tenancy.sql ' 10 | acme: бэкенд' ' 11 | acme: обед' 'UPDATE 0'
! printf '%s\n' "$request" | grep -q 'globex:' || fail "tenancy.sql: запрос acme увидел чат globex:
$out"
[ "$(column_value "$out" globex_chat_visible)" = 0 ] || fail "tenancy.sql: чужой чат виден по id:
$out"
[ "$(column_value "$out" acme_chats)" = 2 ] || fail "tenancy.sql: set_config с параметром не выставил арендатора:
$out"
[ "$(column_value "$out" owner_sees)" = 4 ] && [ "$(column_value "$out" owner_sees_forced)" = 2 ] && [ "$(column_value "$out" superuser_sees_forced)" = 4 ] ||
	fail "tenancy.sql: владелец без FORCE должен видеть 4 строки, с FORCE — 2, суперпользователь — 4:
$out"
ok "tenancy.sql: без арендатора — 42704, после COMMIT — 22P02; запрос видит только своё; запись в чужого — 42501, ссылка на чужой чат — 23503; SET без LOCAL переживает запрос; SET с \$1 — 42601, set_config — да; владелец обходит RLS до FORCE, суперпользователь — всегда"

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

# --- 4a. history.sql: мягкое удаление, архив, обезличивание и аудит триггером (урок 2.6) ---
out=$(sql history.sql)
errors_expected "$out" history.sql 4
has "$out" history.sql 'SQLSTATE 23505' 'SQLSTATE 23503' \
	'violates unique constraint "users_username_key"' \
	'violates unique constraint "users_username_active_key"' \
	'Key (id)=(2) is still referenced from table "messages"' \
	'Key (id)=(1) is still referenced from table "messages"' \
	' tok-alice |       1' ' deleted_1 | пишу после удаления' \
	'Index Scan using users_username_active_key on users' 'Trigger audited_audit: calls=100000'
# Без условия deleted_at IS NULL частичный индекс не годится: первый план после него — полный просмотр.
printf '%s\n' "$out" | sed -n '/user42/,$p' | grep -m 1 -E '^ (Seq|Index) Scan' | grep -q '^ Seq Scan on users$' || fail "history.sql: запрос без deleted_at IS NULL пошёл не полным просмотром:
$out"
# Ручная правка попала в журнал без автора, правка из приложения — с ним.
printf '%s\n' "$out" | grep -qE '^ UPDATE \| user:2 +\| lab +\| Боб +\| Боб Б\. *$' || fail "history.sql: правка приложения записана без автора:
$out"
printf '%s\n' "$out" | grep -qE '^ UPDATE \| +\| lab +\| Боб Б\. +\| Боб *$' || fail "history.sql: ручная правка не попала в журнал:
$out"
# История alice: вставка, мягкое удаление и обезличивание — имя в каждой; после чистки — ни в одной.
printf '%s\n' "$out" | grep -qE '^ UPDATE \| alice +\| deleted_1 *$' || fail "history.sql: журнал не сохранил имя при обезличивании:
$out"
printf '%s\n' "$out" | sed -n '/AS audit_rows_with_name/,/row)/p' | grep -qE '^ +0$' || fail "history.sql: после чистки журнала имя осталось в истории:
$out"
# Цена триггера — только относительно: время и WAL зависят от машины.
times=$(printf '%s\n' "$out" | sed -n '/=== 8/,$p' | sed -nE 's/^ Execution Time: ([0-9.]+) ms$/\1/p')
t_plain=$(printf '%s\n' "$times" | sed -n 1p)
t_audited=$(printf '%s\n' "$times" | sed -n 2p)
wal_plain=$(first_number "$out" '^ plain +\| [^|]+\| +([0-9]+)')
wal_audited=$(first_number "$out" '^ audited +\| [^|]+\| +([0-9]+)')
[ -n "$t_plain" ] && [ -n "$t_audited" ] && less "$(awk -v a="$t_plain" 'BEGIN { print a * 1.2 }')" "$t_audited" || fail "history.sql: UPDATE с триггером $t_audited мс, без $t_plain мс, ждали дольше хотя бы в 1,2 раза:
$out"
[ -n "$wal_plain" ] && [ -n "$wal_audited" ] && less "$(awk -v a="$wal_plain" 'BEGIN { print a * 1.3 }')" "$wal_audited" || fail "history.sql: WAL с триггером $wal_audited байт, без $wal_plain, ждали больше хотя бы в 1,3 раза:
$out"
ok "history.sql: UNIQUE держит имя удалённого (23505), частичный индекс — нет; без deleted_at IS NULL — Seq Scan; FK принял сообщение удалённого; архив и DELETE упёрлись в FK (23503); журнал хранит имя после обезличивания, чистка его убирает; UPDATE с аудитом $t_audited мс против $t_plain мс, WAL $wal_audited против $wal_plain байт"

# --- 5. types.sql: типы и сравнение имён без учёта регистра (урок 2.3) ---
out=$(sql types.sql)
errors_expected "$out" types.sql 10
has "$out" types.sql 'SQLSTATE 22001' 'SQLSTATE 22P02' 'SQLSTATE 42P21' \
	'violates check constraint "titles_text_title_length"' \
	'violates unique constraint "users_citext_username_key"' \
	'violates unique constraint "users_ci_username_key"' \
	'violates unique constraint "users_folded_username_key"' \
	'nondeterministic collations are not supported for regular expressions' \
	'nondeterministic collations are not supported for ILIKE' \
	'2025-02-24 02:46:24.503+00'
# timestamp без зоны: тот же ряд — тот же момент в UTC и другой в Москве.
printf '%s\n' "$out" | grep -qE '^ 2026-10-21 12:00:00 \| 2026-10-21 15:00:00\+03$' || fail "types.sql: timestamptz не показал 15:00+03 в Москве:
$out"
[ "$(printf '%s\n' "$out" | grep -E '^ [tf]$' | tr -d ' \n')" = tf ] || fail "types.sql: сравнение моментов timestamp и timestamptz не дало t и f:
$out"
# casefold приводит ß к ss, lower — нет.
printf '%s\n' "$out" | grep -qE '^ straße +\| strasse +\| straße +\| strasse +\| t$' || fail "types.sql: casefold и lower отработали не так, как в уроке:
$out"
ok "types.sql: varchar — 22001, CHECK — 23514 с именем, enum — 22P02 без имени; timestamp теряет зону; citext и ICU-коллация ловят Alice/alice; регулярки — 0A000, FK между коллациями — 42P21; casefold(ß) = ss, и уникальный индекс по casefold ловит Straße/STRASSE"

# --- 6. keys: bigint, UUIDv4, UUIDv7 (урок 2.3) ---
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
