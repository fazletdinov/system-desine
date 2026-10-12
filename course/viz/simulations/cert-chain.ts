import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Step } from '../core/types'

export type CertId = 'leaf' | 'intermediate' | 'root'
export type CertStatus = 'pending' | 'ok' | 'fail'

export interface Cert {
  id: CertId
  subject: string
  issuer: string
  status: CertStatus
}

export interface CertChainState {
  certs: readonly Cert[]
  check: string
}

export const HOSTNAME = 'api.messenger.example'
export const UNKNOWN_AUTHORITY_ERROR = 'x509: certificate signed by unknown authority'
// Индекс первого шага ветки-ошибки: до него — успешная проверка (6 шагов).
export const ERROR_BRANCH_START = 6

type Statuses = Record<CertId, CertStatus>

const SUBJECTS: Record<CertId, { subject: string; issuer: string }> = {
  leaf: { subject: HOSTNAME, issuer: 'Messenger Issuing CA' },
  intermediate: { subject: 'Messenger Issuing CA', issuer: 'Messenger Root CA' },
  root: { subject: 'Messenger Root CA', issuer: 'Messenger Root CA' },
}

const PENDING: Statuses = { leaf: 'pending', intermediate: 'pending', root: 'pending' }

interface Frame {
  statuses: Statuses
  check: string
  caption: Caption
}

const certs = (statuses: Statuses): Cert[] =>
  (['root', 'intermediate', 'leaf'] as const).map((id) => ({ id, ...SUBJECTS[id], status: statuses[id] }))

const SUCCESS: readonly Frame[] = [
  { statuses: PENDING, check: 'Сервер прислал два сертификата: свой и промежуточный', caption: [{ text: `Во время TLS-рукопожатия сервер присылает ` }, term('сертификат', 'certificate'), { text: ` ${HOSTNAME} (лист) и промежуточный сертификат, которым этот лист подписан. Корневого сертификата в присланном нет: он уже лежит в хранилище вашей ОС.` }] },
  { statuses: { ...PENDING, leaf: 'ok' }, check: 'Подпись листа проверена открытым ключом промежуточного', caption: 'Клиент проверяет электронную подпись листа открытым ключом из промежуточного сертификата. Подпись сходится: лист выдан именно этим промежуточным центром.' },
  { statuses: { leaf: 'ok', intermediate: 'ok', root: 'ok' }, check: 'Подпись промежуточного проверена ключом корня из хранилища ОС', caption: [{ text: 'Теперь то же для промежуточного: его подпись проверяется открытым ключом корня. Корень клиент не получает по сети, а берёт из хранилища ОС — цепочка уткнулась в то, чему клиент доверяет заранее. Так работает ' }, term('удостоверяющий центр', 'ca'), { text: '.' }] },
  { statuses: { leaf: 'ok', intermediate: 'ok', root: 'ok' }, check: 'Срок действия: сейчас между «действует с» и «действует до»', caption: 'Проверка срока: текущее время должно попадать между «действителен с» и «действителен по» у каждого сертификата цепочки. Просроченный сертификат отвергается, даже если подписи верны.' },
  { statuses: { leaf: 'ok', intermediate: 'ok', root: 'ok' }, check: `Имя ${HOSTNAME} есть в поле SAN листа`, caption: `Проверка имени: клиент подключался к ${HOSTNAME}, и это имя должно быть в списке SAN (Subject Alternative Name) листа. Иначе чужой настоящий сертификат подошёл бы к любому сайту.` },
  { statuses: { leaf: 'ok', intermediate: 'ok', root: 'ok' }, check: 'Цепочка принята, можно обмениваться ключами', caption: [{ text: 'Все проверки пройдены: подписи, срок, имя. Клиент уверен, что говорит с настоящим api.messenger.example, и ' }, term('TLS', 'tls'), { text: '-рукопожатие продолжается.' }] },
]

const FAILURE: readonly Frame[] = [
  { statuses: PENDING, check: 'Сервер прислал лист и промежуточный, но корня нет в хранилище ОС', caption: 'Теперь ветка ошибки. Например, сервер подписан корпоративным или самодельным корнем, о котором ОС клиента ничего не знает (так бывает в тестовых окружениях и за прокси с подменой сертификатов).' },
  { statuses: { ...PENDING, leaf: 'ok' }, check: 'Подпись листа промежуточным верна', caption: [{ text: 'Подпись листа сходится: он действительно выдан этим промежуточным центром. Но это пока ничего не доказывает — любой может завести свой ' }, term('центр сертификации', 'ca'), { text: ' и выдать себе сертификат.' }] },
  { statuses: { ...PENDING, leaf: 'ok', root: 'fail' }, check: 'Корень не найден в хранилище ОС', caption: 'Цепочка дошла до корня, а корня в хранилище нет. Доверять нечему: цепочка не упирается в известный клиенту центр.' },
  { statuses: { ...PENDING, leaf: 'ok', root: 'fail' }, check: UNKNOWN_AUTHORITY_ERROR, caption: [{ text: 'Go возвращает ошибку x509: certificate signed by unknown authority, и ' }, term('TLS', 'tls'), { text: '-соединение закрывается, не передав ни байта данных. Лечится добавлением корня в хранилище или его указанием в RootCAs — а не отключением проверки (InsecureSkipVerify).' }] },
]

export function certChainScenario(): Step<CertChainState>[] {
  return [...SUCCESS, ...FAILURE].map((f) => ({
    state: { certs: certs(f.statuses), check: f.check },
    caption: f.caption,
    highlight: (['leaf', 'intermediate', 'root'] as const).filter((id) => f.statuses[id] !== 'pending'),
  }))
}
