import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type Level = 'domain-unit' | 'usecase-fake' | 'contract' | 'http' | 'race'

// Уровни пирамиды снизу вверх; race — не уровень, а режим запуска любых тестов (go test -race).
export const LEVELS: readonly { id: Level; label: string; tool: string }[] = [
  { id: 'domain-unit', label: 'Юнит-тесты домена', tool: 'go test ./internal/chat/domain' },
  { id: 'usecase-fake', label: 'Use case на фейках', tool: 'фейки портов, без сети и диска' },
  { id: 'contract', label: 'Контрактные тесты хранилища', tool: 'storetest.Run' },
  { id: 'http', label: 'Тесты обработчика', tool: 'httptest.NewRecorder' },
  { id: 'race', label: 'Детектор гонок', tool: 'go test -race' },
]

export interface TestLayersState {
  bug: string
  caughtBy: Level
  cost: string
}

interface Frame {
  state: TestLayersState
  caption: Caption
}

const FRAMES: readonly Frame[] = [
  {
    state: { bug: 'NewText пропускает 4097 символов', caughtBy: 'domain-unit', cost: 'микросекунды: чистая функция, ни сети, ни диска' },
    caption: [
      { text: 'Ошибка на единицу: в NewText написали n > TextMaxLen+1 вместо n > TextMaxLen — и сообщение в 4097 символов проходит. Ловит ' },
      term('табличный тест', 'table-driven-test'),
      { text: ' TestNewText: строки «лимит» и «лимит + 1» кириллицей. Самый дешёвый уровень — основание ' },
      term('пирамиды тестов', 'test-pyramid'),
      { text: ': таких тестов много, и бегут они за доли секунды.' },
    ],
  },
  {
    state: { bug: 'ListMessages не проверяет, что читатель — участник чата', caughtBy: 'usecase-fake', cost: 'миллисекунды: хранилище в памяти и фейки портов' },
    caption: [
      { text: 'В ListMessages забыли проверку chat.IsMember(reader) — Ева читает чужую переписку. Писать без членства домен не даст (это проверяет chat.Post), а чтение — правило ' },
      term('use case', 'use-case'),
      { text: '. Ловит тест use case TestNonMemberCannotSendOrRead — его вторая половина: Ева запрашивает историю чужого чата и должна получить ErrNotMember. Порты подменены ' },
      term('фейками', 'fake'),
      { text: ' — быстрыми рабочими реализациями в памяти, поэтому тест проверяет сценарий целиком без базы.' },
    ],
  },
  {
    state: { bug: 'хранилище не проверяет версию чата при записи', caughtBy: 'contract', cost: 'миллисекунды на memory.Store' },
    caption: [
      { text: 'AppendMessage записывает, не сравнив expectedVersion, — два сообщения получают один seq. Тесты use case на фейках этого не заметят, если фейк написан так же небрежно. Ловит ' },
      term('контрактный тест', 'contract-test'),
      { text: ' storetest.Run: одна батарея проверок, обязательная для любой реализации портов хранилища, — запись с устаревшей версией должна вернуть ErrConcurrentUpdate.' },
    ],
  },
  {
    state: { bug: 'ErrNotMember маппится в 500 вместо 403', caughtBy: 'http', cost: 'миллисекунды: httptest без сети' },
    caption: 'В problemFor (таблице контекста chat) забыли строку для ErrNotMember — она падает в «прочее» и уходит клиенту как 500 internal. Домен и use case тут ни при чём: ошибка в адаптере. Ловит тест обработчика: настоящий http.ServeMux, httptest.NewRecorder и фейковый use case, который возвращает ErrNotMember; ждём статус 403 и код not_member.',
  },
  {
    state: { bug: 'запись в map хранилища без мьютекса', caughtBy: 'race', cost: 'по документации Go: время × 2–20, память × 5–10' },
    caption: [
      { text: 'Хранилище пишет в map без sync.RWMutex. Обычный прогон то проходит, то падает с fatal error: concurrent map writes — ' },
      term('гонка данных', 'data-race'),
      { text: ' проявляется не на каждом запуске, а гонку на обычных полях без map рантайм не заметит вовсе. Ловит ' },
      term('детектор гонок', 'race-detector'),
      { text: ': go test -race следит за обращениями к памяти и надёжно сообщает о гонке, даже если в этом запуске ничего не упало. Это режим запуска любых тестов, а не отдельный уровень; нагрузку ему дают TestSendMessageConcurrent и контрактная проверка «одновременная запись».' },
    ],
  },
  {
    state: { bug: 'SQL в будущем Postgres-адаптере перепутал порядок сообщений', caughtBy: 'contract', cost: 'секунды: настоящий Postgres в контейнере (модуль 2a)' },
    caption: 'Модуль 2a: Postgres-адаптер забыл ORDER BY seq — сообщения приходят в произвольном порядке. Новых тестов писать не нужно: Postgres-адаптер вызовет тот же storetest.Run, и его проверка «история по seq» упадёт. Контракт пишется один раз и защищает каждую будущую реализацию порта.',
  },
]

export function testLayersScenario(): Step<TestLayersState>[] {
  return FRAMES.map((f) => ({ state: f.state, caption: f.caption, highlight: [f.state.caughtBy] }))
}
