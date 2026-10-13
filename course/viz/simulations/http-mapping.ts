import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type Stage = 'json-in' | 'dto' | 'command' | 'domain' | 'dto-out' | 'json-out'

export const STAGES: readonly { id: Stage; label: string; layer: string }[] = [
  { id: 'json-in', label: 'JSON запроса', layer: 'сеть' },
  { id: 'dto', label: 'DTO запроса', layer: 'адаптер' },
  { id: 'command', label: 'Команда', layer: 'use case' },
  { id: 'domain', label: 'Доменное сообщение', layer: 'домен' },
  { id: 'dto-out', label: 'DTO ответа', layer: 'адаптер' },
  { id: 'json-out', label: 'JSON ответа', layer: 'сеть' },
]

export interface HttpMappingState {
  stage: Stage
  shape: string
}

// Идентификаторы сокращены, чтобы формы помещались на экран телефона.
const CHAT = '0199…a1'
const ALICE = '0199…01'
const MSG = '0199…f3'
const SEQ = 42
const SENT_AT = '2026-10-13T12:00:00Z'

interface Frame {
  state: HttpMappingState
  caption: Caption
}

const FRAMES: readonly Frame[] = [
  {
    state: {
      stage: 'json-in',
      shape: [`POST /chats/${CHAT}/messages`, `X-User-ID: ${ALICE}`, 'Content-Type: application/json', '', '{"text":"привет"}'].join('\n'),
    },
    caption: 'Одно сообщение «привет» и шесть его представлений. Сначала — то, что пришло по сети: путь с id чата, заголовок X-User-ID с автором и тело — JSON с одним полем. Идентификаторы здесь и дальше сокращены.',
  },
  {
    state: {
      stage: 'dto',
      shape: ['type sendMessageRequest struct {', '\tText string `json:"text"`', '}', '', '// после httpjson.Decode:', 'sendMessageRequest{Text: "привет"}'].join('\n'),
    },
    caption: [
      { text: 'httpjson.Decode разбирает тело в ' },
      term('DTO', 'dto'),
      { text: ' sendMessageRequest. Тип неэкспортируемый, живёт в пакете ' },
      term('адаптера', 'adapter'),
      { text: ', и только у него есть тег json: имя поля в API («text») задаётся здесь, а не в домене. Неизвестное поле или второй объект в теле — 400 ещё на этом шаге.' },
    ],
  },
  {
    state: {
      stage: 'command',
      shape: ['usecase.SendMessageCommand{', `\tChatID:   "${CHAT}", // из пути: r.PathValue("id")`, `\tAuthorID: "${ALICE}", // из заголовка X-User-ID`, '\tText:     "привет",  // из DTO', '}'].join('\n'),
    },
    caption: [
      { text: 'Обработчик собирает команду для ' },
      term('use case', 'use-case'),
      { text: ' из трёх источников: id чата — из пути, автор — из заголовка X-User-ID, текст — из DTO. В команде только строки и ни одного тега: use case не знает, что данные пришли по HTTP.' },
    ],
  },
  {
    state: {
      stage: 'domain',
      shape: [
        'domain.Message{ // поля неэкспортируемые',
        `\tid:       MessageID{"${MSG}"}`,
        `\tchatID:   ChatID{"${CHAT}"}`,
        `\tauthorID: UserID{"${ALICE}"}`,
        '\ttext:     Text{"привет"}',
        `\tseq:      ${SEQ} // выдал chat.Post`,
        `\tsentAt:   ${SENT_AT}`,
        '}',
      ].join('\n'),
    },
    caption: [
      { text: 'Use case проверил строки, а ' },
      term('агрегат', 'aggregate'),
      { text: ` выдал номер: domain.Message с seq ${SEQ}. Поля неэкспортируемые, читать их можно только через методы ID(), Text(), Seq()… — так домен гарантирует, что после Post сообщение никто не изменит. Внутри — ` },
      term('объекты-значения', 'value-object'),
      { text: ' ChatID, UserID, Text. Ни тегов, ни знания о форматах.' },
    ],
  },
  {
    state: {
      stage: 'dto-out',
      shape: ['json.Marshal(msg) // msg — domain.Message', '// → {}', '// encoding/json видит только экспортируемые поля,', '// а у domain.Message их нет'].join('\n'),
    },
    caption: 'Почему не отдать клиенту json.Marshal(msg)? Пакет encoding/json сериализует только экспортируемые поля — получится пустой объект {}. А если сделать поля экспортируемыми и навесить теги, API станет слепком домена: переименовали поле в домене — сломали всех клиентов. Домен и API должны меняться независимо.',
  },
  {
    state: {
      stage: 'dto-out',
      shape: [
        'type messageResponse struct {',
        '\tID       string `json:"id"`',
        '\tChatID   string `json:"chat_id"`',
        '\tAuthorID string `json:"author_id"`',
        '\tText     string `json:"text"`',
        '\tSeq      int64  `json:"seq"`',
        '\tSentAt   string `json:"sent_at"`',
        '}',
        '',
        `// toMessageResponse(msg): Text: "привет", Seq: ${SEQ}, …`,
      ].join('\n'),
    },
    caption: [
      { text: 'Поэтому есть DTO ответа messageResponse и ' },
      term('маппер', 'mapper'),
      { text: ' toMessageResponse: он читает домен через методы и раскладывает по полям с тегами json. Время превращается в строку RFC 3339 в UTC. Маппер знает и домен, и DTO — зависимость направлена внутрь, домен про DTO не знает.' },
    ],
  },
  {
    state: {
      stage: 'json-out',
      shape: [
        'HTTP/1.1 201 Created',
        'Content-Type: application/json; charset=utf-8',
        '',
        `{"id":"${MSG}","chat_id":"${CHAT}","author_id":"${ALICE}","text":"привет","seq":${SEQ},"sent_at":"${SENT_AT}"}`,
      ].join('\n'),
    },
    caption: 'httpjson.Write отдаёт 201 и JSON. Цепочка целиком: JSON → DTO → команда → домен → DTO → JSON. Полей больше, чем хочется писать руками, но каждое преобразование живёт в одном месте, и поменять API или домен можно, не трогая второе.',
  },
]

export function httpMappingScenario(): Step<HttpMappingState>[] {
  return FRAMES.map((f) => ({ state: f.state, caption: f.caption, highlight: [f.state.stage] }))
}
