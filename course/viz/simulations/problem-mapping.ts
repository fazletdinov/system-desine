import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type ProblemSource = 'domain' | 'param' | 'router' | 'panic' | 'decode'

export const PROBLEM_SOURCES: readonly ProblemSource[] = ['domain', 'param', 'router', 'panic', 'decode']

export const SOURCE_LABELS: Readonly<Record<ProblemSource, string>> = {
  domain: 'домен / use case',
  param: 'обёртка кодогенерации',
  router: 'ServeMux',
  panic: 'Recover',
  decode: 'httpjson.Decode',
}

export interface ProblemMappingState {
  source: ProblemSource
  error: string
  problem: Record<string, unknown> | null
  highlightFields: string[]
  // Заголовки ответа, которые не входят в тело problem (например Allow).
  headers: string[]
}

const BASE = 'https://messenger.example/problems/'
const MIME = 'Content-Type: application/problem+json'

interface ProblemInit {
  code: string
  status: number
  title: string
  detail: string
  instance: string
  requestId: string
  extra?: Record<string, unknown>
}

function problem(p: ProblemInit): Record<string, unknown> {
  return { type: BASE + p.code, title: p.title, status: p.status, detail: p.detail, instance: p.instance, code: p.code, request_id: p.requestId, ...p.extra }
}

interface Frame {
  state: ProblemMappingState
  caption: Caption
}

const PANIC_TEXT = 'runtime error: index out of range [3] with length 3'
const COMMON = ['type', 'status', 'code']

const FRAMES: readonly Frame[] = [
  {
    state: { source: 'domain', error: 'domain.ValidationError{Field: "text", Reason: "empty"}', problem: null, highlightFields: [], headers: [] },
    caption: 'Источник 1, домен. Use case вернул доменную ошибку ValidationError: поле text пустое. В ней нет ни статуса, ни JSON — домен ничего не знает про HTTP. Решить, во что она превратится, должен слой на границе.',
  },
  {
    state: {
      source: 'domain',
      error: 'domain.ValidationError{Field: "text", Reason: "empty"}',
      problem: problem({ code: 'invalid_input', status: 400, title: 'Invalid input', detail: 'Field "text" must not be empty', instance: '/v1/chats/0199…a1/messages', requestId: 'a41f7c', extra: { invalid_params: [{ name: 'text', reason: 'must not be empty' }] } }),
      highlightFields: [...COMMON, 'invalid_params'],
      headers: [MIME],
    },
    caption: [
      { text: 'Единая функция отображения ошибок находит тип через errors.As и строит ' },
      term('problem details', 'problem-details'),
      { text: ': 400, код invalid_input, а имя поля уходит в invalid_params. Клиент подсветит именно поле text, не разбирая человеческий текст.' },
    ],
  },
  {
    state: { source: 'param', error: 'invalid format for parameter limit: strconv.Atoi: parsing "abc": invalid syntax', problem: null, highlightFields: [], headers: [] },
    caption: 'Источник 2, параметр запроса. GET …/messages?limit=abc: сгенерированная обёртка не смогла превратить «abc» в число и вызвала наш обработчик ошибок параметров. До нашего хендлера дело даже не дошло.',
  },
  {
    state: {
      source: 'param',
      error: 'invalid format for parameter limit: strconv.Atoi: parsing "abc": invalid syntax',
      problem: problem({ code: 'invalid_input', status: 400, title: 'Invalid input', detail: 'Query parameter "limit" is not a number', instance: '/v1/chats/0199…a1/messages', requestId: 'b3c19d', extra: { invalid_params: [{ name: 'limit', reason: 'must be an integer' }] } }),
      highlightFields: [...COMMON, 'invalid_params'],
      headers: [MIME],
    },
    caption: [
      { text: 'Тот же код invalid_input и та же форма, но другой источник: клиенту всё равно, кто споткнулся — домен или обёртка. Один ' },
      term('маппер', 'mapper'),
      { text: ' ошибок на все источники — один код и один формат на все ошибки ввода.' },
    ],
  },
  {
    state: { source: 'router', error: 'DELETE /v1/chats: method not allowed (allowed: GET, HEAD, POST)', problem: null, highlightFields: [], headers: [] },
    caption: 'Источник 3, маршрутизатор. Путь /v1/chats есть, метода DELETE у него нет. ServeMux сам решил отдать 405 и подготовил заголовок Allow.',
  },
  {
    state: {
      source: 'router',
      error: 'DELETE /v1/chats: method not allowed (allowed: GET, HEAD, POST)',
      problem: problem({ code: 'method_not_allowed', status: 405, title: 'Method not allowed', detail: 'DELETE is not supported on /v1/chats', instance: '/v1/chats', requestId: 'c52e80' }),
      highlightFields: COMMON,
      headers: ['Allow: GET, HEAD, POST', MIME],
    },
    caption: 'Мы оставляем Allow как есть и подменяем только тело: теперь оно в общем формате с кодом method_not_allowed. Заголовок Allow — часть протокола, его нельзя терять.',
  },
  {
    state: { source: 'panic', error: PANIC_TEXT, problem: null, highlightFields: [], headers: [] },
    caption: 'Источник 4, паника. Хендлер обратился к срезу за границей: runtime error. Это не ошибка пользователя, и рассказывать о ней снаружи нельзя: внутри текст ошибки может содержать пути, имена таблиц, фрагменты запросов.',
  },
  {
    state: {
      source: 'panic',
      error: PANIC_TEXT,
      problem: problem({ code: 'internal', status: 500, title: 'Internal error', detail: 'Something went wrong. Quote the request_id when contacting support.', instance: '/v1/chats/0199…a1/messages', requestId: 'd7a4e2' }),
      highlightFields: [...COMMON, 'detail', 'request_id'],
      headers: [MIME],
    },
    caption: [
      { text: 'Клиент получает 500 с безликим detail и ' },
      term('request_id', 'request-id'),
      { text: '. Подробности — стек и текст паники — пишутся только в лог под тем же id. Так поддержка по одному id находит причину, а посторонний ничего не узнаёт о внутренностях.' },
    ],
  },
  {
    state: { source: 'decode', error: 'http: request body too large (limit 1 MiB)', problem: null, highlightFields: [], headers: [] },
    caption: 'Источник 5, тело запроса. Клиент прислал больше 1 МиБ. Decode читает тело через http.MaxBytesReader, и чтение обрывается с типизированной ошибкой, а не съедает память сервера.',
  },
  {
    state: {
      source: 'decode',
      error: 'http: request body too large (limit 1 MiB)',
      problem: problem({ code: 'body_too_large', status: 413, title: 'Request body too large', detail: 'Request body must not exceed 1 MiB', instance: '/v1/chats/0199…a1/messages', requestId: 'e08b63' }),
      highlightFields: COMMON,
      headers: [MIME],
    },
    caption: 'Итог: пять совершенно разных источников — домен, обёртка, маршрутизатор, паника, чтение тела — и один формат на выходе. Поле type — это адрес: общий префикс problems плюс код ошибки, а сам код остаётся стабильным ключом, по которому клиент ветвит логику.',
  },
]

export function problemMappingScenario(): Step<ProblemMappingState>[] {
  return FRAMES.map((fr) => ({ state: fr.state, caption: fr.caption, highlight: [fr.state.source] }))
}
