# Лабораторные модуля 0b

Окружение: три контейнера в сети `labnet` (172.28.0.0/24) — `server` (172.28.0.10),
`client` (172.28.0.20) и `dns` на CoreDNS (172.28.0.53, зона `messenger.example`).

## Требования

- Docker с Compose;
- модуль ядра `sch_netem` (для имитации задержек и потерь): `sudo modprobe sch_netem`.

## Запуск

```bash
docker compose -f labs/0b/compose.yaml up -d --build
alias lab='docker compose -f labs/0b/compose.yaml'   # псевдоним из уроков модуля 0b
```

## Задержки и потери (netem)

```bash
labs/0b/netem.sh 40ms 1%   # RTT 40 мс, 1% потерь на каждом конце
labs/0b/netem.sh off       # снять
```

## Запись трафика

```bash
docker compose -f labs/0b/compose.yaml exec client tcpdump -i eth0 -w /captures/x.pcap
```

Файлы `.pcap` появляются в `labs/0b/captures/` и не попадают в git. Их пишет root внутри контейнера, поэтому удалить с хоста может понадобиться через `sudo`.

## DNS

Внутри контейнеров в `/etc/resolv.conf` виден `127.0.0.11` — это встроенный DNS Docker,
он пересылает запросы на 172.28.0.53. После правки файла зоны увеличьте серийный номер SOA:
CoreDNS (`file`) перечитывает зону только при его смене. Либо перезапустите сервис:
`docker compose -f labs/0b/compose.yaml restart dns`.

## Остановка

```bash
docker compose -f labs/0b/compose.yaml down
```

## Лабораторные по урокам

### Урок 0b.1 — слои, адреса, первая запись трафика

Урок: [0b.1. Слои: от IP-пакета до HTTP](../../course/modules/00b-network/01-layers.md).

```bash
labs/0b/netem.sh 40ms 0%
lab exec -d server h2server -addr :8443 -http-addr :8080 -names api.messenger.example,server -ca-out /captures/ca.pem
lab exec client ip -br addr show eth0
lab exec client ip route
lab exec client ip route get 172.28.0.10
lab exec server ss -tlnp
lab exec -d client tcpdump -i eth0 -w /captures/first.pcap
lab exec client curl -s http://172.28.0.10:8080/
lab exec client ss -tan
lab exec client pkill tcpdump
lab exec client tcpdump -nn -r /captures/first.pcap
lab exec client tcpdump -nn -e -v -r /captures/first.pcap
```

Файл `labs/0b/captures/first.pcap` откройте в Wireshark. В записи клиента между `SYN` и `SYN-ACK`
20 мс, а не 40: `tcpdump` видит исходящий пакет, когда тот выходит из очереди netem.

### Урок 0b.2 — TCP под задержкой и потерями

Урок: [0b.2. TCP изнутри](../../course/modules/00b-network/02-tcp.md). Программа — `tcpecho` (ниже).

```bash
lab exec -d server tcpecho -serve -addr :7000
labs/0b/netem.sh 40ms 0%
lab exec client tcpecho -addr 172.28.0.10:7000 -n 200 -interval 10ms
labs/0b/netem.sh 40ms 5%
lab exec client tcpecho -addr 172.28.0.10:7000 -n 200 -interval 50ms
lab exec client ss -ti dst 172.28.0.10   # во втором терминале, пока идёт клиент (например, -n 30 -interval 200ms)
labs/0b/netem.sh 40ms 10%
lab exec client sh -c 'for i in $(seq 1 40); do tcpecho -addr 172.28.0.10:7000 -n 1 2>&1 | grep connect; done'
lab exec client ss -tan dst 172.28.0.10
```

Результат на нашем стенде (у вас числа будут другими, порядок — такой): при 5% потерь p50 40,7 мс,
p99 294 мс (повтор по таймеру: RTO ≈ 240 мс + RTT); при 10% потерь из 40 рукопожатий 33 заняли ≈ 40 мс,
4 — ≈ 1,05 с, 3 — ≈ 2,07 с (начальный RTO 1 с). Демонстрация Нейгла — в разделе tcpecho ниже.

### Урок 0b.3 — UDP под потерями

