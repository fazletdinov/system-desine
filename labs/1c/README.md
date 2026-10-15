# Лабораторная 1c — контейнер с нуля на Go

Урок: [1.15. Контейнер изнутри](../../course/modules/01-go-service/15-container-internals.md).

`minicontainer` — учебный контейнер на пару сотен строк Go. Он делает то же, что `runc` под Docker,
только без всего остального: запускает процесс в новых namespaces (UTS, PID, mount, net, IPC), меняет ему
корень через `pivot_root` на файловую систему busybox и ставит лимиты cgroup v2 (`memory.max`, `pids.max`,
`cpu.max`). Рядом лежит `hog` — статический Go-бинарь, который занимает память, чтобы было на чём проверить
`memory.max`.

```
1c/
├── Dockerfile          # окружение: golang + rootfs busybox в /rootfs + minicontainer и hog
├── compose.yaml        # privileged, cgroup: private
├── entrypoint.sh       # включает контроллеры cpu, memory, pids для дочерних групп
├── scenario.sh         # пять экспериментов с проверками (его запускает check:labs)
└── cmd/
    ├── minicontainer/  # run (родитель) и child (уже внутри namespaces)
    └── hog/            # «пожиратель» памяти для эксперимента 5
```

## Требования

- Docker с Compose и cgroup v2 (`docker info` показывает `Cgroup Version: 2`) — это любой современный Linux
  и Docker Desktop;
- на Docker Desktop всё работает внутри его виртуальной машины: namespaces и cgroup — её ядра, а не ядра
  macOS или Windows. Хосту это ничего не стоит и ничего на нём не меняет.

## Запуск

```bash
cd labs/1c
docker compose build
docker compose run --rm lab        # оболочка внутри окружения лабы, hostname — lab
```

Внутри — от root:

```bash
minicontainer run [--mem 64M] [--pids 32] [--cpu 50%] [--hostname box] /rootfs <команда> [аргументы…]
```

После выхода команды `minicontainer` печатает в stderr итог `minicontainer: итог: oom_kill=<n> pids_max=<n>`
(счётчики из `memory.events` и `pids.events` группы), удаляет группу и выходит с кодом команды; если команду
убил сигнал N — с кодом 128+N, как shell.

Ctrl+C (SIGINT) или SIGTERM родителю `minicontainer` перехватывает сам: убивает контейнер SIGKILL, печатает итог и
удаляет группу, код выхода — 137. Мягкой остановки нет: процесс-ребёнок — init своего PID namespace, и SIGINT или
SIGTERM, для которых у него нет обработчика, ядро ему просто не доставит (как PID 1 в контейнере без `init: true`,
урок 1.17).

Проверить, что `entrypoint.sh` включил контроллеры:

```
$ cat /sys/fs/cgroup/cgroup.subtree_control
cpu memory pids
```

## Пять экспериментов

Вывод ниже — настоящий, получен в окружении лабы на Docker Desktop для Linux (ядро `7.0.12-linuxkit`).

### 1. PID namespace: процесс один и он — PID 1

```
$ minicontainer run /rootfs ps
PID   USER     TIME  COMMAND
    1 root      0:00 ps
minicontainer: итог: oom_kill=0 pids_max=0
```

`ps` читает свежий `/proc`, который `child` смонтировал внутри нового PID namespace, — там виден только сам
`ps`. В окружении лабы в то же время процессов больше (`ps -e --no-headers | wc -l` дал `4`), и среди них
процесс контейнера имеет обычный номер: `sleep`, запущенный через `minicontainer`, в `ps -e` окружения был
PID 25, а внутри — PID 1.

### 2. UTS namespace: своё имя хоста

```
$ minicontainer run --hostname box /rootfs hostname
box
minicontainer: итог: oom_kill=0 pids_max=0
$ hostname
lab
```

`sethostname` внутри поменял имя только в новом UTS namespace.

### 3. Net namespace: сети нет

