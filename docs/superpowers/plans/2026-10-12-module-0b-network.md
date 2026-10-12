# Модуль 0b «Сеть для бэкендера» — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Модуль 0b: 8 уроков о сети для Go-бэкендера, лабораторные в Docker Compose с `tc netem`, пять Go-программ уровня прода, тип `Simulation<S, A>` и три песочницы (TCP, пул соединений, «полосы» HTTP/1.1–HTTP/2–QUIC), четыре пошаговых плеера.

**Architecture:**
- **Песочницы и плееры** — чистые TS-модули в `course/viz/simulations/` с тестами Vitest. Тонкие Vue-рендереры и сцены в `VizFrame`.
- **Лабораторные** — отдельный Go-модуль `labs/` (без внешних зависимостей), окружение — `labs/0b/compose.yaml`. В уроки код попадает регионами из реальных файлов.
- **Уроки** — в `course/modules/00b-network/`, по стандартам основной спецификации.

**Tech Stack:**
- курс: VitePress 1.6.4, Vue 3.5.43, TypeScript 5.9.3, Vitest 3.2.7, Node 26;
- лабораторные: Go 1.26, Docker 29 / Compose v5, образ `golang:1.26.9-trixie`, `coredns/coredns:1.14.7`.

**Spec:**
- `docs/superpowers/specs/2026-10-12-module-0b-network-design.md` — главный.
- `docs/superpowers/specs/2026-10-09-course-platform-design.md` — §1, §2.2–2.5.

**Отступления от стандарта плана:**
- **Песочницы и чистая логика Go** даны в плане полным кодом и тестами.
- **`main.go` Go-программ, рендереры и сцены** даны **контрактом**: флаги, поведение, формат вывода, обязательные свойства. Иначе план вырос бы втрое. Код по контракту пишет исполнитель, ревьюер проверяет контракт.

## Global Constraints

- Команды npm — из `/home/idel/DevGo/system-desine/course`; Go-команды для лабораторных — из `/home/idel/DevGo/system-desine/labs`. Версии пакетов и образов — точные.
- Язык контента — русский; английский оригинал термина — в скобках при первом упоминании. Каждый термин при первом появлении в уроке — `<T id="…">` (id в двойных кавычках; неизвестный id — ошибка сборки).
- Тексты уроков пишутся по основной спецификации:
  - §1 «Голос автора»: упрощение без искажения, упрощения помечены;
  - §2.2 — все 10 правил, включая самодостаточность;
  - §2.3 — визуализации, у каждой полноэкранный режим;
  - §2.4 — метки `<Part>` у каждого `##`; `<N>` для 3–8 ключевых чисел (только результат формулы, не операнды, не в таблицах); `<Deep>` 150–500 слов.

  Числа — с источником, оценки помечены. Пересчитывать всю арифметику.
- Чистые TS-модули не импортируют `vue` и не трогают DOM на уровне модуля. В компонентах DOM трогается только в `onMounted`, обработчиках и `watch` без `immediate`; слушатели снимаются в `onBeforeUnmount`.
- Цвета только через CSS-переменные (`--viz-*`, `--vp-c-*`, `--course-*`, `--part-*`), обе темы, контраст текста ≥ 4,5:1, `prefers-reduced-motion`, 375 px без горизонтальной прокрутки, каждая сцена в `VizFrame`.
- Go-код (§2.5 и стандарт пользователя):
  - `context.Context` первым аргументом во всём, что ходит вовне;
  - таймауты везде;
  - graceful shutdown по SIGINT/SIGTERM;
  - ошибки оборачиваются через `fmt.Errorf("…: %w", err)`, без игнорирования;
  - логирование через `slog`, параметры — флагами;
  - у каждой горутины явный владелец и способ остановки;
  - `go vet`, `gofmt`, `go test -race` чистые;
  - без внешних зависимостей;
  - без слоёв domain/usecase — это утилиты.
- Каждый блок ` ```go ` в `course/modules/` помечен `<!-- go-check -->` (или с harness) и проходит `npm run check:go`. Фрагменты лабораторных подключаются через `<<< @/../labs/…#регион` и проверяются `npm run check:labs`.
- Вывод лабораторных в уроках — реальный, полученный прогоном. Пометка: «у вас числа будут другими, порядок — такой».
- Коммиты на русском, заканчиваются строкой `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Песочница TCP при окне 1–2 и потере.** Ожидается: повтор по таймеру (дубликатов не хватает на быстрый повтор), без зацикливания, симуляция заканчивается. Покрывают тесты C и ограничение `MAX_TICKS` в задаче 6.
2. **Лабораторная с netem, если хост без модуля `sch_netem`.** Ожидается: `netem.sh` падает с понятным сообщением, а не молча. Покрывает задача 2, шаг 5.
3. **`httpbench` под потерями и таймаутами.** Ожидается: запросы, упавшие по таймауту, считаются ошибками и попадают в отчёт, а не теряются, и программа завершается. Покрывает задача 4.
4. **Пул с `maxIdlePerHost` = 0 или параллельностью больше числа запросов.** Ожидается: корректный таймлайн без деления на ноль. Покрывают тесты `sanitize` в задаче 7.
5. **Полноэкранный оверлей.** Ожидается: Tab и Shift+Tab не уводят фокус на страницу под оверлеем, после выхода фокус на кнопке ⛶. Покрывает задача 1.

---

## Структура файлов

```
course/
├── viz/core/types.ts                 # + Simulation<S, A>
├── viz/components/VizFrame.vue        # удержание и возврат фокуса
├── viz/core/focus.ts (+ test)         # чистая логика «следующий фокусируемый по кругу»
├── viz/simulations/
│   ├── encapsulation.ts, dns-chain.ts, cert-chain.ts, udp-vs-tcp.ts (+ tests)
│   ├── tcp.ts (+ test), pool.ts (+ test), lanes.ts (+ test)
├── viz/renderers/  EncapsulationView, DnsChainView, CertChainView, UdpTcpView, TcpView, PoolView, LanesView .vue
├── viz/scenes/     Encapsulation, DnsChain, CertChain, UdpVsTcp, TcpSandbox, PoolSandbox, LanesSandbox .vue
├── scripts/check-labs.ts             # go vet/test/gofmt + docker compose config
├── glossary/terms.yaml               # + термины 0b
└── modules/00b-network/index.md, 01-layers.md … 08-http3-quic.md
labs/
├── go.mod
└── 0b/ compose.yaml, Dockerfile, netem.sh, dns/, captures/, README.md,
        internal/stats/, internal/udpstat/, cmd/tcpecho, cmd/udpping, cmd/httpbench, cmd/h2server, cmd/tlsdemo
```

---

### Task 1: Платформа — Simulation, фокус в оверлее, каркас модуля, термины

**Files:**
- Create: `course/viz/core/focus.ts`, `course/viz/core/focus.test.ts`, `course/modules/00b-network/index.md`
- Modify: `course/viz/core/types.ts`, `course/viz/components/VizFrame.vue`, `course/glossary/terms.yaml`, `docs/superpowers/specs/2026-10-09-course-platform-design.md` (§4.2), `docs/superpowers/backlog-after-module-0.md`

**Interfaces:**
- Produces:
  - `Simulation<S, A, F = S> { initial(): S; apply(state: S, action: A): S; steps(state: S): readonly Step<F>[] }`;
  - `nextFocusIndex(current: number, count: number, backwards: boolean): number`;
  - модуль 0b в сайдбаре;
  - термины для задач 5–11.

- [ ] **Step 1: Тип `Simulation` в `course/viz/core/types.ts`**

Дописать в конец файла:

```ts
// Песочница: действие пользователя даёт новое состояние, состояние — шаги для Player.
// F — состояние кадра, если кадры отличаются от состояния песочницы (например, TCP: настройки → кадры по тикам).
export interface Simulation<S, A, F = S> {
  initial: () => S
  apply: (state: S, action: A) => S
  steps: (state: S) => readonly Step<F>[]
}
```

- [ ] **Step 2: Написать падающий тест `course/viz/core/focus.test.ts`**

```ts
import { describe, expect, it } from 'vitest'
import { nextFocusIndex } from './focus'

describe('nextFocusIndex', () => {
  it.each([
    [0, 3, false, 1],
    [2, 3, false, 0],
    [0, 3, true, 2],
    [1, 3, true, 0],
    [-1, 3, false, 0],
    [-1, 3, true, 2],
  ])('current=%i count=%i back=%s → %i', (current, count, back, expected) => {
    expect(nextFocusIndex(current, count, back)).toBe(expected)
  })

  it('без элементов возвращает -1', () => {
    expect(nextFocusIndex(0, 0, false)).toBe(-1)
  })
})
```

Run: `npm test -- viz/core/focus.test.ts`
Expected: FAIL — нет модуля.

- [ ] **Step 3: Реализовать `course/viz/core/focus.ts`**

```ts
// Удержание фокуса в оверлее: Tab с последнего элемента ведёт на первый, Shift+Tab с первого — на последний.
// current = -1, если фокус сейчас вне списка.
export function nextFocusIndex(current: number, count: number, backwards: boolean): number {
  if (count <= 0) return -1
  if (current < 0) return backwards ? count - 1 : 0
  return (current + (backwards ? count - 1 : 1)) % count
}
```

Run: `npm test -- viz/core/focus.test.ts`
Expected: PASS.

- [ ] **Step 4: VizFrame удерживает и возвращает фокус**

В `course/viz/components/VizFrame.vue`:
- обёртке (`ref="frame"`) добавить `tabindex="-1"`, чтобы фокус можно было поставить на неё саму, когда внутри нет фокусируемых элементов;
- кнопке ⛶ — `ref="toggleBtn"`;
- в `onKeydown` при `mode !== 'off'` и `event.key === 'Tab'`:
  - собрать фокусируемые элементы внутри `frame` селектором `a[href], button:not([disabled]), input, select, textarea, [tabindex="0"]` (видимые);
  - найти индекс `document.activeElement`;
  - если `nextFocusIndex` даёт индекс ≥ 0, вызвать `preventDefault()` и перевести фокус на этот элемент; если элементов нет — `frame.focus()`;
  - работает и для `native`, и для `overlay`;
- при каждом переходе `mode` → `'off'` (кнопкой, по Esc в оверлее, по `fullscreenchange`): `nextTick(() => toggleBtn.value?.focus())`;
- если внутри нет фокусируемых, `focusContent()` переводит фокус на `frame`.

- [ ] **Step 5: Каркас модуля `course/modules/00b-network/index.md`**

```md
---
title: Модуль 0b. Сеть для бэкендера
order: 0.5
---

# Модуль 0b. Сеть для бэкендера

Между `http.Client.Do` и ответом сервера — DNS, TCP, TLS и HTTP. В уроке 0.3 мы прошли этот путь как модель с числами. Здесь разберём каждый слой изнутри и посмотрим на него вживую: запишем настоящие пакеты, добавим в сеть задержку и потери и увидим, как ведут себя TCP, пул соединений и HTTP/2.

В этом модуле вы:

- поймёте, что происходит на каждом уровне — от IP-пакета до HTTP;
- научитесь читать TCP-соединение через `ss`, `tcpdump` и Wireshark;
- настроите в Go пул соединений и таймауты так, чтобы они не подвели в проде;
- увидите разницу между HTTP/1.1, HTTP/2 и HTTP/3 на потерях пакетов.

Лабораторные запускаются в Docker Compose — инструкция в каждом уроке и в `labs/0b/README.md`.