Урок: [0b.3. UDP: когда надёжность не нужна](../../course/modules/00b-network/03-udp.md). Программа — `udpping` (ниже).

```bash
lab exec -d server udpping -serve -addr :7001
labs/0b/netem.sh 40ms 0%
lab exec client udpping -addr 172.28.0.10:7001 -n 200
labs/0b/netem.sh 40ms 2%
lab exec client udpping -addr 172.28.0.10:7001 -n 200
labs/0b/netem.sh 40ms 5%
lab exec client udpping -addr 172.28.0.10:7001 -n 200
lab exec client udpping -addr 172.28.0.10:7001 -n 5000 -interval 2ms   # точнее: выборка больше
labs/0b/netem.sh 40ms 0%
lab exec -d client tcpdump -i eth0 -w /captures/udp.pcap udp port 7001
lab exec client udpping -addr 172.28.0.10:7001 -n 3 -interval 100ms
lab exec client pkill tcpdump
lab exec client tcpdump -nn -X -c1 -r /captures/udp.pcap   # заголовок UDP побайтно
lab exec client udpping -addr 172.28.0.10:7999 -n 5 -interval 100ms   # порт не слушают: refused=5
```

Результат на нашем стенде (у вас числа будут другими, порядок — такой): RTT p99 ≈ 41 мс при любых потерях
(у TCP при 5% — 294 мс); потери пингов при n=5000 — 3,8% при 2% и 10,3% при 5% на каждом конце
(расчёт 1 − (1 − p)²: 3,96% и 9,75%). При n=200 разброс между прогонами — пара процентных пунктов.

### Урок 0b.4 — DNS

Урок: [0b.4. DNS глубже](../../course/modules/00b-network/04-dns.md). У контейнера `dns` netem нет, поэтому RTT до DNS
при `netem.sh 40ms` — около 20 мс (задерживается только исходящий вопрос клиента).

```bash
labs/0b/netem.sh 40ms 0%
lab exec client cat /etc/resolv.conf
lab exec client dig api.messenger.example
lab exec client dig www.messenger.example +noall +answer
lab exec client dig _api._tcp.messenger.example SRV +noall +answer
lab exec client dig nope.messenger.example             # NXDOMAIN + SOA: отрицательный кэш
lab exec client dig api.messenger.example AAAA         # NODATA
lab exec client dig messenger.example NS               # адрес ns в ADDITIONAL (как glue)
for i in 1 2 3; do lab exec client dig go.dev +noall +answer +stats | grep -E 'IN|Query time'; sleep 5; done   # TTL убывает (нужен интернет)
lab exec client dig +trace go.dev
```

Правка зоны: увеличьте серийный номер SOA и перезапишите файл на месте (Docker Desktop может не увидеть файл,
заменённый `sed -i`); CoreDNS перечитает зону в течение минуты. Вернуть исходную зону — `git checkout` и `restart dns`.

```bash
sed 's/^api     IN A   172.28.0.10/api     IN A   172.28.0.11/; s/2026101201/2026101202/' labs/0b/dns/messenger.example.db > /tmp/zone.db
cat /tmp/zone.db > labs/0b/dns/messenger.example.db
for i in $(seq 1 14); do echo "$(date +%T) $(lab exec client dig api.messenger.example +short)"; sleep 5; done
git checkout labs/0b/dns/messenger.example.db && lab restart dns
```

Резолвер Go и долгие соединения (нужен `h2server`, см. ниже):

```bash
lab exec -e GODEBUG=netdns=go+2 client httpbench -url https://api.messenger.example:8443/ -ca /captures/ca.pem -c 1 -n 5 -keepalive=false   # 5 поисков DNS: кэша нет
lab exec client httpbench -url 'https://api.messenger.example:8443/slow?ms=1000' -ca /captures/ca.pem -c 1 -n 75 -keepalive=true   # поменяйте зону во время прогона
```

Kubernetes-подобный `resolv.conf` (`search` из трёх доменов, `ndots:5`) и недоступный DNS-сервер:

