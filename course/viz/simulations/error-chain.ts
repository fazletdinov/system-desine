import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

// Один «конверт» в цепочке ошибок: кто его создал и что вернёт его Error().
export interface ErrorLayer {
  where: string
  message: string
}

export interface ErrorChainState {
  // От внешней ошибки (её держит вызывающий) к самой внутренней — в том порядке,
  // в каком errors.Is и errors.As идут по цепочке через Unwrap.
  layers: readonly ErrorLayer[]
  isNotFound: boolean // errors.Is(err, domain.ErrChatNotFound)
  asValidation: boolean // errors.As(err, &ve), где ve — *domain.ValidationError
  status: number | null // ответ HTTP-адаптера; null — до адаптера ещё не дошли
}

// Тексты — как в chat/domain/errors.go и text.go мессенджера.
export const CHAT_NOT_FOUND_TEXT = 'чат не найден'
const VALIDATION_TEXT = 'text: пустое сообщение'

const store: ErrorLayer = { where: 'memory.Store.FindChat: return domain.ErrChatNotFound', message: CHAT_NOT_FOUND_TEXT }
const wrappedW: ErrorLayer = { where: 'SendMessage.Execute: fmt.Errorf("send message: %w", err)', message: `send message: ${CHAT_NOT_FOUND_TEXT}` }
const wrappedV: ErrorLayer = { where: 'SendMessage.Execute: fmt.Errorf("send message: %v", err)', message: `send message: ${CHAT_NOT_FOUND_TEXT}` }
const validation: ErrorLayer = { where: 'domain.NewText: &ValidationError{Field: "text", Reason: "пустое сообщение"}', message: VALIDATION_TEXT }

interface Frame {
  layers: readonly ErrorLayer[]
  status: number | null
  caption: Caption
}

const FRAMES: readonly Frame[] = [
  {
    layers: [store],
    status: null,
    caption: [
      { text: 'Клиент шлёт сообщение в чат, которого нет. Хранилище возвращает ' },
      term('сигнальную ошибку', 'sentinel-error'),
      { text: ' domain.ErrChatNotFound — переменную пакета domain. errors.Is сравнивает с ней по идентичности значения, а не по тексту: текст «чат не найден» — для людей и логов.' },
    ],
  },
  {
    layers: [wrappedW, store],
    status: null,
    caption: [
      { text: 'Use case добавляет контекст — ' },
      term('оборачивает', 'error-wrapping'),
      { text: ' ошибку: fmt.Errorf("send message: %w", err). Получается новый «конверт»: его Error() — «send message: чат не найден», а метод Unwrap() возвращает исходную ошибку. errors.Is разворачивает конверты по очереди и находит ErrChatNotFound внутри.' },
    ],
  },
  {
    layers: [wrappedV],
    status: 500,
    caption: [
      { text: 'Вариант с ошибкой: кто-то написал %v вместо %w. Текст Error() буква в букву тот же, но %v лишь вставляет текст ошибки в строку: получается новая ошибка без Unwrap, исходной внутри нет. errors.Is → false, адаптер не узнаёт «не найден» и отвечает 500 internal. Глазами в логе разницу не увидеть — её ловит тест или линтер errorlint.' },
    ],
  },
  {
    layers: [wrappedW, store],
    status: null,
    caption: 'Исправляем на %w — цепочка снова цела, ErrChatNotFound виден через обёртку. Правило: оборачивайте через %w, когда добавляете контекст и вызывающему может понадобиться различить причину. %v уместен, только когда вы сознательно прячете причину от вызывающих.',
  },
  {
    layers: [wrappedW, store],
    status: 404,
    caption: [
      { text: 'HTTP-' },
      term('адаптер', 'adapter'),
      { text: ' — единственное место, где ошибка превращается в статус: errors.Is(err, domain.ErrChatNotFound) → 404 с кодом not_found. Домен и use case про HTTP-статусы не знают.' },
    ],
  },
  {
    layers: [validation],
    status: null,
    caption: 'Вторая история: текст из одних пробелов. domain.NewText возвращает *ValidationError{Field: "text", Reason: "пустое сообщение"}, её Error() — «text: пустое сообщение». Use case возвращает её как есть: поле уже говорит, что не так, добавлять нечего. Сигнальная переменная здесь не подошла бы — клиенту нужно знать, какое поле неверно.',
  },
  {
    layers: [validation],
    status: 400,
    caption: 'Адаптер вызывает errors.As(err, &ve): As идёт по цепочке и ищет ошибку типа *ValidationError; находит — кладёт её в ve. Из ve.Field адаптер берёт имя поля: 400, код invalid_input, field "text". Поэтому As требует указатель на переменную нужного типа — ему некуда иначе записать находку.',
  },
  {
    layers: [validation],
    status: 400,
    caption: 'А errors.Is(err, domain.ErrInvalid) тоже даёт true, хотя ErrInvalid никто не оборачивал. Причина — метод Is у *ValidationError: errors.Is на каждом конверте сначала сравнивает его с целью через ==, а если у конверта есть метод Is(error) bool — спрашивает и его. Наш Is отвечает «да» на ErrInvalid, так что кому всё равно, какое поле неверно, хватает одной проверки.',
  },
  {
    layers: [validation],
    status: 400,
    caption: 'Итог: сигнальные ошибки различают через errors.Is, ошибки с данными — через errors.As; обёртка — только %w. Адаптер проверяет по порядку: ошибка разбора тела → ValidationError → сигнальные (404, 403, 409) → отмена и дедлайн контекста → всё остальное 500 без деталей наружу.',
  },
]

const hasNotFound = (layers: readonly ErrorLayer[]): boolean => layers.some((l) => l.message === CHAT_NOT_FOUND_TEXT)

export function errorChainScenario(): Step<ErrorChainState>[] {
  return FRAMES.map((f) => ({
    state: {
      layers: f.layers,
      isNotFound: hasNotFound(f.layers),
      asValidation: f.layers.some((l) => l === validation),
      status: f.status,
    },
    caption: f.caption,
  }))
}

// Код в теле ответа для статусов этой сцены (таблица ошибок урока 1.7).
export const STATUS_CODE: Readonly<Record<number, string>> = { 400: 'invalid_input', 404: 'not_found', 500: 'internal' }