[Начать с урока 0b.1](./01-layers)
```

Ссылка ведёт на файл задачи 8; до неё сборка с `cleanUrls` упадёт на мёртвой ссылке. Поэтому до задачи 8 строку `[Начать с урока 0b.1](./01-layers)` закомментировать как `<!-- … -->`. Задача 8 её раскомментирует.

- [ ] **Step 6: Термины модуля 0b в `course/glossary/terms.yaml`**

Добавить, если id ещё нет (проверить `grep -n "^<id>:"`). Для каждого: `term`, `en`, `short` — как ниже, `long` — 2–6 предложений в стиле файла (определение, аналогия, число, где в курсе), без `<T>`.

| id | term | en | short |
|---|---|---|---|
| `udp` | UDP | User Datagram Protocol | Протокол доставки отдельных датаграмм без соединения, подтверждений и порядка. |
| `socket` | Сокет | Socket | Конечная точка соединения в программе: адрес и порт; в ОС — файловый дескриптор. |
| `port` | Порт | Port | Число 0–65535, по которому ОС понимает, какой программе отдать пакет. |
| `mtu` | MTU | Maximum Transmission Unit | Максимальный размер пакета, который проходит через сетевой интерфейс без деления; в Ethernet — 1500 байт. |
| `encapsulation` | Инкапсуляция | Encapsulation | Каждый уровень сети заворачивает данные верхнего уровня в свой заголовок, как письмо в конверт. |
| `cwnd` | Окно перегрузки | Congestion window, cwnd | Сколько байт TCP-отправитель может держать в полёте без подтверждения, чтобы не перегрузить сеть. |
| `slow-start` | Медленный старт | Slow start | Начальная фаза TCP: окно перегрузки растёт вдвое каждый RTT, пока не случится потеря или порог. |
| `retransmission` | Повторная передача | Retransmission | Повторная отправка сегмента, который TCP считает потерянным: по таймеру или после трёх дубликатов ACK. |
| `time-wait` | TIME_WAIT | TIME_WAIT | Состояние закрытого TCP-соединения на стороне, закрывшей первой; в Linux длится 60 с. |
| `nagle` | Алгоритм Нейгла | Nagle's algorithm | Склеивание мелких записей в один сегмент, пока ждём ACK; в Go по умолчанию отключён (`TCP_NODELAY`). |
| `datagram` | Датаграмма | Datagram | Отдельное самостоятельное сообщение UDP: дойдёт целиком или не дойдёт вовсе. |
| `dns-record` | DNS-запись | DNS resource record | Строка в зоне DNS: имя, тип (A, AAAA, CNAME, MX, TXT…), значение и TTL. |
| `ttl` | TTL | Time To Live | Сколько секунд ответ разрешено хранить в кэше (в DNS); в IP — счётчик переходов. |
| `resolver` | Резолвер | Resolver | Программа или сервер, который находит IP-адрес по имени, обходя DNS-серверы и кэшируя ответы. |
| `certificate` | Сертификат | X.509 certificate | Документ «этот открытый ключ принадлежит этому имени», подписанный центром сертификации. |
| `ca` | Центр сертификации | Certificate Authority, CA | Организация, чьей подписи на сертификатах доверяют клиенты; корневые CA встроены в ОС и браузеры. |
| `sni` | SNI | Server Name Indication | Расширение TLS: клиент сообщает имя сайта в начале рукопожатия, чтобы сервер выбрал нужный сертификат. |
| `alpn` | ALPN | Application-Layer Protocol Negotiation | Расширение TLS, через которое клиент и сервер договариваются о протоколе — h2 или http/1.1. |
| `session-resumption` | Возобновление TLS-сессии | Session resumption | Повторное подключение по сохранённому секрету без полного рукопожатия и проверки сертификата. |
| `keep-alive` | Keep-alive | HTTP persistent connection | Повторное использование одного TCP-соединения для нескольких HTTP-запросов. |
| `connection-pool` | Пул соединений | Connection pool | Набор открытых соединений, которые программа держит и переиспользует вместо открытия новых. |
| `multiplexing` | Мультиплексирование | Multiplexing | Передача нескольких независимых потоков по одному соединению вперемешку. |
| `head-of-line-blocking` | Блокировка начала очереди | Head-of-line blocking | Задержка всех следующих данных из-за одного застрявшего впереди пакета или запроса. |
| `hpack` | HPACK | HPACK | Сжатие HTTP-заголовков в HTTP/2 через общий словарь уже переданных заголовков. |
| `quic` | QUIC | QUIC | Транспорт поверх UDP с встроенным TLS 1.3 и независимыми потоками; основа HTTP/3. |
| `zero-rtt` | 0-RTT | Zero round-trip time resumption | Отправка данных в первом же пакете повторного подключения; ценой возможного повтора запроса. |
| `netem` | netem | Network emulator | Модуль Linux `tc`, который добавляет на интерфейс задержку, потери и переупорядочивание пакетов. |
| `pcap` | pcap | Packet capture | Формат файла с записанными сетевыми пакетами; его пишет `tcpdump` и открывает Wireshark. |

- [ ] **Step 7: Синхронизация документов**

- В спецификации основной, §4.2: фразу о том, что `Simulation<S, A>` вводится в модуле 2.5, заменить на «вводится в модуле 0b (песочницы TCP, пула, полос)» и показать интерфейс из шага 1.
- В бэклоге: пункт про фокус в оверлее перенести в «Выполнено».

- [ ] **Step 8: Проверить**

Run: `npm test && npm run typecheck && npm run build`
Expected: PASS. В сайдбаре модуль «Модуль 0b. Сеть для бэкендера» стоит после модуля 0, над заголовком — «МОДУЛЬ 0B».

Playwright на уроке 0.3 (1280):
- открыть ⛶ у визуализации;
- нажать Tab столько раз, сколько в ней кнопок, плюс 2 — фокус ни разу не покинул обёртку (`frame.contains(document.activeElement)`);
- Shift+Tab — так же;
- Esc → `document.activeElement` — кнопка ⛶.

Повторить с эмуляцией оверлея: `Element.prototype.requestFullscreen = () => Promise.reject(new Error('x'))`.

- [ ] **Step 9: Commit**

```bash
git add course docs
git commit -m "feat(course): Simulation, удержание фокуса в оверлее, каркас модуля 0b и его термины

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Окружение лабораторных

**Files:**
- Create: `labs/go.mod`, `labs/0b/Dockerfile`, `labs/0b/compose.yaml`, `labs/0b/netem.sh`, `labs/0b/dns/Corefile`, `labs/0b/dns/messenger.example.db`, `labs/0b/README.md`, `labs/0b/captures/.gitkeep`, `course/scripts/check-labs.ts`
- Modify: корневой `.gitignore`, `course/package.json`

**Interfaces:**
- Produces:
  - сервисы Compose `server` (IP `172.28.0.10`), `client` (`172.28.0.20`), `dns` (`172.28.0.53`) в сети `labnet` 172.28.0.0/24;
  - `./netem.sh <rtt> <loss>` и `./netem.sh off`;
  - бинарники лабораторных в образе: `/usr/local/bin/{tcpecho,udpping,httpbench,h2server,tlsdemo}` (появятся по мере задач 3–4; Dockerfile собирает `./0b/cmd/...`);
  - `npm run check:labs`.

- [ ] **Step 1: `labs/go.mod`**

```
module labs

go 1.26
```

Плюс заглушка, чтобы модуль собирался до задачи 3: `labs/0b/internal/stats/doc.go` с `// Package stats — статистика задержек для лабораторных.` и `package stats`.

- [ ] **Step 2: `labs/0b/Dockerfile`**

```dockerfile
FROM golang:1.26.9-trixie

RUN apt-get update \
 && apt-get install -y --no-install-recommends tcpdump iproute2 iputils-ping curl dnsutils openssl procps \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod ./
COPY 0b ./0b
# Собираем все программы лабораторной; пока cmd/ пуст, шаг ничего не делает.
RUN set -e; for d in 0b/cmd/*/; do [ -d "$d" ] || continue; go build -o /usr/local/bin/$(basename "$d") ./"$d"; done

WORKDIR /lab
CMD ["sleep", "infinity"]
```

- [ ] **Step 3: `labs/0b/compose.yaml`**

```yaml
name: lab0b

x-node: &node
  build:
    context: ..
    dockerfile: 0b/Dockerfile
  image: lab0b-node:local
  cap_add: [NET_ADMIN, NET_RAW]
  volumes:
    - ./captures:/captures
  dns: [172.28.0.53]

services:
  server:
    <<: *node
    hostname: server
    networks:
      labnet: { ipv4_address: 172.28.0.10 }
  client:
    <<: *node
    hostname: client
    networks:
      labnet: { ipv4_address: 172.28.0.20 }
  dns:
    image: coredns/coredns:1.14.7
    command: ["-conf", "/etc/coredns/Corefile"]
    volumes:
      - ./dns:/etc/coredns:ro
    networks:
      labnet: { ipv4_address: 172.28.0.53 }

networks:
  labnet:
    ipam:
      config:
        - subnet: 172.28.0.0/24
```

- [ ] **Step 4: DNS-зона**

`labs/0b/dns/Corefile`:

```
messenger.example {
    file /etc/coredns/messenger.example.db
    log
}
. {
    forward . 1.1.1.1 8.8.8.8
    cache 30
    log
}
```

`labs/0b/dns/messenger.example.db`:

```
$ORIGIN messenger.example.
$TTL 30
@       IN SOA ns.messenger.example. admin.messenger.example. 2026101201 3600 600 86400 30
@       IN NS  ns.messenger.example.
ns      IN A   172.28.0.53
api     IN A   172.28.0.10
www     IN CNAME api.messenger.example.
_api._tcp IN SRV 10 5 8443 api.messenger.example.
@       IN TXT "lab=0b"
```

- [ ] **Step 5: `labs/0b/netem.sh`**

```bash
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
```

`chmod +x labs/0b/netem.sh`.

- [ ] **Step 6: `.gitignore` и README**

В корневой `.gitignore` добавить `labs/0b/captures/*.pcap`.

`labs/0b/README.md`:
- требования: Docker с Compose, модуль ядра `sch_netem`;
- запуск: `docker compose -f labs/0b/compose.yaml up -d --build`;
- netem: `labs/0b/netem.sh 40ms 1%`;
- запись трафика: `… exec client tcpdump -i eth0 -w /captures/x.pcap`;
- остановка: `… down`.

Раздел «Лабораторные по урокам» пока пустой, его дополняют задачи 3–4 и 8–11.

- [ ] **Step 7: `course/scripts/check-labs.ts` и npm-скрипт**

```ts
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const LABS = fileURLToPath(new URL('../../labs', import.meta.url))

function run(cmd: string, args: string[], cwd = LABS): string {
  try {
    return execFileSync(cmd, args, { cwd, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
  } catch (e) {
    const err = e as { stderr?: string; stdout?: string; message: string }
    console.error(`${cmd} ${args.join(' ')}:\n${err.stdout ?? ''}${err.stderr ?? err.message}`)
    process.exit(1)
  }
}

run('go', ['version'])
const unformatted = run('gofmt', ['-l', '.']).trim()
if (unformatted !== '') {
  console.error(`gofmt требует форматирования:\n${unformatted}`)
  process.exit(1)
}
run('go', ['vet', './...'])
run('go', ['test', '-race', './...'])
run('docker', ['compose', '-f', '0b/compose.yaml', 'config', '-q'])
console.log('Лабораторные: gofmt, go vet, go test -race и compose config — без ошибок.')
```

В `course/package.json` → `scripts`: `"check:labs": "node scripts/check-labs.ts"`.

- [ ] **Step 8: Проверить окружение вручную**

Run (из корня репозитория):

```bash
docker compose -f labs/0b/compose.yaml up -d --build
labs/0b/netem.sh 40ms 0%
docker compose -f labs/0b/compose.yaml exec client ping -c 5 172.28.0.10
docker compose -f labs/0b/compose.yaml exec client dig +short api.messenger.example
labs/0b/netem.sh off
```

Expected:
- ping показывает `rtt min/avg/max` около 40 мс;
- `dig` возвращает `172.28.0.10`.

Затем `cd course && npm run check:labs` → сообщение об успехе. Окружение оставить запущенным только на время проверки, потом `docker compose -f labs/0b/compose.yaml down`.

- [ ] **Step 9: Commit**

```bash
git add labs .gitignore course/scripts/check-labs.ts course/package.json
git commit -m "feat(labs): окружение лабораторных 0b — Compose, netem, CoreDNS, check:labs

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Go-лабораторные: статистика, эхо, UDP-пинг

**Files:**
- Create: `labs/0b/internal/stats/stats.go`, `labs/0b/internal/stats/stats_test.go`, `labs/0b/internal/udpstat/udpstat.go`, `labs/0b/internal/udpstat/udpstat_test.go`, `labs/0b/cmd/tcpecho/main.go`, `labs/0b/cmd/udpping/main.go`
- Remove: `labs/0b/internal/stats/doc.go`
- Modify: `labs/0b/README.md`

**Interfaces:**
- Consumes: окружение задачи 2.
- Produces:
  - `stats.Percentile(sorted []time.Duration, p float64) time.Duration`;
  - `stats.Summary(ds []time.Duration) Summary{N int; Min, P50, P99, Max time.Duration}`;
  - `udpstat.Analyze(sent int, replies []Reply) Result`;
  - бинарники `tcpecho`, `udpping`;
  - регионы для уроков: `tcpecho/main.go#server`, `tcpecho/main.go#client`, `udpping/main.go#client`.

- [ ] **Step 1: Написать падающие тесты `stats_test.go`**