```bash
lab exec client cp /etc/resolv.conf /tmp/resolv.conf.orig
lab exec client sh -c 'printf "nameserver 127.0.0.11\nsearch default.svc.cluster.local svc.cluster.local cluster.local\noptions ndots:5\n" > /etc/resolv.conf'
lab exec client httpbench -url https://api.github.com/ -c 1 -n 1    # 8 DNS-запросов
lab exec client httpbench -url https://api.github.com./ -c 1 -n 1   # 2 запроса, но HTTP 400 из-за точки в Host
lab exec client sh -c 'printf "nameserver 172.28.0.99\n" > /etc/resolv.conf'
lab exec client httpbench -url https://api.messenger.example:8443/ -ca /captures/ca.pem -c 1 -n 1 -timeout 60s   # i/o timeout через 10 с
lab exec client cp /tmp/resolv.conf.orig /etc/resolv.conf
```

Результат на нашем стенде (у вас числа будут другими, порядок — такой): `dig` своей зоны — 20–21 мс (первый — 42 мс из-за ARP);
`go.dev` — 94 мс холодный кэш, 20 мс тёплый, TTL 30 → 25 → 20; зона перечитана в пределах минуты после смены серийного
номера; долгое соединение: 75 запросов, `errors=0 tcp_conns=1`, хотя DNS уже отдавал новый адрес; без keep-alive —
`dns: n=5`; с `ndots:5` — 83 мс против 21 мс с точкой; недоступный DNS — `total=10.00364s` (5 с × 2 попытки).
`dig +trace` на нашей сети показал перехват DNS провайдером (ответ «корня» с флагом `ra` и без `aa`).

### tcpecho — TCP-эхо

Сервер возвращает каждую строку; клиент меряет время рукопожатия и RTT каждой строки.

```bash
labs/0b/netem.sh 40ms 0%
docker compose -f labs/0b/compose.yaml exec -d server tcpecho -serve -addr :7000
docker compose -f labs/0b/compose.yaml exec client tcpecho -addr 172.28.0.10:7000 -n 20
```

Флаги клиента: `-n`, `-interval`, `-size`, `-dial`, `-timeout`, `-nodelay`, `-writes 1|2`.
Флаги сервера: `-idle` (закрыть молчащее или не читающее соединение), `-shutdown` (сколько ждать при остановке по SIGINT/SIGTERM).
Инспекция сокета во время работы клиента: `docker compose -f labs/0b/compose.yaml exec client ss -ti dst 172.28.0.10`.

Остановка сервера по сигналу завершает только текущий обмен: строки, ещё лежащие в буферах, теряются.

#### Алгоритм Нейгла

Нейгл задерживает маленький сегмент, пока не подтверждён предыдущий. Заметно это в шаблоне
write-write-read: `-writes 2` отправляет строку двумя вызовами `Write`, потом ждёт эхо.

```bash
labs/0b/netem.sh 40ms 0%
docker compose -f labs/0b/compose.yaml exec client tcpecho -addr 172.28.0.10:7000 -n 200 -interval 10ms -writes 2 -nodelay=false
docker compose -f labs/0b/compose.yaml exec client tcpecho -addr 172.28.0.10:7000 -n 200 -interval 10ms -writes 2 -nodelay=true
```

Результат на нашем стенде (RTT 40 мс; у вас числа будут другими, порядок — такой):

| режим | p50 | min |
|---|---|---|
| `-writes 1` (по умолчанию) | 40,2 мс | 40,1 мс |
| `-writes 1 -nodelay=false` (контроль) | 40,2 мс | 40,1 мс |
| `-writes 2 -nodelay=false` | 120,8 мс | 80,3 мс |
| `-writes 2 -nodelay=true` | 40,3 мс | 40,2 мс |

Раскладка 120 мс (проверена `tcpdump` на сервере): 20 мс голова строки до сервера + 40 мс отложенный ACK сервера +
20 мс ACK назад + 20 мс хвост + 20 мс эхо; 80 мс — то же без отложенного ACK.

Один `Write` на строку Нейгл не задерживает: нечего ждать. Минимум 80 мс при `-writes 2 -nodelay=false`
показывает, что первые обмены проходят быстрее: ядро Linux в начале соединения подтверждает сегменты
сразу (quickack), а позже начинает откладывать ACK.