```
$ minicontainer run /rootfs ip -o link
1: lo: <LOOPBACK> mtu 65536 qdisc noop qlen 1000\    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
2: tunl0@NONE: <NOARP> mtu 1480 qdisc noop qlen 1000\    link/ipip 0.0.0.0 brd 0.0.0.0
3: gre0@NONE: <NOARP> mtu 1476 qdisc noop qlen 1000\    link/gre 0.0.0.0 brd 0.0.0.0
4: gretap0@NONE: <BROADCAST,MULTICAST> mtu 1462 qdisc noop qlen 1000\    link/ether 00:00:00:00:00:00 brd ff:ff:ff:ff:ff:ff
5: erspan0@NONE: <BROADCAST,MULTICAST> mtu 1450 qdisc noop qlen 1000\    link/ether 00:00:00:00:00:00 brd ff:ff:ff:ff:ff:ff
6: ip_vti0@NONE: <NOARP> mtu 1480 qdisc noop qlen 1000\    link/ipip 0.0.0.0 brd 0.0.0.0
7: ip6_vti0@NONE: <NOARP> mtu 1332 qdisc noop qlen 1000\    link/tunnel6 00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00 brd 00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00
8: sit0@NONE: <NOARP> mtu 1480 qdisc noop qlen 1000\    link/sit 0.0.0.0 brd 0.0.0.0
9: ip6tnl0@NONE: <NOARP> mtu 1452 qdisc noop qlen 1000\    link/tunnel6 00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00 brd 00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00
10: ip6gre0@NONE: <NOARP> mtu 1448 qdisc noop qlen 1000\    link/[823] 00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00 brd 00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00
minicontainer: итог: oom_kill=0 pids_max=0
$ minicontainer run /rootfs ip -o addr
minicontainer: итог: oom_kill=0 pids_max=0
```

Главное — чего здесь нет: сетевой карты окружения `eth0`. `lo` есть, но выключен (в флагах нет `UP`), адресов нет
вообще. Остальные строки — «запасные» туннельные устройства (fallback tunnels): если модули `ipip`, `gre`, `sit`,
`vti`, `ip6_tunnel` встроены в ядро или загружены, ядро создаёт `tunl0`, `gre0`, `sit0`… в *каждом* новом
net namespace. Ядро VM Docker Desktop собрано именно так. Все они выключены и никуда не подключены. На ядре, где этих модулей
нет, должна остаться одна строка `lo` (так устроен код ядра; на раннере GitHub Actions ещё не проверено — первый
прогон CI покажет). Проверка сценария от этого не зависит. В окружении лабы те же туннели тоже есть — плюс `eth0`,
через который оно ходит в сеть.

`busybox ip` не печатает поле `state`; понять, что интерфейс выключен, можно по флагам: у поднятого в угловых
скобках стоит `UP`. Удобнее спросить прямо — `ip -o link show up` внутри печатает пустоту.

### 4. pids.max: fork упирается в лимит

```
$ minicontainer run --pids 16 /rootfs sh -c 'for i in $(seq 1 64); do sleep 1 & done; wait'
sh: can't fork: Resource temporarily unavailable
minicontainer: итог: oom_kill=0 pids_max=1
```

Shell пытается родить 64 `sleep`, а лимит группы — 16 задач, включая сам shell. Как только задач в группе стало
16, очередной `fork` получил `EAGAIN`, счётчик `max` в `pids.events` вырос, а busybox `sh` на ошибке `fork`
прекратил работу с кодом 2. Окружение лабы живо: лимит — у группы контейнера, а не у
всей машины. Так `pids.max` останавливает fork-бомбу.

### 5. memory.max: OOM-kill

```
$ minicontainer run --mem 32M /rootfs hog -mib 128
занято 16 МиБ
minicontainer: итог: oom_kill=1 pids_max=0
$ echo $?
137
```

`hog` трогает каждую страницу каждого мебибайта — только тронутая страница становится настоящей памятью. Где-то
между 16 и 32 МиБ (сам рантайм Go тоже занимает память группы) группа упёрлась в `memory.max`; ядро не смогло
ничего вытеснить (`memory.swap.max` = 0, своп запрещён) и убило `hog` по SIGKILL: 128 + 9 = 137. Это тот же код,
который показывает `docker ps` у контейнера, убитого за превышение `--memory`.