```go
package stats

import (
	"testing"
	"time"
)

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func TestPercentile(t *testing.T) {
	sorted := []time.Duration{ms(1), ms(2), ms(3), ms(4), ms(5), ms(6), ms(7), ms(8), ms(9), ms(100)}
	tests := []struct {
		name string
		p    float64
		want time.Duration
	}{
		{"медиана — 5-е значение по nearest-rank", 50, ms(5)},
		{"p90 — 9-е значение", 90, ms(9)},
		{"p99 — последнее", 99, ms(100)},
		{"p0 — первое", 0, ms(1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Percentile(sorted, tt.p); got != tt.want {
				t.Fatalf("Percentile(%v) = %v, want %v", tt.p, got, tt.want)
			}
		})
	}
}

func TestPercentileEmpty(t *testing.T) {
	if got := Percentile(nil, 50); got != 0 {
		t.Fatalf("got %v, want 0", got)
	}
}

func TestSummaryDoesNotMutateInput(t *testing.T) {
	in := []time.Duration{ms(3), ms(1), ms(2)}
	s := Summary(in)
	if s.N != 3 || s.Min != ms(1) || s.Max != ms(3) || s.P50 != ms(2) {
		t.Fatalf("unexpected summary %+v", s)
	}
	if in[0] != ms(3) {
		t.Fatal("Summary must not sort the caller's slice")
	}
}
```

Run: `cd labs && go test ./0b/internal/stats/`
Expected: FAIL — нет функций.

- [ ] **Step 2: Реализовать `stats.go`**

```go
// Package stats — статистика задержек для лабораторных.
package stats

import (
	"math"
	"slices"
	"time"
)

// Percentile — метод nearest-rank: наименьшее значение, ниже или равно которому доля p% измерений.
func Percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	rank = max(1, min(rank, len(sorted)))
	return sorted[rank-1]
}

type Result struct {
	N                  int
	Min, P50, P99, Max time.Duration
}

// Summary не сортирует срез вызывающего: копируем, чтобы не было скрытых побочных эффектов.
func Summary(ds []time.Duration) Result {
	if len(ds) == 0 {
		return Result{}
	}
	sorted := slices.Clone(ds)
	slices.Sort(sorted)
	return Result{N: len(sorted), Min: sorted[0], P50: Percentile(sorted, 50), P99: Percentile(sorted, 99), Max: sorted[len(sorted)-1]}
}
```

Тип называется `Result`; в тестах и интерфейсах — `stats.Summary(...)` возвращает `stats.Result`. Удалить `doc.go`.

Run: `go test ./0b/internal/stats/`
Expected: PASS.

- [ ] **Step 3: Написать падающие тесты `udpstat_test.go`**

```go
package udpstat

import (
	"testing"
	"time"
)

func TestAnalyze(t *testing.T) {
	tests := []struct {
		name    string
		sent    int
		replies []Reply
		want    Result
	}{
		{"все дошли по порядку", 3, []Reply{{0, 1}, {1, 1}, {2, 1}}, Result{Sent: 3, Received: 3}},
		{"одна потеря", 3, []Reply{{0, 1}, {2, 1}}, Result{Sent: 3, Received: 2, Lost: 1}},
		{"переупорядочивание", 3, []Reply{{0, 1}, {2, 1}, {1, 1}}, Result{Sent: 3, Received: 3, Reordered: 1}},
		{"дубликат", 2, []Reply{{0, 1}, {0, 1}, {1, 1}}, Result{Sent: 2, Received: 2, Duplicates: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Analyze(tt.sent, tt.replies)
			got.RTTs = nil
			if got != tt.want {
				t.Fatalf("Analyze() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeCollectsRTTsOfUniqueReplies(t *testing.T) {
	got := Analyze(2, []Reply{{0, time.Millisecond}, {0, 5 * time.Millisecond}, {1, 2 * time.Millisecond}})
	if len(got.RTTs) != 2 {
		t.Fatalf("want 2 RTTs, got %v", got.RTTs)
	}
}
```

`Reply` — `struct{ Seq uint32; RTT time.Duration }`, литералы в тестах позиционные. В `Result` поле `RTTs []time.Duration`. Из-за среза `Result` нельзя сравнивать через `!=`, поэтому сравнение `got != tt.want` заменить на сравнение структуры без `RTTs`: вынести счётчики во вложенную структуру `Counts` и сравнивать её. Тест и тип скорректировать согласованно:

```go
type Counts struct{ Sent, Received, Lost, Duplicates, Reordered int }
type Result struct {
	Counts
	RTTs []time.Duration
}
```

В таблице — `want Counts`, сравнение `got.Counts != tt.want`. Строку `got.RTTs = nil` убрать.

Run: `go test ./0b/internal/udpstat/`
Expected: FAIL.

- [ ] **Step 4: Реализовать `udpstat.go`**

```go
// Package udpstat считает потери, дубликаты и переупорядочивание UDP-ответов.
package udpstat

import "time"

type Reply struct {
	Seq uint32
	RTT time.Duration
}

type Counts struct{ Sent, Received, Lost, Duplicates, Reordered int }

type Result struct {
	Counts
	RTTs []time.Duration
}

// Analyze: replies — в порядке прихода. Переупорядоченным считается ответ,
// пришедший после ответа с большим номером.
func Analyze(sent int, replies []Reply) Result {
	seen := make(map[uint32]bool, len(replies))
	r := Result{Counts: Counts{Sent: sent}}
	var maxSeq uint32
	var any bool
	for _, rep := range replies {
		if seen[rep.Seq] {
			r.Duplicates++
			continue
		}
		seen[rep.Seq] = true
		r.Received++
		r.RTTs = append(r.RTTs, rep.RTT)
		if any && rep.Seq < maxSeq {
			r.Reordered++
		}
		if !any || rep.Seq > maxSeq {
			maxSeq = rep.Seq
		}
		any = true
	}
	r.Lost = sent - r.Received
	return r
}
```

Run: `go test -race ./...`
Expected: PASS.

- [ ] **Step 5: `cmd/tcpecho/main.go` — контракт**

**Режимы:** `tcpecho -serve -addr :7000` и `tcpecho -addr server:7000 -n 20 -interval 200ms -size 32`.

**Сервер:**
- `net.ListenConfig.Listen(ctx, "tcp", addr)`;
- на каждое соединение — горутина, которой владеет `errgroup`-подобная структура или `sync.WaitGroup` + контекст;
- читает строки и пишет их обратно;
- `SetDeadline` на чтение (флаг `-idle 60s`): простаивающее соединение закрывается;
- по SIGINT/SIGTERM (`signal.NotifyContext`) перестаёт принимать, закрывает листенер, ждёт активные соединения до `-shutdown 5s`, затем закрывает их;
- лог через `slog`: адрес клиента, число строк, причина закрытия.

**Клиент:**
- `net.Dialer{Timeout: -dial}` через `DialContext`;
- N раз: записать строку `seq=<i> <padding до size>\n`, прочитать ответ (дедлайн `-timeout 5s`), замерить RTT;
- в конце печатает: `connect: <время рукопожатия>` и строку `rtt: n=… min=… p50=… p99=… max=…` через `stats.Summary`.
- Флаг `-nodelay=true` (по умолчанию как в Go). При `false` клиент отключает `TCP_NODELAY` — для демонстрации Нейгла.

**Регионы для уроков:** `// #region server` … `// #endregion server` вокруг цикла приёма и обработки соединения; `// #region client` вокруг цикла запрос/ответ с замером.

- [ ] **Step 6: `cmd/udpping/main.go` — контракт**

**Режимы:** `udpping -serve -addr :7001` и `udpping -addr server:7001 -n 200 -interval 10ms -wait 1s`.

**Сервер:** `net.ListenPacket`, отвечает тем же содержимым тому же адресу; shutdown по сигналу.

**Клиент:**
- датаграмма = 4 байта seq (big endian) + 8 байт времени отправки (UnixNano);
- отправка в одной горутине, приём — в другой; владелец обеих — функция `run(ctx)`;
- приём завершается через `-wait` после последней отправки;
- RTT по времени из пакета;
- итог печатается как `sent=… received=… lost=… (…%) dup=… reordered=…` и `rtt: …` через `udpstat.Analyze` и `stats.Summary`.

**Регион** `// #region client` — отправка с номером и подсчёт.

- [ ] **Step 7: Проверить вручную под netem**

```bash
docker compose -f labs/0b/compose.yaml up -d --build
labs/0b/netem.sh 40ms 0%
docker compose -f labs/0b/compose.yaml exec -d server tcpecho -serve -addr :7000
docker compose -f labs/0b/compose.yaml exec client tcpecho -addr 172.28.0.10:7000 -n 20
labs/0b/netem.sh 40ms 5%
docker compose -f labs/0b/compose.yaml exec -d server udpping -serve -addr :7001
docker compose -f labs/0b/compose.yaml exec client udpping -addr 172.28.0.10:7001 -n 200
docker compose -f labs/0b/compose.yaml exec client ss -ti dst 172.28.0.10
labs/0b/netem.sh off && docker compose -f labs/0b/compose.yaml down
```

Expected:
- `tcpecho`: p50 около 40 мс;
- `udpping` при 5% потерь на каждом конце: `lost` примерно 10% (≈ 1 − 0,95²); точное число будет другим.

Вывод сохранить в отчёт: он пойдёт в уроки.

- [ ] **Step 8: README и проверки**

В `labs/0b/README.md` добавить разделы «tcpecho» и «udpping» с командами из шага 7.

Run:
- `cd course && npm run check:labs` — успех;
- `go test -race ./...` из `labs/` — PASS.

- [ ] **Step 9: Commit**

```bash
git add labs
git commit -m "feat(labs): статистика задержек, TCP-эхо и UDP-пинг для лабораторных 0b

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Go-лабораторные: httpbench, h2server, tlsdemo

**Files:**
- Create: `labs/0b/internal/certs/certs.go`, `labs/0b/internal/certs/certs_test.go`, `labs/0b/cmd/httpbench/main.go`, `labs/0b/cmd/h2server/main.go`, `labs/0b/cmd/tlsdemo/main.go`
- Modify: `labs/0b/README.md`

**Interfaces:**
- Consumes: `stats.Summary` (задача 3).
- Produces:
  - `certs.NewCA(cn string, ttl time.Duration) (*CA, error)`;
  - `(*CA).Issue(names []string, ttl time.Duration) (tls.Certificate, error)`;
  - `(*CA).Pool() *x509.CertPool`;
  - `(*CA).PEM() []byte`;
  - бинарники `httpbench`, `h2server`, `tlsdemo`;
  - регионы `httpbench/main.go#transport`, `httpbench/main.go#trace`, `h2server/main.go#server`, `tlsdemo/main.go#client-config`.

- [ ] **Step 1: Написать падающие тесты `certs_test.go`**

```go
package certs

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"
)

func TestIssuedCertificateVerifiesForItsNameOnly(t *testing.T) {
	ca, err := NewCA("lab-ca", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := ca.Issue([]string{"api.messenger.example"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		dnsName string
		wantErr bool
	}{
		{"своё имя проходит", "api.messenger.example", false},
		{"чужое имя не проходит", "evil.example", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := leaf.Verify(x509.VerifyOptions{Roots: ca.Pool(), DNSName: tt.dnsName})
			if (err != nil) != tt.wantErr {
				t.Fatalf("Verify err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnknownCAIsRejected(t *testing.T) {
	ca, _ := NewCA("lab-ca", time.Hour)
	other, _ := NewCA("other-ca", time.Hour)
	cert, _ := ca.Issue([]string{"api.messenger.example"}, time.Hour)
	leaf, _ := x509.ParseCertificate(cert.Certificate[0])
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: other.Pool(), DNSName: "api.messenger.example"}); err == nil {
		t.Fatal("certificate from unknown CA must be rejected")
	}
}

func TestTLSHandshakeWithIssuedCert(t *testing.T) {
	ca, _ := NewCA("lab-ca", time.Hour)
	cert, _ := ca.Issue([]string{"api.messenger.example"}, time.Hour)
	srv := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}
	cli := &tls.Config{RootCAs: ca.Pool(), ServerName: "api.messenger.example", MinVersion: tls.VersionTLS13}
	if err := handshakeOverPipe(srv, cli); err != nil {
		t.Fatal(err)
	}
}
```

`handshakeOverPipe` — тестовый хелпер в том же файле: `net.Pipe()`, `tls.Server`/`tls.Client`, оба `HandshakeContext` в двух горутинах через `errgroup`-подобный сбор ошибок и контекст с таймаутом 5 с. Внешних зависимостей нет — свой `sync.WaitGroup` + канал ошибок.