### udpping — UDP-пинг

Клиент шлёт датаграммы с номером и временем отправки и считает потери, дубликаты и переупорядочивание.

```bash
labs/0b/netem.sh 40ms 5%
docker compose -f labs/0b/compose.yaml exec -d server udpping -serve -addr :7001
docker compose -f labs/0b/compose.yaml exec client udpping -addr 172.28.0.10:7001 -n 200
```

Потери netem действуют на каждом конце, поэтому при 5% теряется около 1 − 0,95² ≈ 10% round trip'ов.
Флаги клиента: `-n`, `-interval`, `-wait`. Метка времени в пакете — от монотонных часов клиента, поэтому скачок системных часов RTT не портит.

### h2server, httpbench, tlsdemo — HTTP и TLS

`h2server` при старте создаёт собственный удостоверяющий центр (CA), выпускает им сертификат
для `api.messenger.example` и `server` и кладёт сертификат CA в `/captures/ca.pem` — клиенты
доверяют только ему. HTTPS на `:8443` (HTTP/2 или HTTP/1.1 — по ALPN), HTTP/1.1 без TLS на `:8080`.

Обработчики: `/` — протокол и адрес клиента; `/bytes?n=N` — N байт (до 10 МБ);
`/slow?ms=N` — ответ через N мс (до 10 с). По SIGINT/SIGTERM сервер перестаёт принимать
соединения и ждёт активные запросы не дольше `-grace` (15 с). Запрос, клиент которого ушёл
раньше ответа, в журнале помечен `aborted=true`. `ca.pem` пишется атомарно (временный файл
и переименование): клиент не прочитает его наполовину записанным.

```bash
labs/0b/netem.sh 40ms 0%
docker compose -f labs/0b/compose.yaml exec -d server h2server -addr :8443 -http-addr :8080 -names api.messenger.example,server -ca-out /captures/ca.pem
```

#### httpbench — keep-alive, пул соединений, HTTP/2

Первая строка вывода — `requests=… errors=… tcp_conns=… new_conns=… proto=…`:

- `tcp_conns` — сколько TCP-соединений клиент установил на самом деле (успешные `ConnectDone`
  в `httptrace`), включая наборы, брошенные посреди TLS: поэтому строка `tls` может показать меньше;
- `new_conns` — сколько запросов получили соединение с `GotConnInfo.Reused == false`. Это нижняя граница
  новых соединений, а не их число: `net/http` помечает соединение использованным, когда кладёт его
  в пул простаивающих, и соединение, которое набрали «впрок» и положили в пул, позже придёт к запросу
  с `Reused == true`. Разница `tcp_conns − new_conns` — соединения, набранные впустую.

Строки `connect` и `tls` — длительности рукопожатий. Задержка меряется до последнего байта тела.

```bash
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -keepalive=false -c 1 -n 20
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -keepalive=true -c 1 -n 20
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -c 10 -n 100 -max-idle-per-host 2
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -c 10 -n 100 -max-idle-per-host 10
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -http2=true -c 10 -n 100
```

Результат на нашем стенде (RTT 40 мс, без потерь; у вас числа будут другими, порядок — такой):

| запуск | tcp_conns | new_conns | p50 | p99 |
|---|---|---|---|---|
| `-keepalive=false -c 1 -n 20` | 20 | 20 | 126,2 мс | 127,3 мс |
| `-keepalive=true -c 1 -n 20` | 1 | 1 | 41,5 мс | 125,9 мс |
| `-c 10 -n 100 -max-idle-per-host 2` | 10 | 10 | 41,2 мс | 136,3 мс |
| `-c 10 -n 100 -max-idle-per-host 10` | 10 | 10 | 41,2 мс | 135,9 мс |
| `-http2=true -c 10 -n 100` | 10 | 1 | 41,4 мс | 168,2 мс |

Без keep-alive каждый запрос — три RTT: TCP-рукопожатие, TLS 1.3-рукопожатие, сам запрос.
С keep-alive так платит только первый запрос на соединении — он и даёт p99.

