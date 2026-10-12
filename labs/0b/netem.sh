#!/usr/bin/env bash
# Задержка и потери между client и server: ./netem.sh 40ms 1%  |  ./netem.sh off
# Задержка делится пополам на оба конца, поэтому RTT равен первому аргументу.
set -euo pipefail
cd "$(dirname "$0")"

compose() { docker compose -f compose.yaml "$@"; }

if [[ "${1:-}" == "off" ]]; then
  for svc in server client; do compose exec -T "$svc" tc qdisc del dev eth0 root 2>/dev/null || true; done
  echo "netem снят"
  exit 0
fi

rtt="${1:?укажите RTT, например 40ms}"
loss="${2:-0%}"
ms="${rtt%ms}"
[[ "$ms" =~ ^[0-9]+$ ]] || { echo "RTT в формате 40ms" >&2; exit 2; }
half="$((ms / 2))ms"

for svc in server client; do
  if ! compose exec -T "$svc" tc qdisc replace dev eth0 root netem delay "$half" loss "$loss"; then
    echo "не удалось включить netem в $svc: на хосте нужен модуль sch_netem (sudo modprobe sch_netem)" >&2
    exit 1
  fi
done
echo "RTT ≈ $rtt, потери $loss на каждом конце"