Run: `go test ./0b/internal/certs/`
Expected: FAIL.

- [ ] **Step 2: Реализовать `certs.go`**

Требования:
- ключи ECDSA P-256;
- CA: `IsCA: true`, `KeyUsageCertSign`, `BasicConstraintsValid`;
- лист: `DNSNames: names`, `ExtKeyUsageServerAuth`, серийный номер — случайный 128-бит;
- `NotBefore = now - 1m` (запас на расхождение часов).

Все ошибки обёрнуты: `fmt.Errorf("issue certificate: %w", err)`.

Run: `go test -race ./...`
Expected: PASS.

- [ ] **Step 3: `cmd/h2server/main.go` — контракт**

**Запуск:** `h2server -addr :8443 -http-addr :8080 -names api.messenger.example,server -ca-out /captures/ca.pem`.
- Создаёт CA и сертификат через `certs`, пишет `ca.pem` для клиентов.
- Поднимает:
  - HTTPS на `-addr` (HTTP/2 включается автоматически через ALPN);
  - HTTP/1.1 без TLS на `-http-addr`.

**Обработчики:**
- `/` — `proto=<r.Proto> remote=<r.RemoteAddr>`;
- `/bytes?n=<N>` — N байт (до 10 МБ, иначе 400);
- `/slow?ms=<N>` — ждёт N мс (до 10 с), учитывая `r.Context()`.

**Таймауты сервера:** `ReadHeaderTimeout: 5s`, `ReadTimeout: 30s`, `WriteTimeout: 30s`, `IdleTimeout: 90s`.

**Остальное:** graceful shutdown обоих серверов через `Shutdown(ctx)`; `slog` логирует метод, путь, протокол и длительность.

**Регион** `#server` — конфигурация `http.Server` с таймаутами.

- [ ] **Step 4: `cmd/httpbench/main.go` — контракт**

**Запуск:** `httpbench -url https://172.28.0.10:8443/ -ca /captures/ca.pem -server-name api.messenger.example -n 100 -c 10 -keepalive=true -max-idle-per-host 2 -timeout 5s -http2=false`.

**Транспорт (регион `#transport`):**
- `http.Transport` с `MaxIdleConns: 100`, `MaxIdleConnsPerHost: <флаг>`, `IdleConnTimeout: 90s`, `TLSHandshakeTimeout: 5s`, `ResponseHeaderTimeout: <timeout>`, `DisableKeepAlives: !keepalive`;
- `TLSClientConfig` с `RootCAs` из файла и `ServerName`;
- при `-http2=false` — `TLSNextProto: map[string]func(...){}`, чтобы честно сравнивать HTTP/1.1;
- `http.Client{Timeout: <timeout>}`.

**Счёт соединений (регион `#trace`):** `httptrace.ClientTrace` считает новые соединения (`GotConn` с `!info.Reused`), а также время DNS, соединения и TLS.

**Выполнение:**
- `c` воркеров берут задания из канала; горутинами владеет функция `run(ctx)`, завершение — через `sync.WaitGroup`;
- каждое тело читается до конца (`io.Copy(io.Discard, …)`) и закрывается — иначе соединение не вернётся в пул;
- ошибки считаются и группируются по типу (таймаут, отказ соединения, прочее), а не теряются.

**Вывод:**

```
requests=100 errors=0 new_conns=… proto=HTTP/1.1
latency: n=… min=… p50=… p99=… max=…
total=…
```

- [ ] **Step 5: `cmd/tlsdemo/main.go` — контракт**

**Подкоманды:**
- `tlsdemo get -url https://api.messenger.example:8443/ -ca /captures/ca.pem [-server-name X]`;
- `tlsdemo inspect -addr 172.28.0.10:8443 -server-name api.messenger.example -ca /captures/ca.pem`.

**`get`:** клиент с `tls.Config{RootCAs, ServerName, MinVersion: tls.VersionTLS12}` (регион `#client-config`). Выводит:
- версию TLS;
- согласованный ALPN;
- цепочку (субъект, издатель, срок);
- ответ.

**`inspect`:** голое `tls.Dialer.DialContext`, печатает `ConnectionState`: версия, шифр, ALPN, `DidResume`, длительность рукопожатия. Делает два подключения подряд с общим `ClientSessionCache`, чтобы показать возобновление сессии.

**Ошибки проверки** (неизвестный CA, неверное имя) печатаются понятным текстом с исходной ошибкой; код возврата 1.

- [ ] **Step 6: Проверить вручную под netem**

Поднять окружение, `netem.sh 40ms 0%`, запустить `h2server` в `server`. Затем:
1. `httpbench` ×4:
   - `-keepalive=false -c 1 -n 20`;
   - `-keepalive=true -c 1 -n 20`;
   - `-c 10 -n 100 -max-idle-per-host 2`;
   - `-c 10 -n 100 -max-idle-per-host 10`.
2. `-http2=true -c 10 -n 100`.
3. `tlsdemo get` с правильным и неверным `-server-name`, `tlsdemo inspect`.
4. `curl --http1.1 -sk https://172.28.0.10:8443/ -w '%{http_version}\n'` и то же с `--http2`.
5. `netem.sh 40ms 2%`: `httpbench -http2=true` и `-http2=false` с `-c 10 -n 100`.

Весь вывод сохранить в отчёт.

Expected:
- без keep-alive каждый запрос ≈ 3 RTT (TCP + TLS + запрос) ≈ 120 мс;
- с keep-alive ≈ 1 RTT ≈ 40 мс после первого;
- при `max-idle-per-host 2` `new_conns` заметно больше 10, при 10 — ровно 10.

- [ ] **Step 7: README, проверки, commit**

README: разделы `h2server`, `httpbench`, `tlsdemo` с командами шага 6.

Run: `npm run check:labs` — успех.

```bash
git add labs
git commit -m "feat(labs): httpbench, h2server и tlsdemo для лабораторных 0b

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Пошаговые плееры — инкапсуляция, DNS, цепочка сертификатов, UDP против TCP

**Files:**
- Create:
  - `course/viz/simulations/{encapsulation,dns-chain,cert-chain,udp-vs-tcp}.ts` + `.test.ts`;
  - `course/viz/renderers/{EncapsulationView,DnsChainView,CertChainView,UdpTcpView}.vue`;
  - `course/viz/scenes/{Encapsulation,DnsChain,CertChain,UdpVsTcp}.vue`.

**Interfaces:**
- Consumes: `Step`, `Scenario`, `CaptionSegment` (`course/viz/core/caption.ts`), `Player`, `VizFrame`, `formatNumber`.
- Produces: сцены для уроков 0b.1, 0b.3, 0b.4, 0b.5.

**Общий контракт сценариев:**
- `export function <name>Scenario(): Step<State>[]`;
- подписи — русские, термины сегментами `{ text, termId }` с id из глоссария;
- в каждом файле — экспорт констант с числами, которые использует урок.

**Тесты** для каждого сценария (`it` по пунктам):
1. число шагов в разумных пределах (указано ниже);
2. все `termId` в подписях есть в `terms.yaml` (как в `request-path.test.ts`);
3. инварианты, указанные ниже.

- [ ] **Step 1: `encapsulation.ts`**

**Состояние:** `{ layers: readonly Layer[]; active: number }`, где `Layer = { id: 'http'|'tls'|'tcp'|'ip'|'eth'; label: string; headerBytes: number; trailerBytes: number }`.

**Шаги (6):**
- шаг 0 — только HTTP-тело (`payload = 200` байт);
- затем добавляются слои: TLS (5 + 16 + 1 = 22 байта), TCP (20 + 12 опций = 32), IP (20), Ethernet (14 + 4 FCS = 18);
- последний шаг — итог.

**Инварианты:**
- общий размер на шаге k = 200 + сумма заголовков добавленных слоёв;
- на последнем шаге 200 + 22 + 32 + 20 + 18 = 292;
- подпись последнего шага содержит «292» и долю полезных данных ≈ 68%.

**Рендерер:** вложенные прямоугольники-«конверты» с подписью размера; цвета слоёв из `--part-*` или `--viz-*`, по одному на слой; в узкой раскладке — вертикальный стек.

- [ ] **Step 2: `dns-chain.ts`**

**Узлы:** `stub`, `recursive`, `root`, `tld`, `auth`.

**Шаги:**
- холодный кэш: stub → recursive → root (реферал на `.example`) → tld (реферал на `ns.messenger.example`) → auth (A 172.28.0.10, TTL 30) → recursive кэширует → stub;
- затем второй запрос: stub → recursive → сразу ответ из кэша.

Около 12 шагов. Состояние: `{ active: NodeId; packet: { from: NodeId; to: NodeId; kind: 'query'|'referral'|'answer' } | null; cache: readonly string[]; elapsedMs: number }`.

**Задержки:**
- stub ↔ recursive — 20 мс в обе стороны (половина 40-мс RTT, как в 0.3);
- recursive ↔ любой авторитетный — 10 мс в одну сторону.

**Инварианты:**
- время не убывает;
- первый полный ответ — на 20 + 3·20 + 20 = 100 мс;
- второй запрос добавляет ровно 40 мс;
- после ответа `auth` в `cache` есть запись с `api.messenger.example`.

**Рендерер:** узлы в ряд (в узкой раскладке — столбец), движение пакета через GSAP (как `RequestPathView`, с `prefersReducedMotion` и `killTweensOf`), блок «кэш резолвера».

- [ ] **Step 3: `cert-chain.ts`**

**Шаги:**
1. сервер присылает лист `api.messenger.example` и промежуточный сертификат;
2. проверка подписи листа ключом промежуточного;
3. проверка подписи промежуточного ключом корня из хранилища ОС;
4. проверка срока;
5. проверка имени (SAN);
6. успех.

Затем ветка-ошибка: «корень не найден в хранилище → x509: certificate signed by unknown authority».

**Состояние:** `{ certs: readonly { id: 'leaf'|'intermediate'|'root'; subject: string; issuer: string; status: 'pending'|'ok'|'fail' }[]; check: string }`.

**Инварианты:**
- статусы меняются только `pending → ok|fail`;
- в ветке ошибки корень в статусе `fail`, текст ошибки содержит `unknown authority`.

- [ ] **Step 4: `udp-vs-tcp.ts`**

**Две дорожки,** 6 сообщений, потеряно сообщение 3.
- **TCP:** 4–6 приходят, но приложению не отдаются, пока сообщение 3 не повторено через RTO. Модель: 1 тик в одну сторону, RTO = 4 тика.
- **UDP:** 4–6 отдаются сразу, сообщение 3 потеряно навсегда.

**Состояние:** `{ tick: number; tcp: Lane; udp: Lane }`, где `Lane = { delivered: readonly number[]; buffered: readonly number[]; lost: readonly number[] }`.

**Инварианты:**
- в итоге TCP доставил 1–6 по порядку;
- UDP доставил 1, 2, 4, 5, 6;
- TCP закончил позже UDP.

- [ ] **Step 5: Сцены**

Каждая сцена: `<VizFrame label="…"><Player :steps="…" label="…"><template #default="{ step }"><…View :state="step.state" /></template></Player></VizFrame>`.

Рендереры:
- SVG с текстом не меньше 12 px на 375 (правило из `viz/README.md`);
- узкая раскладка через `ResizeObserver`, как в `RequestPathView`.

- [ ] **Step 6: Проверить и закоммитить**

Run: `npm test && npm run typecheck && npm run build`.

Для проверки — временная страница `course/modules/00b-network/_viz.md` с четырьмя сценами. Playwright 375 и 1280, обе темы: шаги листаются, подписи с терминами, ⛶ работает. Временную страницу удалить.

```bash
git add course/viz
git commit -m "feat(viz): плееры модуля 0b — инкапсуляция, DNS, цепочка сертификатов, UDP и TCP

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Песочница TCP

**Files:**
- Create: `course/viz/simulations/tcp.ts`, `course/viz/simulations/tcp.test.ts`, `course/viz/renderers/TcpView.vue`, `course/viz/scenes/TcpSandbox.vue`

**Interfaces:**
- Consumes: `Simulation`, `Step`, `CaptionSegment`.
- Produces:
  - `tcpSimulation: Simulation<TcpState, TcpAction>`;
  - `simulateTcp(state): TcpFrame[]`;
  - `DEFAULT_TCP`;
  - сцена `<TcpSandbox />`.

- [ ] **Step 1: Написать падающие тесты `tcp.test.ts`**

```ts
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds } from '../core/caption'
import { DEFAULT_TCP, simulateTcp, tcpSimulation, type TcpState } from './tcp'