При `-c 10` предел пула 2 и 10 неразличимы, и `tcp_conns=10` — ни одного лишнего набора: воркеры
забирают соединение обратно сразу после ответа, и простаивающих больше двух почти не бывает.
При `-c 50` предел становится виден:

```bash
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -c 50 -n 500 -max-idle-per-host 2    # tcp_conns=203 new_conns=50
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -c 50 -n 500 -max-idle-per-host 50   # tcp_conns=50 new_conns=50
```

С пулом на 2 соединения освободившиеся соединения не помещаются в пул и закрываются. Следующий
запрос не находит свободного соединения и начинает набор нового, но раньше, чем набор закончится
(TCP + TLS — два RTT), ему достаётся чужое освободившееся соединение. Брошенный набор при этом
продолжается: готовое соединение уходит в пул, а если пул полон — закрывается, так и не
обслужив ни одного запроса. Итог: 203 TCP-соединения на 500 запросов вместо 50, хотя `new_conns`
в обоих случаях 50.

HTTP/2 обслуживает все 100 запросов одним соединением (`new_conns=1`), но `tcp_conns=10`:
десять одновременных первых запросов успели начать по набору, пока ни один не завершился
и ALPN не показал, что соединение можно делить. Лишние соединения клиент закрыл без единого запроса.

С потерями (`labs/0b/netem.sh 40ms 2%`, `-c 10 -n 100`, шесть прогонов каждого):

```bash
labs/0b/netem.sh 40ms 2%
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -http2=true -c 10 -n 100
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -http2=false -c 10 -n 100
```

| режим | tcp_conns | new_conns | p50 | p99 |
|---|---|---|---|---|
| `-http2=true` | 8–10 | 1 | 42,1–43,6 мс | 163–365 мс |
| `-http2=false` | 10–33 | 8–10 | 40,8–41,4 мс | 288–380 мс |

Разброс большой: 100 коротких ответов — мало, чтобы по ним сравнивать протоколы под потерями.

#### tlsdemo — проверка сертификата и возобновление сессии

```bash
docker compose -f labs/0b/compose.yaml exec client tlsdemo get -url https://api.messenger.example:8443/ -ca /captures/ca.pem
docker compose -f labs/0b/compose.yaml exec client tlsdemo get -url https://api.messenger.example:8443/ -ca /captures/ca.pem -server-name evil.example
docker compose -f labs/0b/compose.yaml exec client tlsdemo get -url https://api.messenger.example:8443/
docker compose -f labs/0b/compose.yaml exec client tlsdemo inspect -addr 172.28.0.10:8443 -server-name api.messenger.example -ca /captures/ca.pem
```

`get` печатает версию TLS, выбранный ALPN, цепочку сертификатов и ответ. Второй и третий
запуски падают с кодом 1: чужое имя и неизвестный CA (без `-ca` клиент доверяет только системным корням).

`inspect` подключается дважды с общим кэшем сессий и отдельно меряет TCP- и TLS-рукопожатие:

```
соединение 1: версия=TLS 1.3 шифр=TLS_AES_128_GCM_SHA256 ALPN=http/1.1 DidResume=false TCP=40.45ms TLS=44.03ms статус=200 OK
соединение 2: версия=TLS 1.3 шифр=TLS_AES_128_GCM_SHA256 ALPN=http/1.1 DidResume=true TCP=40.44ms TLS=43.23ms статус=200 OK
```

Второе соединение возобновляет сессию (`DidResume=true`), но TLS по-прежнему занимает один RTT.
В TLS 1.3 возобновлённое рукопожатие (режим psk_dhe_ke) тоже делает обмен ключами ECDHE и тоже
длится один RTT; экономится отправка сертификатов и проверка цепочки, а не круг по сети.
В TLS 1.2 возобновление экономит RTT: полное рукопожатие — два RTT, сокращённое — один.
Режим 0-RTT (данные в первом же пакете) в `crypto/tls` из Go не реализован.

Проверка версии протокола через curl:

```bash
docker compose -f labs/0b/compose.yaml exec client curl --http1.1 -sk https://172.28.0.10:8443/ -w '%{http_version}\n'
docker compose -f labs/0b/compose.yaml exec client curl --http2 -sk https://172.28.0.10:8443/ -w '%{http_version}\n'
```
