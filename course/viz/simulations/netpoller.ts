import { term } from '../core/caption'
import type { Step } from '../core/types'

export type NetpollerVariant = 'data' | 'deadline'
export type GId = 'G1' | 'G2'
export type GState = 'running' | 'runnable' | 'waiting' | 'done'

export interface NetpollerFrame {
  t: number // миллисекунды от начала сценария
  gs: readonly GId[]
  // Поток M: какую G выполняет, где находится сам и держит ли процессор P.
  // go — Go-код, syscall — в системном вызове, findRunnable — ищет работу,
  // netpoll — заблокирован в epoll_wait; перед этим M обязан отдать P (proc.go: «netpoll with p» — throw).
  m: { g: GId | null; where: 'go' | 'syscall' | 'findRunnable' | 'netpoll'; p: boolean }
  runq: readonly GId[] // локальная очередь P
  pollDesc: { rg: GId | null } // горутина, запаркованная на чтение этого сокета
  epoll: { ready: boolean } // сокет в списке готовых у epoll, но рантайм ещё не забрал событие
  socketBytes: number // байт в буфере сокета в ядре
  timer: { atMs: number; fired: boolean } | null // таймер дедлайна чтения в куче таймеров P
  syscall: string | null // последний системный вызов
  done: readonly GId[]
  result: { ok: true; n: number } | { ok: false; err: string } | null
}

// Каждая G должна быть ровно в одном месте: на M, в очереди, на pollDesc или завершена.
export function gStates(f: NetpollerFrame, g: GId): GState[] {
  const states: GState[] = []
  if (f.m.g === g) states.push('running')
  if (f.runq.includes(g)) states.push('runnable')
  if (f.pollDesc.rg === g) states.push('waiting')
  if (f.done.includes(g)) states.push('done')
  return states
}

// Размер ответа в сценарии: один небольшой сетевой пакет.
const PAYLOAD_BYTES = 512
// Дедлайн чтения в варианте «дедлайн истёк».
export const READ_DEADLINE_MS = 100

const NETPOLLER = term('сетевой поллер (netpoller)', 'netpoller')
const EPOLL = term('epoll', 'epoll')
const NONBLOCKING = term('неблокирующем режиме', 'nonblocking-io')
const EDGE = term('по фронту (edge-triggered)', 'edge-triggered')

const base: NetpollerFrame = {
  t: 0,
  gs: ['G1', 'G2'],
  m: { g: 'G1', where: 'go', p: true },
  runq: ['G2'],
  pollDesc: { rg: null },
  epoll: { ready: false },
  socketBytes: 0,
  timer: null,
  syscall: null,
  done: [],
  result: null,
}

function common(deadline: boolean): Step<NetpollerFrame>[] {
  const timer = deadline ? { atMs: READ_DEADLINE_MS, fired: false } : null
  return [
    {
      state: { ...base, timer },
      caption: [
        { text: `Горутина G1 на потоке M с процессором P вызывает ${deadline ? `conn.SetReadDeadline(+${READ_DEADLINE_MS} мс), а затем ` : ''}conn.Read(buf). G2 ждёт своей очереди в локальной очереди P. Сокет Go открыл в ` },
        NONBLOCKING,
        { text: ' и при создании зарегистрировал в ' },
        EPOLL,
        { text: deadline ? ' рантайма. Дедлайн — это таймер рантайма в куче таймеров P, а не поток, который спит 100 мс.' : ' рантайма с флагами EPOLLIN | EPOLLOUT | EPOLLRDHUP | EPOLLET.' },
      ],
    },
    {
      state: { ...base, t: 0.01, m: { g: 'G1', where: 'syscall', p: true }, timer, syscall: 'read(fd, buf, 4096) = -1 EAGAIN' },
      caption: 'conn.Read делает системный вызов read. Буфер сокета в ядре пуст, а сокет неблокирующий, поэтому ядро не усыпляет поток, а сразу отвечает EAGAIN: «данных пока нет, попробуй позже». Поток M свободен за микросекунды.',
    },
    {
      state: { ...base, t: 0.02, m: { g: 'G2', where: 'go', p: true }, runq: [], pollDesc: { rg: 'G1' }, timer, syscall: 'read(fd, buf, 4096) = -1 EAGAIN' },
      caption: [
        { text: 'Вместо того чтобы ждать, G1 вызывает pollDesc.waitRead и паркуется (gopark): её указатель записан в поле rg структуры pollDesc этого сокета, статус — ожидание ввода-вывода. ' },
        NETPOLLER,
        { text: ' будет знать, кого будить. Поток M не простаивает: планировщик сразу берёт из очереди G2. Тысячи ждущих соединений стоят тысячи запаркованных ' },
        term('горутин', 'goroutine'),
        { text: ', а не тысячи потоков ОС.' },
      ],
    },
  ]
}

