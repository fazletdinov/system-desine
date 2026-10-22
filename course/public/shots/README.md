# Снимки экрана к урокам

Настоящие снимки программ с окнами, из прогонов этого репозитория. Показываются компонентом
`<Shot>` (`course/components/Shot.vue`): пометки ①②③ и пояснения к ним задаются в уроке, а не
рисуются на картинке, поэтому снимок можно переснять, не трогая текст пометок, если окно
выглядит так же. `npm run check:refs` проверяет, что файл есть, это WebP, размеры совпадают
с `:width`/`:height` в уроке, а номера пометок идут 1…N без пропусков.

Формат — WebP: с потерями (качество 90) или без, что меньше; ориентир — до 250 КБ на файл.
Обрезка и склейка — Python PIL. Склеенные из двух частей окна снимки (пустая середина вырезана)
помечены в подписи и разделены на картинке пунктирным «разрывом».

## 0b-01-layers и 0b-08-quic — Wireshark 4.6.6

Wireshark — в контейнере с веб-доступом к рабочему столу:

```bash
# образ: lscr.io/linuxserver/wireshark@sha256:9f35035b96eec43ec8736e0ad66fa52b0f5a12e37b1e8e9cfc6f19bf2fdf7583
docker run -d --name ws-shot -e PUID=1000 -e PGID=1000 -e TZ=Etc/UTC -p 127.0.0.1:3300:3000 \
  -v "$PWD/labs/0b/captures:/captures:ro" --shm-size=1g \
  lscr.io/linuxserver/wireshark@sha256:9f35035b96eec43ec8736e0ad66fa52b0f5a12e37b1e8e9cfc6f19bf2fdf7583
# через ~25 с перезапустить Wireshark с нужным файлом и аргументами:
docker exec ws-shot sh -c 'pkill -x wireshark; sleep 1; su abc -s /bin/sh -c "XDG_RUNTIME_DIR=/config/.XDG WAYLAND_DISPLAY=wayland-0 QT_QPA_PLATFORM=wayland HOME=/config nohup wireshark ARGS >/tmp/ws.log 2>&1 &"'
```

Окно браузера — 1400×900 (http://127.0.0.1:3300/), ручку боковой панели Selkies (`.toggle-handle`)
перед снимком скрыть стилем.

| файл | аргументы Wireshark | что сделать в окне | обрезка |
|---|---|---|---|
| `wireshark-filter.webp` | `-r /captures/first.pcap -Y "tcp.flags.syn == 1"` | — | всё окно |
| `wireshark-layers.webp` | `-r /captures/first.pcap -g 4` | клик по строке IPv4 в дереве, `→` | y 120–900 |
| `wireshark-flow-graph.webp` | `-r /captures/first.pcap -g 4` | Statistics → Flow Graph | y 0–362 |
| `wireshark-follow-tcp.webp` | `-r /captures/first.pcap -g 4` | правый клик по пакету 4 → Follow → TCP Stream | y 0–222 + y 780–900 |
| `wireshark-seq-raw.webp` | `-r /captures/first.pcap -g 1` | клик по TCP, `→`, клик по `Sequence Number (raw)` | y 120–900 |
| `wireshark-no-keys.webp` | `-r /captures/quic-h3.pcap -g 16` | клик по `QUIC IETF`, `Shift+→` | y 120–900 |
| `wireshark-with-keys.webp` | `-r /captures/quic-h3.pcap -o tls.keylog_file:/captures/quic-h3.keys -g 16` | клик по `Hypertext Transfer Protocol Version 3`, `Shift+→` | y 120–900 |

`first.pcap` — запись урока 0b.1 по `labs/0b/README.md`. `quic-h3.pcap` и `quic-h3.keys` — запись
HTTP/3 внутри сети стенда 0b: до интернета UDP 443 у нас не проходит (урок 0b.8, шаг 3), поэтому
сервером служит Caddy, а клиентом — `curl` из образа лабы, который пишет ключи по `SSLKEYLOGFILE`:

```bash
docker compose -f labs/0b/compose.yaml up -d
printf 'h3.messenger.example {\n\ttls internal\n\trespond "hello over HTTP/3"\n}\n' > /tmp/Caddyfile
docker run -d --name h3-shot --network lab0b_labnet --ip 172.28.0.30 -v /tmp/Caddyfile:/etc/caddy/Caddyfile:ro \
  caddy:2.10.2@sha256:c3d7ee5d2b11f9dc54f947f68a734c84e9c9666c92c88a7f30b9cba5da182adb
lab exec -d client tcpdump --immediate-mode -i eth0 -w /captures/quic-h3.pcap udp port 443
lab exec -e SSLKEYLOGFILE=/captures/quic-h3.keys client curl --http3-only -k -sS -i \
  --resolve h3.messenger.example:443:172.28.0.30 https://h3.messenger.example/
lab exec client pkill tcpdump
docker rm -f h3-shot && docker compose -f labs/0b/compose.yaml down
```

Файл ключей — секрет: с ним расшифровывается вся запись. В git он не попадает (`labs/0b/captures/*.keys`).
