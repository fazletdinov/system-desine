#!/bin/bash
# Сводка трассы выполнения (урок 1.5.10): сколько циклов GC прошло за окно трассы,
# сколько длились паузы и разметка, сколько времени горутины провели в помощи разметке
# (отдельно — внутри хеша Argon2id и вне его) и как менялись куча и её цель.
# То же видно в go tool trace -http глазами; сводка нужна, чтобы сравнить две трассы
# числами. Источник — текстовый разбор трассы go tool trace -d=parsed: формат
# отладочный и может меняться между версиями Go (проверено на go1.27.2).
# Запуск: scripts/trace-summary.sh <файл трассы> — трасса с /debug/pprof/trace или
# снимок flight recorder с /debug/flight.
set -euo pipefail

file=${1:?нужно: scripts/trace-summary.sh <файл трассы>}

go tool trace -d=parsed "$file" | awk '
function at(field) { sub(/^Time=/, "", field); return field + 0 }
# Строка события начинается с M=; строки стека под ним — с отступа.
/^M=/ {
	inStack = 0
	t = 0; scope = ""
	for (i = 1; i <= NF; i++) {
		if ($i ~ /^Time=/) t = at($i)
		if ($i ~ /^Scope=/) scope = $i
	}
	if (t > 0 && (first == 0 || t < first)) first = t
	if (t > last) last = t
	if (!match($0, /Name="[^"]*"/)) next
	name = substr($0, RSTART + 6, RLENGTH - 7)
}
/^M=.* Metric / {
	match($0, /Uint64\([0-9]+\)/); v = substr($0, RSTART + 7, RLENGTH - 8) + 0
	if (name == "/memory/classes/heap/objects:bytes") { if (!heapLo || v < heapLo) heapLo = v; if (v > heapHi) heapHi = v }
	if (name == "/gc/heap/goal:bytes") { if (!goalLo || v < goalLo) goalLo = v; if (v > goalHi) goalHi = v }
	next
}
/^M=.* RangeBegin / {
	key = name SUBSEP scope; begin[key] = t
	if (name == "GC mark assist") { kind[key] = "вне хеша"; inStack = 1; cur = key }
	next
}
# Помощь разметке внутри хеша — аллокации самого argon2 (blake2b — его часть).
inStack && /argon2|blake2b/ { kind[cur] = "в хеше Argon2id" }
/^M=.* RangeEnd / {
	key = name SUBSEP scope
	if (!(key in begin)) next
	d = t - begin[key]; delete begin[key]
	label = name
	if (name == "GC mark assist") label = name " (" kind[key] ")"
	n[label]++; total[label] += d; if (d > longest[label]) longest[label] = d
}
END {
	printf "окно трассы: %.2f с\n", (last - first) / 1e9
	# Порядок обхода массива в awk не задан — строки сортируем. Имя — в конце строки:
	# mawk выравнивает по байтам, и кириллица в середине сбила бы столбцы.
	for (l in n) printf "%5d раз, всего %8.1f мс, самое долгое %7.2f мс  %s\n", n[l], total[l] / 1e6, longest[l] / 1e6, l | "sort -k9"
	close("sort")
	printf "куча (объекты): от %.1f до %.1f МиБ\n", heapLo / 1048576, heapHi / 1048576
	printf "цель кучи: от %.1f до %.1f МиБ\n", goalLo / 1048576, goalHi / 1048576
}'