После всех запусков ни одной группы `minicontainer-*` не осталось — `run` удаляет свою и после OOM-kill, и после Ctrl+C:

```
$ ls -d /sys/fs/cgroup/minicontainer-*
ls: cannot access '/sys/fs/cgroup/minicontainer-*': No such file or directory
```

## Сценарий целиком

```bash
cd labs/1c
docker compose run --rm -T lab /lab/scenario.sh
```

```
ок 1: в контейнере один процесс, PID 1
ок 2: hostname внутри — box, снаружи — lab
ок 3: eth0 окружения нет, lo и остальные интерфейсы выключены, адресов нет
ок 4: лимит pids.max отказал в fork
ок 5: OOM-kill по memory.max, код выхода 137
ок: SIGINT родителю — контейнер остановлен, группа удалена
Все пять экспериментов прошли.
```

Строка без номера — проверка очистки, а не эксперимент: сценарий шлёт SIGINT запущенному `minicontainer run
/rootfs sleep 60`, как Ctrl+C, и проверяет код 137 и что группы не осталось. Тот же сценарий запускает
`npm run check:labs` (из `course/`).

## Почему `privileged` и `cgroup: private`

`minicontainer` делает работу Docker: монтирует файловые системы, вызывает `pivot_root`, создаёт namespaces и
группы cgroup. Обычному контейнеру всё это запрещено (нет `CAP_SYS_ADMIN`, seccomp режет `mount`, `/sys/fs/cgroup`
смонтирован только для чтения). `privileged: true` снимает запреты — и это почти права хоста, поэтому так делают
только в лабе и никогда в проде.

`cgroup: private` даёт окружению свой cgroup namespace: его группа выглядит как корень `/sys/fs/cgroup`, и
`minicontainer` создаёт дочерние группы внутри неё, не трогая группы Docker и других контейнеров. Правило cgroup v2
«нет процессов во внутренних узлах» не даёт включить контроллеры, пока в корне группы живут процессы, поэтому
`entrypoint.sh` сначала переносит их в лист `init`.

## Как это сломать

1. **Убрать `MS_PRIVATE`.** Docker уже сделал все монтирования окружения лабы приватными, поэтому сначала
   воспроизведите обычный хост с systemd, где корень «shared»: `mount --make-rshared /`. С настоящим
   `minicontainer` запуск по-прежнему работает. Теперь закомментируйте в `child` вызов
   `mount --make-rprivate /`, пересоберите (`docker compose build`), снова сделайте `mount --make-rshared /` и
   запустите: `pivot_root` откажет с `invalid argument` — `pivot_root(2)` не разрешает менять корень, если
   новый корень или его родитель участвуют в shared-распространении, иначе смена корня «утекла» бы в чужие mount
   namespaces. Проверить то же без пересборки можно утилитой: `unshare -m --propagation unchanged sh -c 'mount
   --bind /rootfs /rootfs; mkdir -p /rootfs/.o; pivot_root /rootfs /rootfs/.o'`.
2. **Заменить `pivot_root` на `chroot`.** Вместо `PivotRoot` и размонтирования `.oldroot` вызовите
   `syscall.Chroot(rootfs)`. `chroot` меняет только то, что процесс считает корнем, а файловая система окружения
   остаётся смонтированной в его mount namespace. Процесс с root-правами выходит из такого корня классическим
   приёмом: `chroot` в подкаталог, *не* переходя в него, затем `chdir("..")` много раз и `chroot(".")`. Утилита
   `chroot` из busybox сама делает `chdir("/")` и так не умеет — приём пишется десятком строк на Go (`hog` — пример,
   как собрать статический бинарь для rootfs). После `pivot_root` с размонтированием старого корня выходить некуда.
3. **Убрать `memory.swap.max`.** У VM Docker Desktop есть своп (`free -m` в окружении показал 1 ГиБ). Удалите
   строку из `createCgroup`, пересоберите и запустите `minicontainer run --mem 32M /rootfs hog -mib 128`: `hog`
   не будет убит сразу, а начнёт вытесняться в своп — работать медленнее, но дальше. Лимит памяти без лимита
   свопа — мягкий.