const state = (over: Partial<TcpState> = {}): TcpState => ({ ...tcpSimulation.initial(), ...over })
const events = (s: TcpState) => simulateTcp(s).flatMap((f) => f.events.map((e) => ({ tick: f.tick, e })))
const lastTick = (s: TcpState) => simulateTcp(s).at(-1)!.tick

describe('simulateTcp', () => {
  it('A: без потерь 8 сегментов окном 4 доходят за 4 тика без повторов', () => {
    const s = state()
    expect(lastTick(s)).toBe(4)
    expect(events(s).some((x) => x.e === 'fast-retransmit' || x.e === 'timeout')).toBe(false)
    expect(simulateTcp(s).at(-1)!.events).toContain('done')
  })

  it('B: потеря сегмента 2 при окне 4 — быстрый повтор на тике 4, без таймаута', () => {
    const s = state({ dropSegments: [2] })
    expect(events(s)).toContainEqual({ tick: 4, e: 'fast-retransmit' })
    expect(events(s).some((x) => x.e === 'timeout')).toBe(false)
    expect(lastTick(s)).toBe(8)
  })

  it('C: окно 2 и потеря сегмента 1 — дубликатов не хватает, повтор по таймеру', () => {
    const s = state({ config: { ...DEFAULT_TCP, window: 2 }, dropSegments: [1] })
    const ev = events(s)
    expect(ev).toContainEqual({ tick: 8, e: 'timeout' })
    expect(ev.some((x) => x.e === 'fast-retransmit')).toBe(false)
    expect(simulateTcp(s).at(-1)!.events).toContain('done')
  })

  it('D: потеря ACK 2 покрывается следующим кумулятивным ACK — повторов нет', () => {
    const s = state({ dropAcks: [2] })
    expect(events(s).some((x) => x.e === 'fast-retransmit' || x.e === 'timeout')).toBe(false)
    expect(lastTick(s)).toBe(4)
  })

  it('всегда заканчивается не позже ограничителя', () => {
    const s = state({ config: { ...DEFAULT_TCP, window: 1 }, dropSegments: [0, 1, 2, 3, 4, 5, 6, 7] })
    expect(simulateTcp(s).at(-1)!.events).toContain('done')
  })
})

describe('tcpSimulation.apply', () => {
  it('повторное «потерять» снимает потерю, окно ограничено 1..8, reset возвращает начальное', () => {
    let s = tcpSimulation.initial()
    s = tcpSimulation.apply(s, { type: 'drop-segment', seq: 3 })
    expect(s.dropSegments).toEqual([3])
    s = tcpSimulation.apply(s, { type: 'drop-segment', seq: 3 })
    expect(s.dropSegments).toEqual([])
    expect(tcpSimulation.apply(s, { type: 'window', size: 20 }).config.window).toBe(8)
    expect(tcpSimulation.apply(s, { type: 'window', size: 0 }).config.window).toBe(1)
    expect(tcpSimulation.apply({ ...s, dropAcks: [1] }, { type: 'reset' })).toEqual(tcpSimulation.initial())
  })
})

describe('шаги', () => {
  it('у каждого шага есть подпись, все термины — из глоссария', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    const steps = tcpSimulation.steps(state({ dropSegments: [2] }))
    expect(steps.length).toBeGreaterThan(3)
    for (const st of steps) for (const id of captionTermIds(st.caption)) expect(known.has(id)).toBe(true)
  })
})
```

Run: `npm test -- viz/simulations/tcp.test.ts`
Expected: FAIL.

- [ ] **Step 2: Реализовать `course/viz/simulations/tcp.ts`**

```ts
import type { CaptionSegment } from '../core/caption'
import type { Simulation, Step } from '../core/types'

// Учебная модель TCP: задержка в одну сторону — 1 тик, кумулятивные ACK, быстрый повтор после трёх
// дубликатов, повтор по таймеру RTO. Нет SACK, slow start и точной формулы RTO — это упрощения,
// о которых урок говорит прямо.
export interface TcpConfig {
  segments: number
  window: number
  rto: number
}

export interface TcpState {
  config: TcpConfig
  dropSegments: readonly number[]
  dropAcks: readonly number[]
}

export type TcpAction =
  | { type: 'drop-segment'; seq: number }
  | { type: 'drop-ack'; ack: number }
  | { type: 'window'; size: number }
  | { type: 'reset' }

export type TcpEvent = 'send' | 'deliver' | 'ack' | 'dup-ack' | 'fast-retransmit' | 'timeout' | 'done'

export interface Packet {
  kind: 'segment' | 'ack'
  n: number
  lost: boolean
  retransmit: boolean
}

export interface TcpFrame {
  tick: number
  base: number
  nextSeq: number
  window: number
  expected: number
  received: readonly number[]
  dupAcks: number
  sent: readonly Packet[]
  arrived: readonly Packet[]
  events: readonly TcpEvent[]
}

export const DEFAULT_TCP: TcpConfig = { segments: 8, window: 4, rto: 6 }
const MAX_TICKS = 80

function toggle(list: readonly number[], n: number): number[] {
  return list.includes(n) ? list.filter((x) => x !== n) : [...list, n].sort((a, b) => a - b)
}

export function simulateTcp(state: TcpState): TcpFrame[] {
  const { segments, window, rto } = state.config
  const segLossUsed = new Set<number>()
  const ackLossUsed = new Set<number>()
  let base = 0
  let nextSeq = 0
  let expected = 0
  let dupAcks = 0
  let timerStart = 0
  const received = new Set<number>()
  let inTransit: Packet[] = []
  const frames: TcpFrame[] = []

  for (let tick = 0; tick <= MAX_TICKS; tick++) {
    const events: TcpEvent[] = []
    const arrived = inTransit.filter((p) => !p.lost)
    const sent: Packet[] = []
    const send = (p: Omit<Packet, 'lost'>, lost: boolean): void => {
      sent.push({ ...p, lost })
    }

    // 1. Получатель: принимает сегменты и отвечает кумулятивным ACK на каждый.
    for (const p of arrived.filter((x) => x.kind === 'segment').sort((a, b) => a.n - b.n)) {
      received.add(p.n)
      while (received.has(expected)) expected++
      events.push('deliver')
      const loseAck = state.dropAcks.includes(expected) && !ackLossUsed.has(expected)
      if (loseAck) ackLossUsed.add(expected)
      send({ kind: 'ack', n: expected, retransmit: false }, loseAck)
    }

    // 2. Отправитель: обрабатывает ACK.
    for (const p of arrived.filter((x) => x.kind === 'ack').sort((a, b) => a.n - b.n)) {
      if (p.n > base) {
        base = p.n
        dupAcks = 0
        timerStart = tick
        events.push('ack')
      } else if (p.n === base && base < nextSeq) {
        dupAcks++
        events.push('dup-ack')
        if (dupAcks === 3) {
          send({ kind: 'segment', n: base, retransmit: true }, false)
          events.push('fast-retransmit')
          dupAcks = 0
          timerStart = tick
        }
      }
    }

    // 3. Таймер: самый старый неподтверждённый сегмент не подтверждён за RTO тиков.
    if (base < nextSeq && tick - timerStart >= rto && !events.includes('fast-retransmit')) {
      send({ kind: 'segment', n: base, retransmit: true }, false)
      events.push('timeout')
      timerStart = tick
    }

    // 4. Новые сегменты, пока позволяет окно.
    while (nextSeq < base + window && nextSeq < segments) {
      if (base === nextSeq) timerStart = tick
      const lose = state.dropSegments.includes(nextSeq) && !segLossUsed.has(nextSeq)
      if (lose) segLossUsed.add(nextSeq)
      send({ kind: 'segment', n: nextSeq, retransmit: false }, lose)
      events.push('send')
      nextSeq++
    }

    const done = base >= segments
    if (done) events.push('done')
    frames.push({ tick, base, nextSeq, window, expected, received: [...received].sort((a, b) => a - b), dupAcks, sent, arrived, events })
    if (done) break
    inTransit = sent
  }
  return frames
}

function caption(f: TcpFrame): CaptionSegment[] {
  const parts: CaptionSegment[] = [{ text: `Тик ${f.tick}. ` }]
  const segs = f.sent.filter((p) => p.kind === 'segment' && !p.retransmit).map((p) => p.n)
  if (f.events.includes('fast-retransmit')) {
    parts.push({ text: `Три дубликата ACK ${f.base} подряд — отправитель не ждёт таймера: ` }, { text: 'быстрый повтор', termId: 'retransmission' }, { text: ` сегмента ${f.base}. ` })
  }
  if (f.events.includes('timeout')) {
    parts.push({ text: `Подтверждения нет уже RTO тиков — повтор сегмента ${f.base} `, }, { text: 'по таймеру', termId: 'retransmission' }, { text: '. Дубликатов не хватило: окно слишком маленькое. ' })
  }
  if (segs.length > 0) parts.push({ text: `Отправлены сегменты ${segs.join(', ')} — столько, сколько позволяет окно (${f.window}). ` })
  if (f.events.includes('deliver')) parts.push({ text: `Получатель ждёт сегмент ${f.expected} и отвечает ACK ${f.expected}. ` })
  if (f.events.includes('done')) parts.push({ text: 'Все сегменты подтверждены.' })
  return parts
}

export const tcpSimulation: Simulation<TcpState, TcpAction, TcpFrame> = {
  initial: () => ({ config: { ...DEFAULT_TCP }, dropSegments: [], dropAcks: [] }),
  apply: (s, a) => {
    switch (a.type) {
      case 'drop-segment':
        return { ...s, dropSegments: toggle(s.dropSegments, a.seq) }
      case 'drop-ack':
        return { ...s, dropAcks: toggle(s.dropAcks, a.ack) }
      case 'window':
        return { ...s, config: { ...s.config, window: Math.min(8, Math.max(1, Math.trunc(a.size))) } }
      case 'reset':
        return tcpSimulation.initial()
    }
  },
  steps: (s) => simulateTcp(s).map((f): Step<TcpFrame> => ({ state: f, caption: caption(f) })),
}
```

Run: `npm test -- viz/simulations/tcp.test.ts`
Expected: PASS (все 7).

Если тест B или C даёт другой тик, проверить модель вручную по трассе из спецификации задачи. Числа в тестах выведены из правил выше: тики приёма и отправки, задержка в одну сторону — 1 тик. Не подгонять тесты под код без разбора, а описать расхождение в отчёте.

- [ ] **Step 3: `TcpView.vue` и `TcpSandbox.vue` — контракт**

**Рендерер:**
- две вертикальные оси «Отправитель» и «Получатель»;
- кадр показывает отправленные на этом тике пакеты как стрелки: сегменты голубым `--viz-request`, ACK зелёным `--viz-response`, потерянные — красным `--viz-error` с крестиком на середине, повторы — пунктиром;
- сверху полоса номеров сегментов 0…7: подтверждённые / в полёте / в окне / вне окна, рамка окна;
- у получателя — буфер принятых не по порядку;
- узкая раскладка для 375.

**Сцена:**
- `const state = ref(tcpSimulation.initial())`, `const steps = computed(() => tcpSimulation.steps(state.value))`, `<Player :steps="steps">`;
- над плеером панель действий:
  - кнопки «Потерять сегмент k» (k = 0…segments−1, переключатель с `aria-pressed`);
  - «Потерять ACK k» (k = 1…segments);
  - ползунок окна 1–8;
  - «Сброс».

  Изменение пересоздаёт шаги, плеер сбрасывается на первый шаг.
- всё в `VizFrame label="Песочница TCP"`.

- [ ] **Step 4: Проверить и закоммитить**

Run: `npm test && npm run typecheck && npm run build`.

Временная страница, Playwright:
- потерять сегмент 2 → на шаге тика 4 подпись про быстрый повтор;
- окно 2 и потерять 1 → подпись про повтор по таймеру;
- 375 px без прокрутки, обе темы, ⛶.

Удалить временную страницу.

```bash
git add course/viz
git commit -m "feat(viz): песочница TCP — окно, потери, быстрый повтор и повтор по таймеру

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Песочницы пула соединений и «полос»

**Files:**
- Create: `course/viz/simulations/pool.ts` (+ test), `course/viz/simulations/lanes.ts` (+ test), `course/viz/renderers/PoolView.vue`, `course/viz/renderers/LanesView.vue`, `course/viz/scenes/PoolSandbox.vue`, `course/viz/scenes/LanesSandbox.vue`

