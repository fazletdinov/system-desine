import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type LayerId = 'http' | 'usecase' | 'domain' | 'store'

// Сверху вниз — от внешнего мира к хранилищу; зависимости в коде смотрят внутрь, к domain.
export const LAYERS: readonly { id: LayerId; label: string; pkg: string }[] = [
  { id: 'http', label: 'HTTP-адаптер', pkg: 'chat/adapters/httpapi' },
  { id: 'usecase', label: 'Use case', pkg: 'chat/usecase' },
  { id: 'domain', label: 'Домен', pkg: 'chat/domain' },
  { id: 'store', label: 'Хранилище', pkg: 'chat/adapters/memory' },
]

export interface LayerFlowState {
  active: LayerId
  sees: readonly string[]
  note: string
  attempt: number // попытка в цикле SendMessage.Execute; 0 — цикл ещё не начался
}

// Версия и seq в 1a растут вместе: их меняет только Post. Начинаем с чата, где уже 41 сообщение.
const SEQ_BEFORE = 41

interface Frame {
  state: LayerFlowState
  caption: Caption
}

const FRAMES: readonly Frame[] = [
  {
    state: {
      active: 'http',
      sees: ['*http.Request', 'r.PathValue("id") — id чата', 'заголовок X-User-ID — автор', 'sendMessageRequest{Text: "привет"}'],
      note: 'httpjson.Decode: лимит тела, ровно один JSON-объект',
      attempt: 0,
    },
    caption: [
      { text: 'POST /chats/{id}/messages приходит в обработчик — ' },
      term('адаптер', 'adapter'),
      { text: ' HTTP. Только он видит запрос целиком: путь, заголовки, тело. Тело он разбирает в ' },
      term('DTO', 'dto'),
      { text: ' sendMessageRequest — тип с тегами json, живущий в пакете адаптера.' },
    ],
  },
  {
    state: {
      active: 'usecase',
      sees: ['ctx context.Context', 'SendMessageCommand{ChatID, AuthorID, Text string}'],
      note: 'Execute(ctx, cmd) — единственный публичный метод',
      attempt: 0,
    },
    caption: [
      { text: 'Обработчик собирает команду SendMessageCommand — три строки — и вызывает ' },
      term('use case', 'use-case'),
      { text: '. Команда объявлена в пакете usecase, поэтому use case ничего не знает про HTTP: тот же вызов сделает CLI, gRPC или тест.' },
    ],
  },
  {
    state: {
      active: 'usecase',
      sees: ['domain.ChatID', 'domain.UserID — автор', 'domain.Text', 'MessageID — от порта IDGenerator', 'sentAt — от порта Clock'],
      note: 'ParseChatID, ParseUserID, NewText — до любого похода в хранилище',
      attempt: 0,
    },
    caption: 'Use case превращает строки в доменные типы: domain.ParseChatID, ParseUserID, NewText. Кривые данные отсекаются здесь, до хранилища. Id сообщения и время берутся один раз — до цикла повторов, чтобы повтор не создал второе сообщение.',
  },
  {
    state: {
      active: 'store',
      sees: ['ctx', 'domain.ChatID', `снимок чата: lastSeq ${SEQ_BEFORE}, version ${SEQ_BEFORE}`],
      note: 'FindChat через порт ChatFinder → RestoreChat(снимок)',
      attempt: 1,
    },
    caption: [
      { text: 'Попытка 1. Use case загружает чат через ' },
      term('порт', 'hexagonal-port'),
      { text: ` ChatFinder — интерфейс из одного метода, объявленный в пакете usecase. Какое хранилище за ним стоит, use case не знает; сейчас это memory.Store. Чат приходит с версией ${SEQ_BEFORE}.` },
    ],
  },
  {
    state: {
      active: 'domain',
      sees: ['*Chat: участники, lastSeq, version', 'UserID автора', 'Text', 'MessageID', 'sentAt'],
      note: `chat.Post: автор — участник? seq = ${SEQ_BEFORE} + 1 = ${SEQ_BEFORE + 1}, version → ${SEQ_BEFORE + 1}`,
      attempt: 1,
    },
    caption: [
      { text: 'chat.Post — бизнес-правило в ' },
      term('агрегате', 'aggregate'),
      { text: ': проверяет ' },
      term('инвариант', 'invariant'),
      { text: ` «автор — участник», выдаёт seq = lastSeq + 1 = ${SEQ_BEFORE + 1} и поднимает версию. Домен видит только доменные типы — ни запроса, ни JSON, ни хранилища.` },
    ],
  },
  {
    state: {
      active: 'store',
      sees: ['ctx', '*domain.Chat', 'domain.Message', `expectedVersion = ${SEQ_BEFORE}`],
      note: `AppendMessage(ctx, chat, msg, ${SEQ_BEFORE})`,
      attempt: 1,
    },
    caption: [
      { text: `Use case сохраняет: AppendMessage(ctx, chat, msg, expectedVersion). expectedVersion = ${SEQ_BEFORE} — версия, которую он прочитал. Это ` },
      term('оптимистическая блокировка', 'optimistic-locking'),
      { text: ': чат не запирают на время работы, а при записи проверяют, что его никто не изменил.' },
    ],
  },
  {
    state: {
      active: 'store',
      sees: [`в хранилище version ${SEQ_BEFORE + 1} ≠ ${SEQ_BEFORE}`, 'domain.ErrConcurrentUpdate'],
      note: 'конфликт версии: ничего не записано',
      attempt: 1,
    },
    caption: `Пока шла попытка 1, Боб успел отправить своё сообщение — оно получило seq ${SEQ_BEFORE + 1}, версия в хранилище стала ${SEQ_BEFORE + 1}. Проверка и запись идут под одной блокировкой, версии не совпали — хранилище ничего не пишет и возвращает domain.ErrConcurrentUpdate. Без проверки два сообщения получили бы один seq.`,
  },
  {
    state: {
      active: 'domain',
      sees: [`*Chat: lastSeq ${SEQ_BEFORE + 1}, version ${SEQ_BEFORE + 1}`, 'тот же MessageID', 'тот же Text'],
      note: `заново FindChat и Post: seq = ${SEQ_BEFORE + 2}`,
      attempt: 2,
    },
    caption: `Попытка 2 из MaxSendAttempts = 3. Use case заново читает чат (версия ${SEQ_BEFORE + 1}) и снова зовёт Post: теперь seq = ${SEQ_BEFORE + 2}. Повторяется вся связка «прочитать → проверить правила → записать», а не одна запись: правила агрегата проверяются на свежем состоянии. Сейчас из правил — только «автор — участник», но когда появится, например, удаление участников, повтор без новой проверки пропустил бы сообщение от уже исключённого.`,
  },
  {
    state: {
      active: 'store',
      sees: ['*domain.Chat', 'domain.Message', `expectedVersion = ${SEQ_BEFORE + 1}`],
      note: `версии совпали: снимок чата и сообщение seq ${SEQ_BEFORE + 2} сохранены`,
      attempt: 2,
    },
    caption: `AppendMessage с expectedVersion = ${SEQ_BEFORE + 1}: версии совпали, хранилище записывает новое состояние чата и сообщение. Если бы и третья попытка проиграла гонку, use case вернул бы ErrConcurrentUpdate, обёрнутую через %w, — адаптер ответил бы 409 conflict.`,
  },
  {
    state: {
      active: 'http',
      sees: ['domain.Message', `messageResponse{…, Seq: ${SEQ_BEFORE + 2}}`, '201 Created'],
      note: 'toMessageResponse → httpjson.Write',
      attempt: 2,
    },
    caption: [
      { text: 'Use case вернул domain.Message. Обработчик переводит его в DTO messageResponse (' },
      term('маппер', 'mapper'),
      { text: ` toMessageResponse) и пишет JSON с кодом 201. Каждый слой видел только своё: HTTP — запрос, use case — команду и порты, домен — свои типы, хранилище — снимок чата.` },
    ],
  },
]

export function layerFlowScenario(): Step<LayerFlowState>[] {
  return FRAMES.map((f) => ({ state: f.state, caption: f.caption, highlight: [f.state.active] }))
}

// Как в chat/usecase/send_message.go.
export const MAX_SEND_ATTEMPTS = 3
