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
  // detail необязателен (RFC 9457): у 500 его нет нарочно — рассказывать нечего и нельзя.
  detail?: string
  instance: string
  requestId: string
  extra?: Record<string, unknown>
}

function problem(p: ProblemInit): Record<string, unknown> {
  const detail = p.detail === undefined ? {} : { detail: p.detail }
  return { type: BASE + p.code, title: p.title, status: p.status, ...detail, instance: p.instance, code: p.code, request_id: p.requestId, ...p.extra }
}

interface Frame {
  state: ProblemMappingState
  caption: Caption
}

// Тексты ошибок — настоящие: так их печатают домен, сгенерированная обёртка (oapi-codegen v2.8.0),
// рантайм Go и net/http. Ответы — те, что мессенджер отдаёт в уроке 1.10.
const DOMAIN_TEXT = '&domain.ValidationError{Field: "text", Reason: "пустое сообщение"}'
const PARAM_TEXT = 'Invalid format for parameter limit: error binding string parameter: strconv.ParseInt: parsing "abc": invalid syntax'
const ROUTER_TEXT = 'ServeMux: путь /v1/chats есть, метода DELETE нет → 405, Allow: POST'
const PANIC_TEXT = 'runtime error: index out of range [3] with length 3'
const DECODE_TEXT = 'http: request body too large'
const MESSAGES = '/v1/chats/0199…a1/messages'
const COMMON = ['type', 'status', 'code']

const FRAMES: readonly Frame[] = [
  {
    state: { source: 'domain', error: DOMAIN_TEXT, problem: null, highlightFields: [], headers: [] },
    caption: 'Источник 1, домен. Use case вернул доменную ошибку ValidationError: текст сообщения пуст. В ней нет ни статуса, ни JSON — домен ничего не знает про HTTP. Решить, во что она превратится, должен слой на границе.',
  },
  {
    state: {
      source: 'domain',
      error: DOMAIN_TEXT,
      problem: problem({ code: 'invalid_input', status: 400, title: 'Некорректные данные', detail: 'пустое сообщение', instance: MESSAGES, requestId: 'a41f7c', extra: { invalid_params: [{ name: 'text', reason: 'пустое сообщение' }] } }),
      highlightFields: [...COMMON, 'invalid_params'],
      headers: [MIME],
    },
    caption: [
      { text: 'Таблица контекста находит тип через errors.As и строит ' },
      term('problem details', 'problem-details'),
      { text: ': 400, код invalid_input, а имя поля уходит в invalid_params. Клиент подсветит именно поле text, не разбирая человеческий текст.' },
    ],
  },
  {
    state: { source: 'param', error: PARAM_TEXT, problem: null, highlightFields: [], headers: [] },
    caption: 'Источник 2, параметр запроса. GET …/messages?limit=abc: сгенерированная обёртка не смогла превратить «abc» в число и вызвала наш обработчик ошибок параметров. До нашего хендлера дело даже не дошло. Текст ошибки — на языке Go: strconv.ParseInt клиенту ничего не скажет.',
  },
  {
    state: {
      source: 'param',
      error: PARAM_TEXT,
      problem: problem({ code: 'invalid_input', status: 400, title: 'Некорректные данные', detail: 'неверный формат', instance: MESSAGES, requestId: 'b3c19d', extra: { invalid_params: [{ name: 'limit', reason: 'неверный формат' }] } }),
      highlightFields: [...COMMON, 'invalid_params'],
      headers: [MIME],
    },
    caption: [
      { text: 'Тот же код invalid_input и та же форма, но другой источник: клиенту всё равно, кто споткнулся — домен или обёртка. Один ' },
      term('маппер', 'mapper'),
      { text: ' ошибок на все источники — один код и один формат на все ошибки ввода. Из текста обёртки взято только имя параметра.' },
    ],
  },
  {
    state: { source: 'router', error: ROUTER_TEXT, problem: null, highlightFields: [], headers: [] },
    caption: 'Источник 3, маршрутизатор. Путь /v1/chats есть, метода DELETE у него нет. ServeMux сам решил отдать 405 и подготовил заголовок Allow с единственным методом этого пути — POST. Своим текстом он ответил бы обычной строкой text/plain.',
  },
  {
    state: {
      source: 'router',
      error: ROUTER_TEXT,
      problem: problem({ code: 'method_not_allowed', status: 405, title: 'Метод не поддерживается', detail: 'метод не поддерживается для этого пути', instance: '/v1/chats', requestId: 'c52e80' }),
      highlightFields: COMMON,
      headers: ['Allow: POST', MIME],
    },
    caption: 'Мы оставляем Allow как есть и подменяем только тело: теперь оно в общем формате с кодом method_not_allowed. Заголовок Allow — часть протокола (RFC 9110 требует его у 405), его нельзя терять.',
  },
  {
    state: { source: 'panic', error: PANIC_TEXT, problem: null, highlightFields: [], headers: [] },
    caption: 'Источник 4, паника. Хендлер обратился к срезу за границей: runtime error. Это не ошибка пользователя, и рассказывать о ней снаружи нельзя: внутри текст ошибки может содержать пути, имена таблиц, фрагменты запросов.',
  },
  {
    state: {
      source: 'panic',
      error: PANIC_TEXT,
      problem: problem({ code: 'internal', status: 500, title: 'Внутренняя ошибка сервера', instance: MESSAGES, requestId: 'd7a4e2' }),
      highlightFields: [...COMMON, 'request_id'],
      headers: [MIME],
    },
    caption: [
      { text: 'Клиент получает 500 без detail — только неизменный title и ' },
      term('request_id', 'request-id'),
      { text: '. Подробности — стек и текст паники — пишутся только в журнал под тем же id. Так поддержка по одному id находит причину, а посторонний ничего не узнаёт о внутренностях.' },
    ],
  },
  {
    state: { source: 'decode', error: DECODE_TEXT, problem: null, highlightFields: [], headers: [] },
    caption: 'Источник 5, тело запроса. Клиент прислал больше 1 МиБ. Decode читает тело через http.MaxBytesReader, и чтение обрывается с типизированной ошибкой *http.MaxBytesError, а не съедает память сервера.',
  },
  {
    state: {
      source: 'decode',
      error: DECODE_TEXT,
      problem: problem({ code: 'body_too_large', status: 413, title: 'Слишком большое тело запроса', detail: 'тело больше 1048576 байт', instance: MESSAGES, requestId: 'e08b63' }),
      highlightFields: COMMON,
      headers: [MIME],
    },
    caption: 'Итог: пять совершенно разных источников — домен, обёртка, маршрутизатор, паника, чтение тела — и один формат на выходе. Поле type — это адрес: общий префикс problems плюс код ошибки, а сам код остаётся стабильным ключом, по которому клиент ветвит логику.',
  },
]

export function problemMappingScenario(): Step<ProblemMappingState>[] {
  return FRAMES.map((fr) => ({ state: fr.state, caption: fr.caption, highlight: [fr.state.source] }))
}
