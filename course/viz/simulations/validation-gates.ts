import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

// Ворота, через которые проходит тело POST /chats/{id}/messages, — от границы HTTP к хранилищу.
export type Gate = 'json' | 'dto' | 'value-object' | 'aggregate' | 'stored'

export const GATES: readonly { id: Gate; label: string; check: string }[] = [
  { id: 'json', label: 'JSON', check: 'httpjson.Decode' },
  { id: 'dto', label: 'DTO → команда', check: 'обработчик' },
  { id: 'value-object', label: 'Объекты-значения', check: 'domain.NewText…' },
  { id: 'aggregate', label: 'Агрегат', check: '(*domain.Chat).Post' },
  { id: 'stored', label: 'Сохранено', check: 'AppendMessage' },
]

export interface GatesState {
  input: string
  gate: Gate
  verdict: 'pass' | 'reject'
  reason?: string
}

// Тексты отказов совпадают с кодом мессенджера (домен — урок 1.2, httpjson — урок 1.7).
export const TEXT_MAX_LEN = 4096
const LONG_RUNES = TEXT_MAX_LEN + 1
// «я» в UTF-8 занимает 2 байта.
const LONG_BYTES = LONG_RUNES * 2

export const INPUTS = {
  blank: '{"text":"   "}',
  tooLong: `{"text":"яяя…я"}  — ${LONG_RUNES} букв «я»`,
  wrongType: '{"text": 42}',
  stranger: '{"text":"привет"}  — от Евы, она не участница чата',
  member: '{"text":"привет"}  — от Алисы, участницы чата',
} as const

interface Frame {
  state: GatesState
  caption: Caption
}

const FRAMES: readonly Frame[] = [
  {
    state: { input: INPUTS.blank, gate: 'dto', verdict: 'pass' },
    caption: [
      { text: 'Прогон 1. Тело — корректный JSON, поле text — строка, так что ворота JSON и ' },
      term('DTO', 'dto'),
      { text: ' пропускают его: им всё равно, что строка состоит из пробелов. Они проверяют форму запроса, а не смысл. Дальше обработчик собирает команду SendMessageCommand: id чата из пути, автор из заголовка X-User-ID.' },
    ],
  },
  {
    state: { input: INPUTS.blank, gate: 'value-object', verdict: 'reject', reason: 'text: пустое сообщение → 400 invalid_input, field "text"' },
    caption: [
      term('Use case', 'use-case'),
      { text: ' превращает строки команды в ' },
      term('объекты-значения', 'value-object'),
      { text: '. domain.NewText обрезает пробелы по краям (strings.TrimSpace), остаётся 0 символов — отказ. Правило «сообщение не пустое» живёт в одном месте, в домене, и его нельзя обойти: другого способа получить domain.Text нет.' },
    ],
  },
  {
    state: { input: INPUTS.tooLong, gate: 'dto', verdict: 'pass' },
    caption: [
      { text: `Прогон 2. ${LONG_RUNES} букв «я» — это ${LONG_BYTES} байт: в UTF-8 каждая кириллическая буква занимает 2 байта. До лимита тела httpjson.MaxBodyBytes (1 МиБ) очень далеко, JSON корректный — ворота JSON и DTO пропускают.` },
    ],
  },
  {
    state: { input: INPUTS.tooLong, gate: 'value-object', verdict: 'reject', reason: `text: не длиннее ${TEXT_MAX_LEN} символов → 400 invalid_input, field "text"` },
    caption: [
      { text: `NewText считает символы функцией utf8.RuneCountInString — по рунам, а не по байтам: ${LONG_RUNES} рун > ${TEXT_MAX_LEN}, отказ. Если бы считали байты (len(s)), лимит срабатывал бы уже на 2049 «я», а эмодзи (4 байта) — на 1025: пользователь видел бы разные лимиты для разных алфавитов.` },
    ],
  },
  {
    state: {
      input: INPUTS.wrongType,
      gate: 'json',
      verdict: 'reject',
      reason: 'некорректный JSON: json: cannot unmarshal number into Go struct field sendMessageRequest.text of type string → 400 bad_json',
    },
    caption: [
      { text: 'Прогон 3. Поле text — число, а в DTO sendMessageRequest оно объявлено строкой. encoding/json отказывает уже при разборе, httpjson.Decode возвращает 400 с кодом bad_json. До use case и домена этот запрос не доходит: проверять типы полей — работа границы, а не бизнес-правил.' },
    ],
  },
  {
    state: { input: INPUTS.stranger, gate: 'value-object', verdict: 'pass' },
    caption: [
      { text: 'Прогон 4. Текст нормальный, id чата и автора — правильные UUID: все объекты-значения созданы. Но объект-значение проверяет только самого себя. Он не знает, чей это чат и кто в нём состоит, — для этого нужно состояние, которого у него нет.' },
    ],
  },
  {
    state: { input: INPUTS.stranger, gate: 'aggregate', verdict: 'reject', reason: 'domain.ErrNotMember («пользователь не участник чата») → 403 not_member' },
    caption: [
      { text: 'Use case загружает чат через порт ChatFinder и вызывает chat.Post. ' },
      term('Агрегат', 'aggregate'),
      { text: ' знает список участников и проверяет ' },
      term('инвариант', 'invariant'),
      { text: ' «писать может только участник»: Евы в списке нет — сигнальная ошибка domain.ErrNotMember, HTTP-адаптер превратит её в 403.' },
    ],
  },
  {
    state: { input: INPUTS.member, gate: 'stored', verdict: 'pass' },
    caption: [
      { text: 'Для сравнения — тот же текст от Алисы. Все ворота пройдены, Post выдал следующий seq, хранилище сохранило сообщение. Итог: форму запроса проверяет граница, отдельные значения — объекты-значения, правила, которым нужно состояние, — агрегат.' },
    ],
  },
]

export function validationGatesScenario(): Step<GatesState>[] {
  return FRAMES.map((f) => ({ state: f.state, caption: f.caption, highlight: [f.state.gate] }))
}