function dataScenario(): Step<NetpollerFrame>[] {
  const parked = { ...base, runq: [], pollDesc: { rg: 'G1' as GId } }
  return [
    ...common(false),
    {
      state: { ...parked, t: 3, m: { g: 'G2', where: 'go', p: true }, epoll: { ready: true }, socketBytes: PAYLOAD_BYTES, syscall: null },
      caption: [
        { text: `На 3-й миллисекунде из сети приходит ${PAYLOAD_BYTES} байт. Ядро кладёт их в буфер сокета и ставит сокет в список готовых у epoll. Регистрация ` },
        EDGE,
        { text: ': ядро сообщит о готовности один раз, в момент её появления. Сигнала в процесс нет, G2 работает дальше: Go узнает о событии, только когда сам спросит epoll.' },
      ],
    },
    {
      state: { ...parked, t: 5, m: { g: null, where: 'findRunnable', p: true }, epoll: { ready: true }, socketBytes: PAYLOAD_BYTES, done: ['G2'] },
      caption: 'На 5-й миллисекунде G2 завершилась. M с процессором P входит в findRunnable — поиск следующей горутины: таймеры не созрели, локальная и глобальная очереди пусты. Прежде чем красть работу у других P, рантайм быстро проверяет сеть: это дешевле кражи.',
    },
    {
      state: { ...base, t: 5.01, m: { g: 'G1', where: 'go', p: true }, runq: [], socketBytes: PAYLOAD_BYTES, done: ['G2'], syscall: 'epoll_wait(epfd, events, 128, 0) = 1' },
      caption: [
        { text: 'findRunnable вызывает netpoll(0) — неблокирующий epoll_wait с нулевым таймаутом, ещё до кражи у других P. Ядро отдаёт одно событие: сокет готов к чтению. Рантайм достаёт из pollDesc запаркованную G1, переводит её в готовые и сразу запускает на этом M — до кражи дело не дошло. Если бы все M были заняты, сеть проверил бы ' },
        term('sysmon', 'sysmon'),
        { text: ' (если её не опрашивали дольше 10 мс); если все свободны — один M блокируется в epoll_wait.' },
      ],
    },
    {
      state: { ...base, t: 5.02, m: { g: 'G1', where: 'go', p: true }, runq: [], done: ['G2'], syscall: `read(fd, buf, 4096) = ${PAYLOAD_BYTES}`, result: { ok: true, n: PAYLOAD_BYTES } },
      caption: `G1 продолжает внутри conn.Read и повторяет read: на этот раз ядро отдаёт ${PAYLOAD_BYTES} байт. conn.Read возвращает n = ${PAYLOAD_BYTES}, err = nil. Для программиста это был обычный блокирующий вызов, а поток ОС за 5 мс успел выполнить другую горутину.`,
    },
  ]
}

function deadlineScenario(): Step<NetpollerFrame>[] {
  const parked = { ...base, runq: [], pollDesc: { rg: 'G1' as GId }, timer: { atMs: READ_DEADLINE_MS, fired: false } }
  return [
    ...common(true),
    {
      state: { ...parked, t: 60, m: { g: 'G2', where: 'go', p: true } },
      caption: 'Проходит 60 мс. Данных нет: буфер сокета пуст, epoll молчит. G1 по-прежнему запаркована на pollDesc и не занимает ни потока, ни процессора.',
    },
    {
      state: { ...parked, t: 80, m: { g: null, where: 'netpoll', p: false }, done: ['G2'], syscall: 'epoll_wait(epfd, events, 128, 20) …' },
      caption: [
        { text: 'На 80-й миллисекунде G2 завершилась. findRunnable прошёл весь путь: очереди пусты, быстрый netpoll(0) пуст, украсть у других P нечего. Тогда M отдаёт P в список свободных и только после этого блокируется в epoll_wait. Таймаут — 20 мс, до ближайшего таймера (дедлайна G1): так его разбудит либо пакет, либо время ' },
        term('дедлайна', 'deadline'),
        { text: '.' },
      ],
    },
    {
      state: { ...base, t: READ_DEADLINE_MS, m: { g: 'G1', where: 'go', p: true }, runq: [], timer: { atMs: READ_DEADLINE_MS, fired: true }, done: ['G2'], syscall: 'epoll_wait(epfd, events, 128, 20) = 0' },
      caption: [
        { text: `На ${READ_DEADLINE_MS}-й миллисекунде epoll_wait вернулся по таймауту без событий. M снова берёт свободный P и повторяет поиск работы, начиная с таймеров: таймер дедлайна сработал и вызвал netpolldeadlineimpl — он помечает дедлайн чтения истёкшим и будит G1 с pollDesc. ` },
        NETPOLLER,
        { text: ' тот же, только разбудило его время, а не данные. M запускает G1.' },
      ],
    },
    {
      state: { ...base, t: READ_DEADLINE_MS + 0.01, m: { g: 'G1', where: 'go', p: true }, runq: [], timer: { atMs: READ_DEADLINE_MS, fired: true }, done: ['G2'], syscall: null, result: { ok: false, err: 'os.ErrDeadlineExceeded' } },
      caption: 'G1 возвращается в conn.Read, видит истёкший дедлайн и выходит без повторного read. Ошибка — *net.OpError с текстом «i/o timeout», внутри которой os.ErrDeadlineExceeded: errors.Is(err, os.ErrDeadlineExceeded) истинно. Так устроены таймауты чтения в net/http: ни один поток ОС не спал все 100 мс.',
    },
  ]
}

export function netpollerScenario(variant: NetpollerVariant): readonly Step<NetpollerFrame>[] {
  return variant === 'data' ? dataScenario() : deadlineScenario()
}
