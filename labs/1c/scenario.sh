#!/bin/bash
# Сценарный прогон пяти экспериментов урока 1.15 внутри контейнера лабы.
# Падает на первом расхождении; check:labs запускает его на каждой проверке.
set -euo pipefail

fail() {
	echo "ПРОВАЛ: $*" >&2
	exit 1
}

# 1. Свой PID namespace: процесс видит себя единственным и с PID 1.
out=$(minicontainer run /rootfs ps 2>/dev/null)
lines=$(echo "$out" | awk 'NR > 1' | wc -l)
pid=$(echo "$out" | awk 'NR == 2 { print $1 }')
if [ "$lines" -ne 1 ] || [ "$pid" != 1 ]; then
	fail "ps в контейнере: ждали один процесс с PID 1, получили:
$out"
fi
echo "ок 1: в контейнере один процесс, PID 1"

# 2. Свой UTS namespace: hostname внутри не меняет hostname окружения.
inside=$(minicontainer run --hostname box /rootfs hostname 2>/dev/null)
[ "$inside" = box ] || fail "hostname внутри: $inside"
[ "$(hostname)" = lab ] || fail "hostname окружения изменился: $(hostname)"
echo "ок 2: hostname внутри — box, снаружи — lab"

# 3. Свой net namespace: сетевой карты окружения (eth0) нет, ни один интерфейс не
# поднят, адресов нет. Кроме lo ядро может создать в новом namespace «запасные»
# туннели (tunl0, gre0, sit0…), если модули ipip/gre/sit встроены в ядро, как в VM
# Docker Desktop, — они тоже выключены и никуда не ведут.
ip -o link show eth0 >/dev/null 2>&1 || fail "у окружения нет eth0 — сравнивать не с чем"
links=$(minicontainer run /rootfs ip -o link 2>/dev/null)
up=$(minicontainer run /rootfs ip -o link show up 2>/dev/null)
addrs=$(minicontainer run /rootfs ip -o addr 2>/dev/null)
if ! echo "$links" | grep -q '^1: lo:' || echo "$links" | grep -q 'eth0' || [ -n "$up" ] || [ -n "$addrs" ]; then
	fail "сеть в контейнере:
$links
поднятые: $up
адреса: $addrs"
fi
echo "ок 3: eth0 окружения нет, lo и остальные интерфейсы выключены, адресов нет"

# 4. pids.max: 64 фоновых процесса при лимите 16 — часть не родится.
# shellcheck disable=SC2016 # $(seq …) раскрывает sh внутри контейнера, а не этот скрипт
err=$(minicontainer run --pids 16 /rootfs sh -c 'for i in $(seq 1 64); do sleep 1 & done; wait' 2>&1 >/dev/null || true)
echo "$err" | grep -Eq 'pids_max=[1-9]' || fail "pids.max не сработал: $err"
echo "ок 4: лимит pids.max отказал в fork"

# 5. memory.max: hog хочет 128 МиБ при лимите 32 — OOM-kill, код 137.
set +e
err=$(minicontainer run --mem 32M /rootfs hog -mib 128 2>&1 >/dev/null)
code=$?
set -e
[ "$code" -eq 137 ] || fail "код выхода $code, ждали 137: $err"
echo "$err" | grep -Eq 'oom_kill=[1-9]' || fail "oom_kill не отмечен: $err"
echo "ок 5: OOM-kill по memory.max, код выхода 137"

# Ни одна группа не осталась после запусков — в том числе после OOM-kill.
if ls -d /sys/fs/cgroup/minicontainer-* >/dev/null 2>&1; then
	fail "остались группы: $(ls -d /sys/fs/cgroup/minicontainer-*)"
fi
echo "Все пять экспериментов прошли."
