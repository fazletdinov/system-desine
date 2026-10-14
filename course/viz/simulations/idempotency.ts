import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

export type IdemMode = 'no-key' | 'key'
export type Body = 'A' | 'B'
export type Outcome = 'created' | 'replayed' | 'in-progress' | 'key-reused' | 'server-error'

export interface Attempt {
  n: number
  body: Body
  status: number // что ответил сервер
  delivered: boolean // дошёл ли ответ до клиента
  outcome: Outcome
}

// Запись ключа на сервере: тело — вместо отпечатка (SHA-256 метода, пути, типа тела и тела).
export interface KeyRecord {
  body: Body
  done: boolean
  status: number
}

export interface IdemState {
  mode: IdemMode
  chats: number
  record: KeyRecord | null
  attempts: readonly Attempt[]
  loseNext: boolean
  failNext: boolean
  last: IdemAction['type'] | 'start'
}

export type IdemAction =
  | { type: 'send'; body: Body }
  | { type: 'parallel' }
  | { type: 'lose' }
  | { type: 'fail' }
  | { type: 'mode'; mode: IdemMode }
  | { type: 'reset' }

const start = (mode: IdemMode): IdemState => ({ mode, chats: 0, record: null, attempts: [], loseNext: false, failNext: false, last: 'start' })

// process — сервер обрабатывает один запрос до конца. Возвращает новое состояние и ответ.
function process(s: IdemState, body: Body): { s: IdemState; status: number; outcome: Outcome } {
  if (s.mode === 'key' && s.record !== null) {
    if (s.record.body !== body) return { s, status: 422, outcome: 'key-reused' }
    if (!s.record.done) return { s, status: 409, outcome: 'in-progress' }
    return { s, status: s.record.status, outcome: 'replayed' }
  }
  if (s.failNext) return { s: { ...s, failNext: false, record: null }, status: 500, outcome: 'server-error' }
  const record = s.mode === 'key' ? { body, done: true, status: 201 } : null
  return { s: { ...s, chats: s.chats + 1, record }, status: 201, outcome: 'created' }
}

function attempt(s: IdemState, body: Body, status: number, outcome: Outcome): IdemState {
  const a: Attempt = { n: s.attempts.length + 1, body, status, delivered: !s.loseNext, outcome }
  return { ...s, attempts: [...s.attempts, a], loseNext: false }
}

export const idempotencySimulation: Simulation<IdemState, IdemAction> = {
  initial: () => start('no-key'),
  apply: (s, a) => {
    switch (a.type) {
      case 'reset':
        return start(s.mode)
      case 'mode':
        return start(a.mode)
      case 'lose':
        return { ...s, loseNext: true, last: 'lose' }
      case 'fail':
        return { ...s, failNext: true, last: 'fail' }
      case 'send': {
        const r = process(s, a.body)
        return { ...attempt(r.s, a.body, r.status, r.outcome), last: 'send' }
      }
      case 'parallel': {
        // Два одинаковых запроса пришли одновременно. С ключом первый успел занять запись
        // «в работе» — второй видит её незавершённой. Без ключа сервер их не различает.
        if (s.mode === 'key' && s.record === null && !s.failNext) {
          const busy = { ...s, record: { body: 'A' as Body, done: false, status: 0 } }
          const second = attempt(busy, 'A', 409, 'in-progress')
          const first = process({ ...second, record: null }, 'A')
          return { ...attempt(first.s, 'A', first.status, first.outcome), last: 'parallel' }
        }
        const one = process(s, 'A')
        const s1 = attempt(one.s, 'A', one.status, one.outcome)
        const two = process(s1, 'A')
        return { ...attempt(two.s, 'A', two.status, two.outcome), last: 'parallel' }
      }
    }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<IdemState>],
}

function captionFor(s: IdemState): Caption {
  const mode: Caption =
    s.mode === 'key'
      ? [{ text: 'Клиент шлёт ' }, term('ключ идемпотентности', 'idempotency-key'), { text: ': сервер запоминает ответ и на повтор отдаёт его же.' }]
      : [{ text: 'Без ключа сервер не отличает повтор от нового запроса.' }]
  if (s.attempts.length === 0)
    return [...mode, { text: ' Отправьте запрос, потеряйте ответ и повторите — так ведёт себя любой клиент с ' }, term('доставкой хотя бы раз', 'at-least-once'), { text: '.' }]
  const lastA = s.attempts.at(-1) as Attempt
  const delivered = lastA.delivered ? `клиент получил ${lastA.status}` : `сервер ответил ${lastA.status}, но ответ потерялся`
  return [...mode, { text: ` Попытка ${lastA.n}: ${delivered}. Создано чатов: ${s.chats}.` }]
}
