import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type PipelineScenario = 'ok' | 'panic' | 'no-identity' | 'unknown-route' | 'wrong-method'
export type PipelineAt = 'request-id' | 'access-log' | 'recover' | 'auth' | 'router' | 'handler' | 'response'

// Порядок = порядок вложенности: внешний слой первый. Recover внутри AccessLog,
// чтобы паника дошла до лога уже как 500; auth до маршрутизации, поэтому
// неизвестный путь без личности получает 401, а не 404.
export const LAYERS: readonly { id: Exclude<PipelineAt, 'response'>; label: string; role: string }[] = [
  { id: 'request-id', label: 'RequestID', role: 'присваивает id запроса' },
  { id: 'access-log', label: 'AccessLog', role: 'пишет строку лога' },
  { id: 'recover', label: 'Recover', role: 'ловит панику' },
  { id: 'auth', label: 'auth', role: 'кладёт пользователя в context' },
  { id: 'router', label: 'ServeMux', role: 'выбирает обработчик' },
  { id: 'handler', label: 'хендлер', role: 'бизнес-сценарий' },
]

export const PIPELINE_IDS: readonly PipelineAt[] = [...LAYERS.map((l) => l.id), 'response']

export interface RequestPipelineState {
  scenario: PipelineScenario
  at: PipelineAt
  status: number | null
  requestId: string
  log: string[]
  request: string
  // Что ушло клиенту: показывается на кадре response и там, где слой ответил сам.
  reply: string | null
}

const RID = 'a41f7c'
const RID_PANIC = 'b72e09'
const RID_401 = 'c3d815'
const RID_404 = 'd90a26'
const RID_405 = 'e5b4f1'

const POST = 'POST /v1/chats/{id}/messages'

interface Frame {
  state: RequestPipelineState
  caption: Caption
}

