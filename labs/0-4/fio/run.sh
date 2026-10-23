#!/bin/sh
# Один замер fio по файлу в /data (bind mount каталога на диске хоста).
#   lab-fio <ключ> <rw> <bs> <размер файла> <секунд>
# Печатает сводку по задержке завершения (clat) и строки RESULT <ключ>_p50_us …;
# полный JSON fio — в /data/<ключ>.json, scenario.sh забирает его в results/.
set -eu

key=$1 rw=$2 bs=$3 size=$4 runtime=$5
file=/data/fio-test.dat

no_disk() {
	echo "ОШИБКА: каталог данных /data — $1, а не диск: fio измерил бы память." >&2
	echo "Укажите каталог на настоящем диске: LAB04_DATA=/путь/на/диске bash 0-4/scenario.sh" >&2
	exit 3
}

if [ ! -d /data ]; then
	echo "ОШИБКА: каталог /data не смонтирован; запускайте через compose: docker compose -f 0-4/compose.yaml run --rm fio …" >&2
	exit 3
fi

# Тип ФС по магическому числу statfs: busybox знает не все имена (overlay — UNKNOWN).
# tmpfs с Linux 6.6 принимает O_DIRECT, поэтому одной пробы O_DIRECT ниже мало.
case $(stat -f -c %t /data) in
1021994) no_disk tmpfs ;;
858458f6) no_disk ramfs ;;
794c7630) no_disk "overlay (слой контейнера)" ;;
esac

# O_DIRECT обходит page cache: без него fio измерил бы чтение из RAM, а не с SSD.
# ФС без O_DIRECT (часть FUSE, старые tmpfs) — замер бессмысленен, а не «быстрый».
if ! fio --name=probe --filename=/data/.odirect-probe --size=4k --bs=4k --direct=1 \
	--ioengine=libaio --rw=read --output=/dev/null >/dev/null 2>&1; then
	rm -f /data/.odirect-probe
	echo "ОШИБКА: файловая система каталога данных не поддерживает O_DIRECT (запись в обход page cache)." >&2
	echo "Укажите каталог на настоящем диске: LAB04_DATA=/путь/на/диске bash 0-4/scenario.sh" >&2
	exit 3
fi
rm -f /data/.odirect-probe

# iodepth=1 — один запрос в полёте: так меряется задержка одного чтения, как в шкале урока,
# а не пропускная способность очереди NVMe. time_based — ровно runtime секунд, файл идёт по кругу.
fio --name="$key" --filename="$file" --size="$size" \
	--direct=1 --ioengine=libaio --iodepth=1 --rw="$rw" --bs="$bs" \
	--runtime="$runtime" --time_based --clat_percentiles=1 --lat_percentiles=1 --output-format=json --output="/data/$key.json" >/dev/null

j=/data/$key.json
echo "fio: $(jq -r '."fio version"' "$j"), --rw=$rw --bs=$bs --size=$size --runtime=$runtime --direct=1 --ioengine=libaio --iodepth=1 --clat_percentiles=1 --lat_percentiles=1"
# lat = slat + clat — время одного чтения, которое видит программа: от вызова io_submit до
# завершения. slat — сама отправка, clat — от отправки до завершения. Для 4 КБ slat — единицы
# микросекунд. Для 1 МБ — нет: ядро режет запрос на куски по max_sectors_kb (у NVMe часто 128 КБ),
# и отправка восьми кусков занимает заметное время. Шкала урока берёт lat, clat — для сравнения.
jq -r '.jobs[0].read as $r | $r.clat_ns as $c | $r.lat_ns as $l |
	def us: . / 1e3 | . * 10 | round / 10;
	def pct($h; $p): $h.percentile[$p] / 1e3;
	"операций: \($r.total_ios), IOPS: \($r.iops | floor), полоса: \($r.bw_bytes / 1e6 | floor) МБ/с",
	"slat мкс: среднее \($r.slat_ns.mean | us)",
	"lat  мкс: min \($l.min | us), среднее \($l.mean | us), p50 \(pct($l; "50.000000")), p90 \(pct($l; "90.000000")), p99 \(pct($l; "99.000000")), p99.9 \(pct($l; "99.900000")), max \($l.max | us)",
	"clat мкс: min \($c.min | us), среднее \($c.mean | us), p50 \(pct($c; "50.000000")), p90 \(pct($c; "90.000000")), p99 \(pct($c; "99.000000")), p99.9 \(pct($c; "99.900000")), max \($c.max | us)"' "$j"
jq -r --arg k "$key" '.jobs[0].read as $r |
	"RESULT \($k)_p50_us \($r.clat_ns.percentile."50.000000" / 1e3) us",
	"RESULT \($k)_p99_us \($r.clat_ns.percentile."99.000000" / 1e3) us",
	"RESULT \($k)_mean_us \($r.clat_ns.mean / 1e3 | . * 10 | round / 10) us",
	"RESULT \($k)_lat_p50_us \($r.lat_ns.percentile."50.000000" / 1e3) us",
	"RESULT \($k)_lat_p99_us \($r.lat_ns.percentile."99.000000" / 1e3) us",
	"RESULT \($k)_lat_mean_us \($r.lat_ns.mean / 1e3 | . * 10 | round / 10) us"' "$j"
