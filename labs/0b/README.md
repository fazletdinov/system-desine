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