**Interfaces:**
- Consumes: `formatMs`, `VizFrame`.
- Produces:
  - `poolTimeline(input: PoolInput): PoolResult`, `sanitizePool(input): PoolInput`;
  - `lanesTimeline(input: LanesInput): LanesResult`;
  - сцены `<PoolSandbox preset?="default|go-default" />`, `<LanesSandbox protocols?="h1,h2" | "h1,h2,quic" />`.

  Песочницы без плеера: таймлайн целиком, пересчёт по ползункам — как калькулятор в 0.5.

- [ ] **Step 1: Тесты и код `pool.ts`**

```ts
// pool.test.ts
import { describe, expect, it } from 'vitest'
import { poolTimeline, sanitizePool, type PoolInput } from './pool'

const base: PoolInput = { rttMs: 40, requests: 5, concurrency: 1, maxIdlePerHost: 2, tls: true, keepAlive: true }

describe('poolTimeline', () => {
  it.each<[string, Partial<PoolInput>, number, number]>([
    ['keep-alive, последовательно: одно соединение, (2 + n) RTT', {}, 1, 280],
    ['без keep-alive: каждый запрос — TCP + TLS + запрос', { keepAlive: false }, 5, 600],
    ['без TLS: (1 + n) RTT', { tls: false }, 1, 240],
    ['10 воркеров, пул 10, 100 запросов: 10 соединений, 12 RTT', { requests: 100, concurrency: 10, maxIdlePerHost: 10 }, 10, 480],
  ])('%s', (_name, over, conns, total) => {
    const r = poolTimeline({ ...base, ...over })
    expect(r.connections).toBe(conns)
    expect(r.totalMs).toBe(total)
  })

  it('пул 2 при 10 воркерах открывает больше соединений и работает дольше, чем пул 10', () => {
    const small = poolTimeline({ ...base, requests: 100, concurrency: 10, maxIdlePerHost: 2 })
    const big = poolTimeline({ ...base, requests: 100, concurrency: 10, maxIdlePerHost: 10 })
    expect(small.connections).toBeGreaterThan(big.connections)
    expect(small.totalMs).toBeGreaterThan(big.totalMs)
  })

  it('спаны не пересекаются внутри одного соединения', () => {
    const r = poolTimeline({ ...base, requests: 30, concurrency: 4, maxIdlePerHost: 2 })
    const byConn = Map.groupBy(r.spans, (s) => s.conn)
    for (const spans of byConn.values()) {
      const sorted = [...spans].sort((a, b) => a.startMs - b.startMs)
      for (let i = 1; i < sorted.length; i++) expect(sorted[i]!.startMs).toBeGreaterThanOrEqual(sorted[i - 1]!.endMs)
    }
  })
})

describe('sanitizePool', () => {
  it('приводит мусор к допустимому', () => {
    expect(sanitizePool({ rttMs: -1, requests: 0, concurrency: 0, maxIdlePerHost: -3, tls: true, keepAlive: true })).toEqual({
      rttMs: 1, requests: 1, concurrency: 1, maxIdlePerHost: 0, tls: true, keepAlive: true,
    })
    expect(sanitizePool({ ...base, concurrency: 50, requests: 5 }).concurrency).toBe(5)
  })
})
```

`Map.groupBy` есть в Node 26. Если `tsconfig` (`lib`) его не знает, добавить `"lib": ["ES2024", "DOM"]` или сгруппировать вручную.

```ts
// pool.ts
// Модель пула http.Transport: соединение после ответа возвращается в простаивающие, только если
// простаивающих меньше maxIdlePerHost (MaxIdleConnsPerHost, по умолчанию в Go — 2); иначе закрывается.
// Общее число соединений не ограничено (MaxConnsPerHost = 0). Время — в целых RTT.
export interface PoolInput {
  rttMs: number
  requests: number
  concurrency: number
  maxIdlePerHost: number
  tls: boolean
  keepAlive: boolean
}

export type SpanKind = 'tcp' | 'tls' | 'request'

export interface Span {
  conn: number
  request: number
  kind: SpanKind
  startMs: number
  endMs: number
}

export interface PoolResult {
  spans: Span[]
  totalMs: number
  connections: number
}

const clampInt = (v: number, lo: number, hi: number): number => (Number.isFinite(v) ? Math.min(hi, Math.max(lo, Math.trunc(v))) : lo)

export function sanitizePool(i: PoolInput): PoolInput {
  const requests = clampInt(i.requests, 1, 1000)
  return {
    rttMs: clampInt(i.rttMs, 1, 1000),
    requests,
    concurrency: Math.min(clampInt(i.concurrency, 1, 100), requests),
    maxIdlePerHost: clampInt(i.maxIdlePerHost, 0, 100),
    tls: i.tls,
    keepAlive: i.keepAlive,
  }
}

interface Worker {
  freeAt: number
  conn: number | null
}

export function poolTimeline(raw: PoolInput): PoolResult {
  const i = sanitizePool(raw)
  const R = i.rttMs
  const spans: Span[] = []
  const idle: number[] = []
  const workers: Worker[] = Array.from({ length: i.concurrency }, () => ({ freeAt: 0, conn: null }))
  let connections = 0
  let next = 0

  while (next < i.requests) {
    const t = Math.min(...workers.map((w) => w.freeAt))
    const ready = workers.filter((w) => w.freeAt === t)
    // Сначала все, кто закончил в момент t, возвращают соединения — потом новые запросы их забирают.
    for (const w of ready) {
      if (w.conn === null) continue
      if (i.keepAlive && idle.length < i.maxIdlePerHost) idle.push(w.conn)
      w.conn = null
    }
    for (const w of ready) {
      if (next >= i.requests) break
      const req = next++
      let start = t
      let conn = i.keepAlive ? idle.shift() : undefined
      if (conn === undefined) {
        conn = connections++
        spans.push({ conn, request: req, kind: 'tcp', startMs: start, endMs: start + R })
        start += R
        if (i.tls) {
          spans.push({ conn, request: req, kind: 'tls', startMs: start, endMs: start + R })
          start += R
        }
      }
      spans.push({ conn, request: req, kind: 'request', startMs: start, endMs: start + R })
      w.freeAt = start + R
      w.conn = conn
    }
    // Воркеры, оставшиеся без запросов, больше не участвуют: их время — «бесконечность».
    for (const w of ready) if (w.conn === null) w.freeAt = Number.POSITIVE_INFINITY
  }
  const totalMs = spans.reduce((m, s) => Math.max(m, s.endMs), 0)
  return { spans, totalMs, connections }
}
```

**Проверка тестов по коду:**
- `keepAlive` при `concurrency 1`: 1-й запрос — 3R, остальные 4 — по R, итого 7R = 280 ✓;
- без keep-alive: 5 × 3R = 600 ✓;
- без TLS: 2R + 4R = 240 ✓;
- 10/10/100: каждый воркер — 3R + 9R = 12R = 480, соединений 10 ✓.

Run: `npm test -- viz/simulations/pool.test.ts`
Expected: PASS.

- [ ] **Step 2: Тесты и код `lanes.ts`**

```ts
// lanes.test.ts
import { describe, expect, it } from 'vitest'
import { lanesTimeline, type LanesInput } from './lanes'

const base: LanesInput = { proto: 'h2', streams: 3, packetsPerStream: 4, lostPacket: null, h1Connections: 1, rttSlots: 4 }
const done = (over: Partial<LanesInput>) => lanesTimeline({ ...base, ...over }).streamDone

describe('lanesTimeline', () => {
  it('без потерь HTTP/2 и QUIC заканчивают одинаково', () => {
    expect(done({ proto: 'quic' })).toEqual(done({ proto: 'h2' }))
  })

  // Пакет 9 — последний пакет потока 0 (пакеты чередуются: id = номер_пакета × потоки + поток).
  it('потеря пакета 9: в HTTP/2 задерживаются все потоки', () => {
    const clean = done({ proto: 'h2' })
    const lossy = done({ proto: 'h2', lostPacket: 9 })
    lossy.forEach((d, s) => expect(d).toBeGreaterThan(clean[s]!))
  })

  it('потеря пакета 9: в QUIC задерживается только поток 0', () => {
    const clean = done({ proto: 'quic' })
    const lossy = done({ proto: 'quic', lostPacket: 9 })
    expect(lossy[0]).toBeGreaterThan(clean[0]!)
    expect(lossy.slice(1)).toEqual(clean.slice(1))
  })

  it('HTTP/1.1 с одним соединением: потоки заканчиваются строго по очереди', () => {
    const d = done({ proto: 'h1', h1Connections: 1 })
    for (let s = 1; s < d.length; s++) expect(d[s]).toBeGreaterThan(d[s - 1]!)
  })

  it('у потерянного пакета время прихода — после повтора', () => {
    const r = lanesTimeline({ ...base, lostPacket: 0 })
    const p = r.packets.find((x) => x.id === 0)!
    expect(p.lost).toBe(true)
    expect(p.arrivesAt).toBe(p.sentAt + base.rttSlots + base.rttSlots / 2)
  })
})
```

```ts
// lanes.ts
// Учебная модель: отправитель шлёт по одному пакету за слот в каждом соединении; задержка в одну
// сторону — rttSlots / 2; потерянный пакет обнаруживается через RTT и приходит ещё через полпути.
// HTTP/2: одно TCP-соединение, потоки чередуются, TCP отдаёт данные приложению строго по порядку.
// QUIC: то же чередование, но порядок соблюдается только внутри потока.
// HTTP/1.1: потоки распределены по h1Connections соединениям и идут в каждом по очереди.
// Управления перегрузкой нет — это упрощение, названное в уроке.
export type Proto = 'h1' | 'h2' | 'quic'

export interface LanesInput {
  proto: Proto
  streams: number
  packetsPerStream: number
  lostPacket: number | null
  h1Connections: number
  rttSlots: number
}

export interface LanePacket {
  id: number
  stream: number
  conn: number
  sentAt: number
  arrivesAt: number
  deliveredAt: number
  lost: boolean
}

export interface LanesResult {
  packets: LanePacket[]
  streamDone: number[]
}

export function lanesTimeline(i: LanesInput): LanesResult {
  const half = i.rttSlots / 2
  const order: { stream: number; conn: number; sentAt: number }[] = []
  if (i.proto === 'h1') {
    const k = Math.max(1, i.h1Connections)
    const slot = Array.from({ length: k }, () => 0)
    for (let s = 0; s < i.streams; s++) {
      const conn = s % k
      for (let p = 0; p < i.packetsPerStream; p++) order.push({ stream: s, conn, sentAt: slot[conn]!++ })
    }
    order.sort((a, b) => a.sentAt - b.sentAt || a.conn - b.conn)
  } else {
    let t = 0
    for (let p = 0; p < i.packetsPerStream; p++) for (let s = 0; s < i.streams; s++) order.push({ stream: s, conn: 0, sentAt: t++ })
  }

  const packets: LanePacket[] = order.map((o, id) => {
    const lost = id === i.lostPacket
    const arrivesAt = lost ? o.sentAt + i.rttSlots + half : o.sentAt + half
    return { id, ...o, arrivesAt, deliveredAt: arrivesAt, lost }
  })

  // Порядок доставки приложению: по соединению (TCP) или по потоку (QUIC).
  const lastByKey = new Map<string, number>()
  for (const p of [...packets].sort((a, b) => a.sentAt - b.sentAt || a.conn - b.conn)) {
    const key = i.proto === 'quic' ? `s${p.stream}` : `c${p.conn}`
    p.deliveredAt = Math.max(p.arrivesAt, lastByKey.get(key) ?? 0)
    lastByKey.set(key, p.deliveredAt)
  }

  const streamDone = Array.from({ length: i.streams }, (_, s) =>
    Math.max(...packets.filter((p) => p.stream === s).map((p) => p.deliveredAt)),
  )
  return { packets, streamDone }
}
```

Run: `npm test -- viz/simulations/lanes.test.ts`
Expected: PASS.

- [ ] **Step 3: Рендереры и сцены — контракт**

**`PoolView`:**
- диаграмма Ганта: строка на соединение, отрезки `tcp` (`--viz-network`), `tls` (`--viz-service`), `request` (`--viz-request`), подпись номера запроса при ширине отрезка ≥ 24 px;
- шкала времени в мс;
- итог: общее время, число соединений, число рукопожатий;
- больше 40 соединений — показать первые 40 и строку «и ещё N».

