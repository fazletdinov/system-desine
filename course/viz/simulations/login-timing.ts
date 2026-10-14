import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type LoginImpl = 'naive' | 'safe'
export type LoginAttempt = 'unknown-user' | 'wrong-password'

export interface LoginResponse {
  status: number
  code: string
  text: string
}

export interface LoginTimingState {
  impl: LoginImpl
  attempt: LoginAttempt
  response: LoginResponse
  elapsedMs: number
  steps: string[]
}

export const LOGIN_HIGHLIGHT_IDS: readonly string[] = ['text', 'time', 'response', 'uniform']

// Порядок величины на машине автора; точные цифры меряются в уроке.
const FAST_MS = 1
const HASH_MS = 50
const SAFE_WRONG_MS = 52

const NOT_FOUND: LoginResponse = { status: 404, code: 'user_not_found', text: 'Пользователь не найден' }
const BAD_PASSWORD: LoginResponse = { status: 401, code: 'invalid_password', text: 'Неверный пароль' }
const UNIFORM: LoginResponse = { status: 401, code: 'invalid_credentials', text: 'Неверный логин или пароль' }

const MARK = 'на машине автора; меряется в уроке'

interface Frame {
  state: LoginTimingState
  caption: Caption
  highlight: string
}

const FRAMES: readonly Frame[] = [
  {
    state: { impl: 'naive', attempt: 'unknown-user', response: NOT_FOUND, elapsedMs: FAST_MS, steps: ['SELECT … WHERE login = "mallory" → строк нет', 'return 404 «Пользователь не найден»'] },
    highlight: 'response',
    caption: `Наивный вход, попытка 1: логина «mallory» нет в базе. Код честно сообщает 404 и сразу выходит — хеш считать не из чего. Ответ приходит за ~${FAST_MS} мс (${MARK}).`,
  },
  {
    state: { impl: 'naive', attempt: 'wrong-password', response: BAD_PASSWORD, elapsedMs: HASH_MS, steps: ['SELECT … WHERE login = "alice" → строка найдена', 'argon2id.Verify(пароль, хеш)  // намеренно медленно', 'return 401 «Неверный пароль»'] },
    highlight: 'response',
    caption: [
      { text: 'Попытка 2: логин alice есть, пароль не тот. Проверка ' },
      term('Argon2id', 'argon2id'),
      { text: ` занимает ~${HASH_MS} мс — медленность и есть его защита (${MARK}). Ответ 401 с другим кодом и другим текстом.` },
    ],
  },
  {
    state: { impl: 'naive', attempt: 'unknown-user', response: NOT_FOUND, elapsedMs: FAST_MS, steps: ['Сравниваем ответы: «Пользователь не найден» ≠ «Неверный пароль»', 'Первая утечка: текст'] },
    highlight: 'text',
    caption: [
      { text: 'Две попытки видны рядом. Первая утечка — текст и статус: по ответу злоумышленник узнаёт, существует ли логин. Это ' },
      term('перечисление пользователей', 'user-enumeration'),
      { text: ': получив список аккаунтов, он будет подбирать пароли только к реальным.' },
    ],
  },
  {
    state: { impl: 'naive', attempt: 'wrong-password', response: BAD_PASSWORD, elapsedMs: HASH_MS, steps: [`1 мс против ${HASH_MS} мс — разница в ${HASH_MS} раз`, 'Вторая утечка: время'] },
    highlight: 'time',
    caption: [
      { text: 'Даже если унифицировать текст, остаётся вторая утечка — время. Один ответ быстрый, другой в десятки раз медленнее, и разницу видно по секундомеру. Это ' },
      term('атака по времени', 'timing-attack'),
      { text: ': секрет определяется не из ответа, а из того, сколько на него ушло.' },
    ],
  },
  {
    state: { impl: 'safe', attempt: 'unknown-user', response: UNIFORM, elapsedMs: HASH_MS, steps: ['SELECT … WHERE login = "mallory" → строк нет', 'берём заготовленный фиктивный хеш', 'argon2id.Verify(пароль, фиктивный хеш) → false', 'return 401 «Неверный логин или пароль»'] },
    highlight: 'uniform',
    caption: [
      { text: 'Правильная реализация, попытка 1: пользователя нет, но мы всё равно проверяем пароль — против фиктивного хеша, посчитанного при старте с теми же параметрами. Ответ 401 invalid_credentials приходит за те же ~' + HASH_MS + ' мс (' + MARK + '). Это и есть ' },
      term('хеширование пароля', 'password-hashing'),
      { text: ' без побочного канала.' },
    ],
  },
  {
    state: { impl: 'safe', attempt: 'wrong-password', response: UNIFORM, elapsedMs: SAFE_WRONG_MS, steps: ['SELECT … WHERE login = "alice" → строка найдена', 'argon2id.Verify(пароль, хеш) → false', 'return 401 «Неверный логин или пароль»'] },
    highlight: 'uniform',
    caption: `Попытка 2: пользователь найден, пароль неверный. Тот же статус, тот же код, тот же текст, время почти то же (~${SAFE_WRONG_MS} мс, ${MARK}). Снаружи два случая неразличимы.`,
  },
  {
    state: { impl: 'safe', attempt: 'unknown-user', response: UNIFORM, elapsedMs: HASH_MS, steps: ['статус: 401 в обоих случаях', 'код: invalid_credentials в обоих случаях', `время: ~${HASH_MS} мс против ~${SAFE_WRONG_MS} мс`], },
    highlight: 'uniform',
    caption: 'Сравните: статус, код, текст совпадают, время расходится на единицы процентов, что тонет в сетевом шуме. Отличить «нет такого логина» от «неверный пароль» по ответу больше нельзя.',
  },
  {
    state: { impl: 'safe', attempt: 'wrong-password', response: UNIFORM, elapsedMs: SAFE_WRONG_MS, steps: ['клиенту: invalid_credentials', 'в лог: reason=wrong_password, user=alice, request_id=…', 'в лог: reason=unknown_user, login=mallory, request_id=…'] },
    highlight: 'response',
    caption: 'Различие не пропало, а переехало туда, куда посторонним хода нет: в лог. Оператор видит настоящую причину под request_id, клиент — один и тот же ответ.',
  },
]

export function loginTimingScenario(): Step<LoginTimingState>[] {
  return FRAMES.map((fr) => ({ state: fr.state, caption: fr.caption, highlight: [fr.highlight] }))
}
