# Лабораторные модуля 0b

Окружение: три контейнера в сети `labnet` (172.28.0.0/24) — `server` (172.28.0.10),
`client` (172.28.0.20) и `dns` на CoreDNS (172.28.0.53, зона `messenger.example`).

## Требования

- Docker с Compose;
- модуль ядра `sch_netem` (для имитации задержек и потерь): `sudo modprobe sch_netem`.

## Запуск

```bash
docker compose -f labs/0b/compose.yaml up -d --build
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

Модель (не измерение) для 120 мс: 20 мс голова строки до сервера + ~40 мс отложенный ACK сервера +
20 мс ACK назад + 20 мс хвост + 20 мс эхо; 80 мс — то же без отложенного ACK. Расклад проверяйте через `tcpdump`.

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
соединения и ждёт активные запросы не дольше `-grace` (15 с).

```bash
labs/0b/netem.sh 40ms 0%
docker compose -f labs/0b/compose.yaml exec -d server h2server -addr :8443 -http-addr :8080 -names api.messenger.example,server -ca-out /captures/ca.pem
```

#### httpbench — keep-alive, пул соединений, HTTP/2

`new_conns` — сколько новых соединений получили запросы (`httptrace`, `GotConn` с `Reused == false`).
Строки `connect` и `tls` — сколько TCP- и TLS-рукопожатий клиент на самом деле сделал и сколько они длились.
Задержка меряется до последнего байта тела.

```bash
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -keepalive=false -c 1 -n 20
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -keepalive=true -c 1 -n 20
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -c 10 -n 100 -max-idle-per-host 2
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -c 10 -n 100 -max-idle-per-host 10
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -http2=true -c 10 -n 100
```

Результат на нашем стенде (RTT 40 мс, без потерь; у вас числа будут другими, порядок — такой):

| запуск | new_conns | connect n | p50 | p99 |
|---|---|---|---|---|
| `-keepalive=false -c 1 -n 20` | 20 | 20 | 126,0 мс | 127,3 мс |
| `-keepalive=true -c 1 -n 20` | 1 | 1 | 41,5 мс | 123,6 мс |
| `-c 10 -n 100 -max-idle-per-host 2` | 10 | 10 | 41,3 мс | 134,8 мс |
| `-c 10 -n 100 -max-idle-per-host 10` | 10 | 10 | 41,1 мс | 134,3 мс |
| `-http2=true -c 10 -n 100` | 1 | 10 | 40,8 мс | 169,5 мс |

Без keep-alive каждый запрос — три RTT: TCP-рукопожатие, TLS 1.3-рукопожатие, сам запрос.
С keep-alive так платит только первый запрос на соединении — он и даёт p99.

При `-c 10` предел пула 2 и 10 неразличимы: воркеры забирают соединение обратно сразу после
ответа, и простаивающих больше двух почти не бывает. Предел виден, когда воркеров намного больше
или нагрузка неравномерна — например, при потерях (ниже):

```bash
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -c 50 -n 500 -max-idle-per-host 2    # connect: n=194
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -c 50 -n 500 -max-idle-per-host 50   # connect: n=50
```

Соединение, вернувшееся в переполненный пул, закрывается. Запрос, не нашедший свободного
соединения, начинает новое рукопожатие, но чаще успевает получить чужое освободившееся —
поэтому `new_conns` в нашем прогоне остался 50, а рукопожатий стало почти вчетверо больше.

HTTP/2 обслуживает все 100 запросов одним соединением (`new_conns=1`), но `connect n=10`:
десять одновременных первых запросов успели начать по рукопожатию, пока ни одно не завершилось
и ALPN не показал, что соединение можно делить. Девять из них клиент закрыл без единого запроса.

С потерями (`labs/0b/netem.sh 40ms 2%`, `-c 10 -n 100`, три прогона каждого):

```bash
labs/0b/netem.sh 40ms 2%
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -http2=true -c 10 -n 100
docker compose -f labs/0b/compose.yaml exec client httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -http2=false -c 10 -n 100
```


| режим | new_conns | connect n | p50 | p99 |
|---|---|---|---|---|
| `-http2=true` | 1 | 8–10 | 42,4–43,6 мс | 163–172 мс |
| `-http2=false` | 8–10 | 10–33 | 41,3–41,4 мс | 288–289 мс |

#### tlsdemo — проверка сертификата и возобновление сессии

```bash
docker compose -f labs/0b/compose.yaml exec client tlsdemo get -url https://api.messenger.example:8443/ -ca /captures/ca.pem
docker compose -f labs/0b/compose.yaml exec client tlsdemo get -url https://api.messenger.example:8443/ -ca /captures/ca.pem -server-name evil.example
docker compose -f labs/0b/compose.yaml exec client tlsdemo get -url https://api.messenger.example:8443/
docker compose -f labs/0b/compose.yaml exec client tlsdemo inspect -addr 172.28.0.10:8443 -server-name api.messenger.example -ca /captures/ca.pem
```

`get` печатает версию TLS, выбранный ALPN, цепочку сертификатов и ответ. Второй и третий
запуски падают с кодом 1: чужое имя и неизвестный CA (без `-ca` клиент доверяет только системным корням).
`inspect` подключается дважды с общим кэшем сессий: второе соединение — `DidResume=true`.
Время подключения при этом то же (около 2 RTT): в TLS 1.3 полное и возобновлённое рукопожатие
занимают по одному RTT; возобновление экономит проверку цепочки и обмен сертификатами, а не круги по сети.

Проверка версии протокола через curl:

```bash
docker compose -f labs/0b/compose.yaml exec client curl --http1.1 -sk https://172.28.0.10:8443/ -w '%{http_version}\n'
docker compose -f labs/0b/compose.yaml exec client curl --http2 -sk https://172.28.0.10:8443/ -w '%{http_version}\n'
```