**`PoolSandbox`:**
- ползунки RTT (5–200 мс), запросы (1–200), параллельность (1–50), `MaxIdleConnsPerHost` (0–50);
- переключатели TLS и keep-alive;
- `preset="go-default"` — 100 запросов, 10 воркеров, пул 2, TLS, keep-alive;
- рядом с ползунком пула подсказка «в Go по умолчанию — 2».

**`LanesView`:** для каждого протокола дорожка-таймлайн. Пакеты — квадраты по слотам, цвет по потоку (3 различимых цвета палитры с подписями «поток 1–3»). Потерянный пакет — крест и стрелка повтора. Задержанные доставкой приложению — штриховка. Справа — время готовности каждого потока.

**`LanesSandbox`:**
- выбор пакета для потери кликом по квадрату (повторный клик — снять);
- ползунок RTT в слотах (2–8);
- для h1 — число соединений (1–6);
- одна и та же потеря применяется ко всем показанным протоколам, чтобы их было видно рядом;
- проп `protocols` задаёт, какие дорожки показаны (в 0b.7 — h1 и h2, в 0b.8 — все три).

Все сцены — в `VizFrame`, узкая раскладка, клавиатура (кнопки и ползунки нативные).

- [ ] **Step 4: Проверить и закоммитить**

Run: `npm test && npm run typecheck && npm run build`.

Временная страница, Playwright 375 и 1280, обе темы:
- пресет `go-default` показывает больше 10 соединений;
- потеря пакета в конце потока 0 задерживает все потоки HTTP/2 и только поток 0 в QUIC. Ранняя потеря при малом RTT может «рассосаться» к концу — урок это объясняет.

Удалить временную страницу.

```bash
git add course/viz
git commit -m "feat(viz): песочницы пула соединений и «полос» HTTP/1.1, HTTP/2 и QUIC

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Общие правила для уроков (задачи 8–11)

1. **Шаблон.** Шаблон урока основной спецификации; у каждого `##` — `<Part kind>` (`problem`, `theory`, `viz`, `mistakes`, `tradeoff`, `code`, `selfcheck`, `sources`, `practice`). Лабораторная — раздел `## <Part kind="practice" /> Лабораторная: …`.
2. **Числа.** 3–8 `<N>` на урок.
3. **«Под капотом».** Минимум один `<Deep>` (150–500 слов); темы указаны в задаче.
4. **Самодостаточность.** Перечитать и закрыть места, где нужна книга; список — в отчёт.
5. **Код.** Код лабораторной — только `<<< @/../labs/0b/cmd/<prog>/main.go#<регион>{go}`, без копий. Иллюстративные Go-фрагменты — с `<!-- go-check -->`, при упрощении — `<Callout type="simplified">`.
6. **Вывод команд.** Реальный, из отчётов задач 3–4 или собственного прогона, с пометкой «у вас числа будут другими, порядок — такой».
7. **Источники.** Формат «Автор, «Название», глава/раздел — что найти». Каждую ссылку на RFC и раздел проверить.
8. **Связи.** Назад — на 0.3, 0.4, 0.6; вперёд — на 2.5, 3, 6.7, 9, 12.
9. **Факты.** Перечисленные в задаче факты исполнитель **проверяет**. План ошибался раньше — неверный факт исправить и записать в отчёт.
10. **Проверка.** `npm test && npm run check:go && npm run check:labs && npm run build`, Playwright 375 и 1280, обе темы: метки, `<Deep>` с клавиатуры, сцены работают, ⛶, нет горизонтальной прокрутки.

### Task 8: Уроки 0b.1 «Слои» и 0b.2 «TCP изнутри»

**Files:**
- Create: `course/modules/00b-network/01-layers.md`, `course/modules/00b-network/02-tcp.md`
- Modify: `course/modules/00b-network/index.md` (раскомментировать ссылку на 0b.1), `labs/0b/README.md`

**Interfaces:**
- Consumes: сцены `Encapsulation`, `TcpSandbox`; программы `echo`; окружение `labs/0b`.

- [ ] **Step 1: Урок 0b.1 `01-layers.md`**

Frontmatter: `title: 0b.1. Слои: от IP-пакета до HTTP`.

**Разделы:**
- **Проблема.** «Что на самом деле уходит в сеть, когда Go делает `http.Get`» — 200 байт текста превращаются в 292 байта на проводе, а Wireshark показывает десятки пакетов.
- **Теория:**
  - IP-адрес и маршрут (`ip route`);
  - порт; сокет как файловый дескриптор;
  - четвёрка соединения (src ip, src port, dst ip, dst port);
  - эфемерные порты (диапазон 32768–60999 из урока 0.6);
  - инкапсуляция;
  - MTU 1500 и MSS 1460 (1448 с метками времени);
  - что видит каждое устройство (коммутатор, маршрутизатор, балансировщик L4/L7 — ссылка на 2.5).
- **Визуализация:** `<Encapsulation />` + задания.
- **Лабораторная** (`practice`): `ss -tan`, `ip addr`, `ip route` в контейнерах; `tcpdump -i eth0 -w /captures/first.pcap` во время `curl http://172.28.0.10:8080/`; открыть в Wireshark и найти рукопожатие и заголовки. Реальный вывод `ss` и `tcpdump -nn -r`.
- **Под капотом:**
  - «Что происходит при `net.Dial`»: системные вызовы `socket`, `connect`, неблокирующий режим, netpoller (ссылка на 1.5), файловый дескриптор и лимит `ulimit -n`;
  - «MTU, фрагментация и Path MTU Discovery»: флаг DF, ICMP «Fragmentation needed», чёрные дыры MTU в туннелях (VPN, overlay-сети Kubernetes).
- **Ошибки, компромисс** (большой MTU / jumbo frames внутри ЦОД против совместимости), **самопроверка, источники.**

**Термины:** `socket`, `port`, `encapsulation`, `mtu`, `pcap`, `ip-address`.

**Числа:**
- 292 байта;
- ≈ 68% полезных;
- 1500 MTU;
- 1460 MSS;
- 28 232 порта.

- [ ] **Step 2: Урок 0b.2 `02-tcp.md`**

Frontmatter: `title: 0b.2. TCP изнутри`.

**Разделы:**
- **Проблема.** «Почему один потерянный пакет превращает 40-мс запрос в секундный» — повтор `SYN` по таймеру 1 с (RFC 6298), об этом же мини-интервью модуля.
- **Теория:**
  - рукопожатие (повтор 0.3 кратко со ссылкой);
  - ISN (RFC 6528), ACK, окно получателя;
  - кумулятивное подтверждение;
  - повтор по таймеру: начальный RTO 1 с, удвоение, минимальный RTO в Linux 200 мс;
  - быстрый повтор после 3 дублирующих ACK (RFC 5681);
  - управление перегрузкой: начальное окно 10 сегментов (RFC 6928), slow start, cwnd;
  - закрытие FIN и TIME_WAIT (60 с в Linux, со ссылкой на 0.3 и 0.6);
  - Nagle и отложенные ACK: задержка до 40 мс в Linux; в Go `TCP_NODELAY` включён по умолчанию.
- **Визуализация:** `<TcpSandbox />` + задания:
  - потерять сегмент 2 при окне 4 — быстрый повтор;
  - окно 2 и потерять 1 — повтор по таймеру; почему;
  - потерять ACK — ничего не случилось; почему.
- **Лабораторная:**
  - `echo` под `netem.sh 40ms 0%`, затем `5%`;
  - `ss -ti dst 172.28.0.10` — rtt, cwnd, retrans;
  - `tcpdump` повторов;
  - код сервера и клиента регионами `#server` и `#client`.
- **Под капотом:**
  - «Почему окно перегрузки растёт именно так»: AIMD, потеря как сигнал, CUBIC в Linux по умолчанию, BBR — кратко, с оговоркой упрощения;
  - «TIME_WAIT: зачем две минуты превратились в одну и почему не надо `tcp_tw_recycle`»: 2·MSL, его удаление из Linux 4.12, `tcp_tw_reuse`.
- **Ошибки:**
  - лечить TIME_WAIT ядром вместо пула;
  - не ставить таймауты;
  - путать окно получателя и cwnd.
- **Компромисс:** Nagle против задержки.
- **Самопроверка, источники.**

**Термины:** `tcp`, `retransmission`, `cwnd`, `slow-start`, `time-wait`, `nagle`, `netem`.

**Числа:**
- 1 с — RTO;
- 3 дубликата;
- 10 сегментов — начальное окно;
- 60 с — TIME_WAIT;
- 40 мс — отложенный ACK.

- [ ] **Step 3: Проверить (правило 10), README, commit**

В `labs/0b/README.md` добавить раздел «Урок 0b.1» и «Урок 0b.2» со ссылками на уроки.

```bash
git add course labs
git commit -m "feat(course): уроки 0b.1 «Слои» и 0b.2 «TCP изнутри»

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Уроки 0b.3 «UDP» и 0b.4 «DNS глубже»

**Files:**
- Create: `course/modules/00b-network/03-udp.md`, `course/modules/00b-network/04-dns.md`
- Modify: `labs/0b/README.md`

**Interfaces:**
- Consumes: сцены `UdpVsTcp`, `DnsChain`; программа `udpping`; CoreDNS из Compose.

- [ ] **Step 1: Урок 0b.3 `03-udp.md`**

Frontmatter: `title: 0b.3. UDP: когда надёжность не нужна`.

**Разделы:**
- **Проблема.** Видеозвонок, где старый кадр бесполезен, или DNS-запрос, где рукопожатие дороже самого ответа.
- **Теория:**
  - датаграмма, заголовок 8 байт;
  - нет соединения, порядка и повторов;
  - максимальная полезная нагрузка 65 507 байт на IPv4, но фрагментация опасна;
  - DNS: классические 512 байт, EDNS0 и рекомендованные 1232 байта (DNS Flag Day 2020);
  - где UDP: DNS, медиа (RTP), игры, QUIC (ссылка на 0b.8);
  - что приложение берёт на себя.
- **Визуализация:** `<UdpVsTcp />`.
- **Лабораторная:** `udpping` под потерями 0%, 2%, 5%. Почему итоговые потери ≈ 1 − (1 − p)² при потерях на обоих концах.
- **Под капотом:** «Почему UDP легко использовать для DDoS-усиления»: подмена адреса отправителя, коэффициенты усиления DNS/NTP, rate limit на серверах — с источником.
- **Ошибки, компромисс, самопроверка, источники.**

**Термины:** `udp`, `datagram`.

**Числа:**
- 8 байт;
- 65 507;
- 1232;
- ≈ 9,75% при 5% на каждом конце — пересчитать.

- [ ] **Step 2: Урок 0b.4 `04-dns.md`**

Frontmatter: `title: 0b.4. DNS глубже`.

**Разделы:**
- **Проблема.** «Переехали на новый IP — у части пользователей сервис лежит сутки», плюс 5-секундные задержки в Kubernetes из-за `ndots`.
- **Теория:**
  - записи A, AAAA, CNAME (нельзя на вершине зоны), NS, MX, TXT, SRV;
  - TTL;
  - отрицательный кэш (минимум SOA, RFC 2308);
  - рекурсивный и авторитетный сервер (связь с Deep 0.3);
  - резолвер Go: чистый Go или cgo, кэша нет, `/etc/resolv.conf`, `/etc/hosts`;
  - `search` и `ndots` (в Kubernetes по умолчанию `ndots:5`) и лишние запросы;
  - таймаут резолвера по умолчанию 5 с (`options timeout`);
  - DNS и балансировка: несколько A-записей, round-robin, клиентский кэш.
- **Визуализация:** `<DnsChain />`.
- **Лабораторная:**
  - `dig api.messenger.example`, `dig +trace` (на публичный домен), `dig` с отображением TTL;
  - изменить запись в `messenger.example.db` (CoreDNS `reload` или перезапуск) и увидеть, как кэш `forward`/`cache` живёт до TTL;
  - Go: `net.DefaultResolver.LookupHost` в простом фрагменте с `go-check`.
- **Под капотом:**
  - «Почему `ndots:5` удваивает и утраивает запросы и как это лечить»: FQDN с точкой, `dnsConfig`;
  - «Как DNS ломает отказоустойчивость»: кэш у клиента пережил переключение, длинный TTL против нагрузки на DNS.
- **Ошибки:**
  - длинный TTL перед переездом;
  - кэш DNS в долгоживущих Go-соединениях (соединение не переподключается);
  - CNAME на вершине.
- **Компромисс:** короткий TTL — гибкость, длинный — меньше запросов и задержки.
- **Самопроверка, источники.**

**Термины:** `dns`, `dns-record`, `ttl`, `resolver`.

**Числа:**
- TTL 30 с (лабораторная);
- `ndots:5`;
- 5 с — таймаут;
- 100 мс против 40 мс (холодный и тёплый кэш из плеера).

- [ ] **Step 3: Проверить, README, commit**

```bash
git add course labs
git commit -m "feat(course): уроки 0b.3 «UDP» и 0b.4 «DNS глубже»

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: Уроки 0b.5 «TLS на практике» и 0b.6 «HTTP/1.1, keep-alive и пулы»