const FRAMES: readonly Frame[] = [
  {
    state: { scenario: 'ok', at: 'request-id', status: null, requestId: RID, log: [], request: POST, reply: null },
    caption: [
      { text: 'Сценарий 1, успех. Запрос входит в самый внешний слой. ' },
      term('RequestID', 'request-id'),
      { text: ' берёт id из входящего X-Request-ID или создаёт свой, кладёт в context и в заголовок ответа. Дальше этот id попадёт в каждую строку лога и в тело каждой ошибки.' },
    ],
  },
  {
    state: { scenario: 'ok', at: 'handler', status: null, requestId: RID, log: [], request: POST, reply: null },
    caption: [
      { text: 'Пройдены AccessLog (запомнил время старта), Recover (взвёл защиту от паники), auth (положил пользователя в context) и ServeMux (выбрал шаблон по методу и пути). Каждый слой — это ' },
      term('middleware', 'middleware'),
      { text: ': функция, которая оборачивает следующий обработчик. Хендлер видит уже готовое окружение и занимается только делом.' },
    ],
  },
  {
    state: { scenario: 'ok', at: 'response', status: 201, requestId: RID, log: [`${RID} POST /v1/chats/{id}/messages 201 12ms`], request: POST, reply: '201 Created' },
    caption: 'Хендлер ответил 201. Ответ идёт наружу через те же слои в обратном порядке, и AccessLog, вернув управление, знает статус и длительность — только теперь он может написать одну строку: метод, путь, статус, длительность, id запроса. (В сцене строка сокращена — шаблон пути и миллисекунды; настоящая строка — JSON с самим путём и наносекундами.)',
  },
  {
    state: { scenario: 'panic', at: 'handler', status: null, requestId: RID_PANIC, log: [], request: POST, reply: null },
    caption: 'Сценарий 2, паника. Запрос дошёл до хендлера, и в нём случилась ошибка программиста: индекс за границей среза. Паника раскручивает стек вверх по слоям — в обратную сторону от запроса.',
  },
  {
    state: { scenario: 'panic', at: 'recover', status: 500, requestId: RID_PANIC, log: [], request: POST, reply: '500 Internal Server Error (problem+json без деталей)' },
    caption: 'Первым на пути стоит Recover. Он ловит панику, пишет стек в лог сервера, а клиенту отвечает 500 в общем формате ошибок — без текста паники. Процесс жив, соседние запросы не заметили ничего.',
  },
  {
    state: { scenario: 'panic', at: 'response', status: 500, requestId: RID_PANIC, log: [`${RID_PANIC} POST /v1/chats/{id}/messages 500 3ms`], request: POST, reply: '500 Internal Server Error' },
    caption: [
      { text: 'Строка в логе со статусом 500 — потому что ' },
      term('Recover', 'middleware'),
      { text: ' стоит внутри AccessLog. Поменяйте их местами, и паника пролетит мимо лога: запрос исчезнет из access-лога ровно тогда, когда он нужнее всего.' },
    ],
  },
  {
    state: { scenario: 'no-identity', at: 'auth', status: 401, requestId: RID_401, log: [], request: POST, reply: '401 Unauthorized' },
    caption: 'Сценарий 3, нет заголовка личности. RequestID, AccessLog и Recover пропустили запрос, а auth не нашёл X-User-ID и ответил 401 сам. Цепочка оборвалась: ServeMux и хендлер не вызывались, до бизнес-логики запрос не дошёл.',
  },
  {
    state: { scenario: 'no-identity', at: 'response', status: 401, requestId: RID_401, log: [`${RID_401} POST /v1/chats/{id}/messages 401 1ms`], request: POST, reply: '401 Unauthorized' },
    caption: 'Отказ тоже попал в лог: AccessLog стоит снаружи auth и видит любой ответ — и успешный, и обрезанный на полпути.',
  },
  {
    state: { scenario: 'unknown-route', at: 'router', status: 404, requestId: RID_404, log: [], request: 'GET /v1/nothing-here', reply: '404 Not Found (problem+json, code=not_found)' },
    caption: 'Сценарий 4, неизвестный путь. Запрос с личностью прошёл auth (аутентификация стоит до маршрутизации, поэтому без личности тот же путь получил бы 401, а не 404). ServeMux не нашёл шаблон — 404, и ответ оформлен в общем формате ошибок.',
  },
  {
    state: { scenario: 'unknown-route', at: 'response', status: 404, requestId: RID_404, log: [`${RID_404} GET /v1/nothing-here 404 1ms`], request: 'GET /v1/nothing-here', reply: '404 Not Found' },
    caption: 'В лог попадает ровно такая же строка, как у любого другого запроса. Для оператора «клиент ходит по несуществующему пути» — обычное наблюдаемое событие, а не особый случай.',
  },
  {
    state: { scenario: 'wrong-method', at: 'router', status: 405, requestId: RID_405, log: [], request: 'DELETE /v1/chats', reply: '405 Method Not Allowed, Allow: GET, HEAD, POST' },
    caption: 'Сценарий 5, неверный метод. Путь /v1/chats существует, но для DELETE шаблона нет. ServeMux отвечает 405 и сам добавляет заголовок Allow со списком допустимых методов: GET, HEAD, POST. Наш код нужен только затем, чтобы тело ответа тоже было в общем формате.',
  },
  {
    state: { scenario: 'wrong-method', at: 'response', status: 405, requestId: RID_405, log: [`${RID_405} DELETE /v1/chats 405 1ms`], request: 'DELETE /v1/chats', reply: '405 Method Not Allowed, Allow: GET, HEAD, POST' },
    caption: 'Итог: пять разных исходов, а лог и формат ошибок единые. Всё это сделали слои вокруг хендлера, сам хендлер про них не знает.',
  },
]

export function requestPipelineScenario(): Step<RequestPipelineState>[] {
  return FRAMES.map((f) => ({ state: f.state, caption: f.caption, highlight: [f.state.at] }))
}
