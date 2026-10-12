#!/usr/bin/env bash
# Задержка и потери между client и server: ./netem.sh 40ms 1%  |  ./netem.sh off
# Задержка делится пополам на оба конца, поэтому RTT равен первому аргументу.
set -euo pipefail
cd "$(dirname "$0")"

compose() { docker compose -f compose.yaml "$@"; }
services=(server client)

if [[ "${1:-}" == "off" ]]; then
  for svc in "${services[@]}"; do
    # Ошибка exec (стек не запущен, docker недоступен) — настоящая; "нет netem" — норма.
    if ! show="$(compose exec -T "$svc" tc qdisc show dev eth0 2>&1)"; then
      echo "не удалось обратиться к $svc (стек запущен? docker up -d): $show" >&2
      exit 1
    fi
    if grep -q netem <<<"$show"; then
      compose exec -T "$svc" tc qdisc del dev eth0 root
    fi
  done
  echo "netem снят"
  exit 0
fi

rtt="${1:?укажите RTT, например 40ms}"
loss="${2:-0%}"
[[ "$rtt" =~ ^[0-9]+ms$ ]] || { echo "RTT в формате 40ms" >&2; exit 2; }
[[ "$loss" =~ ^[0-9]+(\.[0-9]+)?%$ ]] || { echo "потери в формате 1% или 0.5%" >&2; exit 2; }
ms="${rtt%ms}"
# Микросекунды, чтобы нечётный RTT делился ровно пополам.
half="$((ms * 500))us"

applied=()
for svc in "${services[@]}"; do
  if ! out="$(compose exec -T "$svc" tc qdisc replace dev eth0 root netem delay "$half" loss "$loss" 2>&1)"; then
    echo "не удалось включить netem в $svc: $out" >&2
    if grep -qiE 'unknown qdisc|netem.*not found|no such file' <<<"$out"; then
      echo "на хосте нужен модуль sch_netem (sudo modprobe sch_netem)" >&2
    fi
    for done_svc in "${applied[@]}"; do
      compose exec -T "$done_svc" tc qdisc del dev eth0 root || true
    done
    exit 1
  fi
  applied+=("$svc")
done

# Первый пакет иначе платит лишний RTT за ARP через netem на обоих концах.
compose exec -T client ping -c1 -W2 172.28.0.10 >/dev/null || true
echo "RTT ≈ $rtt, потери $loss на каждом конце"