**Files:**
- Create: `course/modules/00b-network/05-tls.md`, `course/modules/00b-network/06-http1-pools.md`
- Modify: `labs/0b/README.md`

**Interfaces:**
- Consumes: сцены `CertChain`, `PoolSandbox`; программы `tlsdemo`, `h2server`, `httpbench`.

- [ ] **Step 1: Урок 0b.5 `05-tls.md`**

Frontmatter: `title: 0b.5. TLS на практике`.

**Разделы:**
- **Проблема.** `x509: certificate signed by unknown authority` в проде после смены сертификата, и соблазн `InsecureSkipVerify`.
- **Теория:**
  - сертификат X.509: субъект, издатель, SAN, срок, открытый ключ;
  - цепочка: лист → промежуточный → корень; корни в хранилище ОС, в Go — системный пул или `RootCAs`;
  - проверка имени по SAN (CN не используется);
  - SNI: клиент сообщает имя;
  - ALPN: выбор h2 или http/1.1;
  - возобновление сессии в TLS 1.3 (PSK, `ClientSessionCache` в Go) и его цена;
  - `tls.Config`: `MinVersion` (у клиента Go по умолчанию TLS 1.2), `ServerName`;
  - взгляд на mTLS — подробно в 6.7.
- **Визуализация:** `<CertChain />`.
- **Лабораторная:**
  - `tlsdemo get` с верным и неверным именем;
  - `tlsdemo inspect` — `DidResume` на втором подключении;
  - `openssl s_client -connect 172.28.0.10:8443 -servername api.messenger.example -CAfile /captures/ca.pem`;
  - код `#client-config`.
- **Под капотом:** «Как Go проверяет цепочку»: `x509.Verify`, построение цепочки, промежуточные из ответа сервера, `crypto/x509` и системный пул в Linux.
- **Ошибки:**
  - `InsecureSkipVerify`;
  - сервер не отдаёт промежуточный сертификат;
  - истёкший сертификат без мониторинга — ссылка на 12.
- **Компромисс:** TLS-терминация на балансировщике (связь с 0.3) против сквозного шифрования.
- **Самопроверка, источники.**

**Термины:** `tls`, `certificate`, `ca`, `sni`, `alpn`, `session-resumption`.

**Числа:**
- 1 RTT — полное рукопожатие TLS 1.3;
- 2 RTT — TLS 1.2;
- `DidResume`.

- [ ] **Step 2: Урок 0b.6 `06-http1-pools.md`**

Frontmatter: `title: 0b.6. HTTP/1.1, keep-alive и пулы соединений`.

**Разделы:**
- **Проблема.** «Сервис ходит в соседний 100 раз в секунду, и половина времени уходит на рукопожатия» — числа из `httpbench`.
- **Теория:**
  - формат HTTP/1.1 (стартовая строка, заголовки, тело, `Content-Length` или chunked);
  - keep-alive по умолчанию в 1.1, `Connection: close`;
  - браузеры держат до 6 соединений на хост;
  - пул `http.Transport`: `MaxIdleConns` = 100, `MaxIdleConnsPerHost` = 2 по умолчанию, `MaxConnsPerHost` = 0, `IdleConnTimeout` = 90 с;
  - почему тело нужно дочитать и закрыть;
  - таймауты клиента: `http.Client.Timeout` по умолчанию 0 (нет таймаута!), `DialContext`, `TLSHandshakeTimeout` = 10 с, `ResponseHeaderTimeout`;
  - таймауты сервера: по умолчанию их нет; `ReadHeaderTimeout` против Slowloris;
  - рассинхрон idle-таймаутов клиента и балансировщика (клиентский должен быть меньше).
- **Визуализация:** `<PoolSandbox preset="go-default" />` и задания: пул 2 против 10, keep-alive вкл/выкл.
- **Лабораторная:** четыре прогона `httpbench` из задачи 4 и разбор вывода; код `#transport` и `#trace`; сервер `#server`.
- **Под капотом:**
  - «Что делает `http.Transport`, когда пул пуст»: dial в фоне, гонка за соединением (Go 1.x), `MaxConnsPerHost`;
  - «Почему сервер закрыл соединение, а клиент об этом не знал»: idle timeout на балансировщике, ошибка `EOF` на переиспользованном соединении и повтор идемпотентных запросов в Go.
- **Ошибки:**
  - `http.Client{}` без таймаута;
  - создание `Transport` на каждый запрос;
  - недочитанное тело;
  - `MaxIdleConnsPerHost` = 2 при высокой нагрузке на один хост.
- **Компромисс:** большой пул против нагрузки на сервер и балансировщик.
- **Самопроверка, источники** (документация `net/http`, Cloudflare «The complete guide to Go net/http timeouts» — проверить заголовок и год).

**Термины:** `keep-alive`, `connection-pool`, `head-of-line-blocking` (в HTTP/1.1 — на уровне запросов).

**Числа:**
- 2 — `MaxIdleConnsPerHost`;
- 90 с;
- 0 — таймаут по умолчанию;
- 3 RTT против 1 RTT;
- 6 соединений браузера.

- [ ] **Step 3: Проверить, README, commit**

```bash
git add course labs
git commit -m "feat(course): уроки 0b.5 «TLS на практике» и 0b.6 «HTTP/1.1 и пулы»

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: Уроки 0b.7 «HTTP/2» и 0b.8 «HTTP/3 и QUIC» с мини-интервью

**Files:**
- Create: `course/modules/00b-network/07-http2.md`, `course/modules/00b-network/08-http3-quic.md`
- Modify: `labs/0b/README.md`

**Interfaces:**
- Consumes: сцена `LanesSandbox`; программы `h2server`, `httpbench`.

- [ ] **Step 1: Урок 0b.7 `07-http2.md`**

Frontmatter: `title: 0b.7. HTTP/2: много запросов в одном соединении`.

**Разделы:**
- **Проблема.** Шесть соединений браузера и очередь запросов в HTTP/1.1.
- **Теория:**
  - фреймы (HEADERS, DATA, SETTINGS, WINDOW_UPDATE, RST_STREAM, GOAWAY);
  - потоки и мультиплексирование;
  - управление потоком на уровне потока и соединения;
  - HPACK (RFC 7541) и почему заголовки сжимаются в разы;
  - приоритеты — кратко, RFC 9113 их упростил;
  - согласование через ALPN;
  - head-of-line blocking ушёл с уровня запросов, но остался на уровне TCP;
  - HTTP/2 в Go: включается автоматически с TLS у сервера и `http.Client`; h2c — нет по умолчанию;
  - gRPC работает поверх HTTP/2 — ссылка на 6.
- **Визуализация:** `<LanesSandbox protocols="h1,h2" />` и задания.
- **Лабораторная:**
  - `curl --http1.1` и `--http2 -w '%{http_version}'`;
  - `httpbench -http2=true` и `false` при 0% и 2% потерь;
  - разбор, почему при потерях выигрыш HTTP/2 уменьшается.
- **Под капотом:** «GOAWAY и плавное отключение»: как сервер говорит клиенту «больше не шли новые потоки», связь с graceful shutdown и выкаткой — ссылка на 13b.
- **Ошибки:**
  - одно HTTP/2-соединение на весь трафик к балансировщику, из-за чего нагрузка липнет к одному бэкенду (ссылка на 2.5 и gRPC);
  - бесконечные потоки без таймаута.
- **Компромисс:** одно соединение — меньше рукопожатий, но общая судьба при потере.
- **Самопроверка, источники.**

**Термины:** `multiplexing`, `hpack`, `head-of-line-blocking`, `alpn`.

- [ ] **Step 2: Урок 0b.8 `08-http3-quic.md` и мини-интервью**

Frontmatter: `title: 0b.8. HTTP/3 и QUIC`.

**Разделы:**
- **Проблема.** Мобильная сеть с потерями и сменой Wi-Fi на LTE.
- **Теория:**
  - почему не «улучшить TCP» (окостенение middlebox'ов, TCP в ядре);
  - QUIC поверх UDP (RFC 9000), TLS 1.3 внутри (RFC 9001);
  - независимые потоки;
  - рукопожатие 1 RTT и 0-RTT с риском повтора (связь с Deep 0.3);
  - ID соединения и миграция;
  - QPACK (RFC 9204);
  - HTTP/3 (RFC 9114), обнаружение через `Alt-Svc`;
  - где HTTP/3 уже работает;
  - почему в Go-лабораторной его нет: в стандартной библиотеке нет QUIC-сервера (проверить на Go 1.26 и написать точно).
- **Визуализация:** `<LanesSandbox protocols="h1,h2,quic" />`.
- **Практика:** `curl --http3 -I https://cloudflare.com` (или другой публичный сайт), если сборка curl поддерживает (`curl -V` покажет `HTTP3`). Если нет — показать реальный вывод из другой среды с пометкой.
- **Под капотом:** «Почему QUIC шифрует даже заголовки транспорта»: защита от окостенения, что видит middlebox, цена для отладки (Wireshark и `SSLKEYLOGFILE`).
- **Ошибки, компромисс** (UDP режут в корпоративных сетях → откат на HTTP/2), **самопроверка, источники.**
- **`## <Part kind="practice" /> Мини-интервью модуля 0b`.** Задание: «Пользователи жалуются, что API иногда отвечает ровно через 3 секунды или ровно через 1 секунду. Как найти причину на сетевом уровне?» Эталонный разбор в `<SelfCheck label="Задание." button="Показать эталонный разбор">`:
  - 1 с и 3 с = потеря SYN один или два раза (RTO 1 с, затем 2 с);
  - 5 с — таймаут DNS;
  - как подтвердить: `ss -ti`, `tcpdump`, метрики connect и DNS из `httptrace`;
  - что чинить: keep-alive и пулы, DNS-кэш или `ndots`, ретраи с таймаутами;
  - связь с 5 (устойчивость) и 12 (наблюдаемость).

**Термины:** `quic`, `zero-rtt`.

- [ ] **Step 3: Проверить, README, commit**

```bash
git add course labs
git commit -m "feat(course): уроки 0b.7 «HTTP/2» и 0b.8 «HTTP/3 и QUIC», мини-интервью модуля

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: Связи с модулем 0, итоговая проверка, README, бэклог

**Files:**
- Modify: `course/modules/00-foundation/03-request-path.md`, `course/modules/00-foundation/04-latency.md`, `course/modules/00-foundation/06-messenger-requirements.md` (если ссылается на 0b), `README.md`, `docs/superpowers/backlog-after-module-0.md`

- [ ] **Step 1: Ссылки из модуля 0**

`grep -n "0b" course/modules/00-foundation/*.md`. Каждое «подробно — модуль 0b» заменить ссылкой на конкретный урок и раздел, например `[урок 0b.2](../00b-network/02-tcp#…)` с реальным якорем из собранного HTML. Каждую ссылку проверить в браузере.

- [ ] **Step 2: Полный прогон**

Run: `cd course && npm test && npm run typecheck && npm run check:go && npm run check:labs && npm run build`
Expected: всё зелёное, сборка без мёртвых ссылок.

- [ ] **Step 3: Лабораторные с нуля**

`docker compose -f labs/0b/compose.yaml down -v`, затем пройти README `labs/0b` сверху вниз по всем урокам. Каждая команда работает и даёт вывод того же порядка, что в уроках. Расхождения исправить (в уроке или в README).

- [ ] **Step 4: Playwright**

Все уроки 0b, `/`, `/glossary`, ширина 375 и 1280, тёмная и светлая тема:
- нет горизонтальной прокрутки;
- строка «Модуль 0b · Урок N»;
- метки и цвет заголовков;
- каждая сцена реагирует;
- ⛶ с удержанием фокуса.

Скриншоты смотреть, а не только сохранять.

- [ ] **Step 5: README и бэклог**

- **README:** раздел «Лабораторные» — `labs/`, Compose, `netem.sh`, `npm run check:labs`, регионы `#region` для `<<<`.
- **Бэклог:** закрытое отметить, новое отложенное добавить.

- [ ] **Step 6: Commit**

```bash
git add course labs README.md docs
git commit -m "docs: связи модуля 0 с 0b, README лабораторных, бэклог

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
