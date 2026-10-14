import type { Caption } from '../core/caption'
import { term } from '../core/caption'
import type { Simulation, Step } from '../core/types'

// Настоящие токены, подписанные Ed25519 на учебном ключе (seed 0x01…0x20, задача 2, шаг 4).
// Проверить подпись можно на jwt.io открытым ключом PUBLIC_X.
export const MAIN_TOKEN =
  'eyJhbGciOiJFZERTQSIsImtpZCI6IldXcG5fcGZIdWk5WUtSNENadFFzREdNdTdfR2NoMnpZQ2hmU3ZueGd0UGsiLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJtZXNzZW5nZXIiLCJzdWIiOiIwMTkyOWY2ZS03YTFjLTdjM2UtOWE1MS02ZjJkMWI4ZTRhMDEiLCJhdWQiOlsibWVzc2VuZ2VyLWFwaSJdLCJleHAiOjE3OTE5ODAxMDAsIm5iZiI6MTc5MTk3OTIwMCwiaWF0IjoxNzkxOTc5MjAwLCJqdGkiOiIwMTkyOWY2ZS03YTFjLTdjM2UtOWE1MS02ZjJkMWI4ZTRhZmYifQ.YWHJ-Inm2MQ2OZpTXCO2tNZx0OfOVtyJHMZHv0Sq7DCKmivcaBz8_wARWmz5373x79rnceDZKfOIuwvHPxFIAQ'
export const FOREIGN_AUD_TOKEN =
  'eyJhbGciOiJFZERTQSIsImtpZCI6IldXcG5fcGZIdWk5WUtSNENadFFzREdNdTdfR2NoMnpZQ2hmU3ZueGd0UGsiLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJtZXNzZW5nZXIiLCJzdWIiOiIwMTkyOWY2ZS03YTFjLTdjM2UtOWE1MS02ZjJkMWI4ZTRhMDEiLCJhdWQiOlsiYmlsbGluZy1hcGkiXSwiZXhwIjoxNzkxOTgwMTAwLCJuYmYiOjE3OTE5NzkyMDAsImlhdCI6MTc5MTk3OTIwMCwianRpIjoiMDE5MjlmNmUtN2ExYy03YzNlLTlhNTEtNmYyZDFiOGU0YWZmIn0.LmYzN0Gkb4nnKmun1V08D0nOYCFNoC570GYKspgRKaJooXWWHxbK1UtYuDu3iQ3GxgbEI_IH5jchv2baIQ4gDA'
export const KID = 'WWpn_pfHui9YKR4CZtQsDGMu7_Gch2zYChfSvnxgtPk'
export const PUBLIC_X = 'ebVWLo_mVPlAeLES6KmLp5AfhTrmlb7X4OORC60ElmQ'

export const ISSUER = 'messenger'
export const AUDIENCE = 'messenger-api'
export const LEEWAY_SEC = 30
const IAT = Date.UTC(2026, 9, 14, 12, 0, 0) / 1000

export interface JwtHeader { alg: string; kid: string; typ: string }
export interface JwtPayload { iss: string; sub: string; aud: string[]; exp: number; nbf: number; iat: number; jti: string }

export interface JwtState {
  token: 'main' | 'foreign-aud'
  header: JwtHeader
  payload: JwtPayload
  signature: string
  clockOffsetSec: number
  last: JwtAction['type'] | 'start'
}

export type JwtAction =
  | { type: 'sub'; value: string }
  | { type: 'alg'; value: 'none' | 'HS256' }
  | { type: 'kid'; value: string }
  | { type: 'tamper-signature' }
  | { type: 'clock'; seconds: number }
  | { type: 'token'; which: 'main' | 'foreign-aud' }
  | { type: 'reset' }

export type Rule = 'ok' | 'alg' | 'kid' | 'signature' | 'exp' | 'iat' | 'nbf' | 'aud' | 'iss'

const parts = (t: string): [string, string, string] => t.split('.') as [string, string, string]

function b64urlDecode(s: string): string {
  const bin = atob(s.replace(/-/g, '+').replace(/_/g, '/'))
  return new TextDecoder().decode(Uint8Array.from(bin, (c) => c.charCodeAt(0)))
}

function b64urlEncode(s: string): string {
  const bytes = new TextEncoder().encode(s)
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function fromToken(t: string, which: JwtState['token']): JwtState {
  const [h, p, sig] = parts(t)
  return { token: which, header: JSON.parse(b64urlDecode(h)), payload: JSON.parse(b64urlDecode(p)), signature: sig, clockOffsetSec: 0, last: 'start' }
}

const original = (s: JwtState): string => (s.token === 'main' ? MAIN_TOKEN : FOREIGN_AUD_TOKEN)

// Порядок полей — как у Go (заголовок: alg, kid, typ; claims — порядок RegisteredClaims),
// иначе байты разойдутся с подписанными даже без правок.
export function encodeToken(s: JwtState): string {
  const h = { alg: s.header.alg, kid: s.header.kid, typ: s.header.typ }
  const p = { iss: s.payload.iss, sub: s.payload.sub, aud: s.payload.aud, exp: s.payload.exp, nbf: s.payload.nbf, iat: s.payload.iat, jti: s.payload.jti }
  return `${b64urlEncode(JSON.stringify(h))}.${b64urlEncode(JSON.stringify(p))}.${s.signature}`
}

// verify повторяет порядок golang-jwt v5 с нашими опциями: сначала алгоритм из белого списка,
// потом ключ по kid, потом подпись и только потом claims. Подпись в модели верна, если
// подписанные байты и сама подпись не менялись — так её и проверяет Ed25519.
export function verify(s: JwtState): { ok: boolean; rule: Rule } {
  const fail = (rule: Rule) => ({ ok: false, rule })
  if (s.header.alg !== 'EdDSA') return fail('alg')
  if (s.header.kid !== KID) return fail('kid')
  const [h, p] = parts(encodeToken(s))
  const [oh, op, osig] = parts(original(s))
  if (h !== oh || p !== op || s.signature !== osig) return fail('signature')
  const now = IAT + s.clockOffsetSec
  // Граница строгая, как в golang-jwt/jwt/v5 (сравнение через Before): ровно exp + допуск — уже отказ.
  if (now >= s.payload.exp + LEEWAY_SEC) return fail('exp')
  if (now + LEEWAY_SEC < s.payload.iat) return fail('iat')
  if (now + LEEWAY_SEC < s.payload.nbf) return fail('nbf')
  if (!s.payload.aud.includes(AUDIENCE)) return fail('aud')
  if (s.payload.iss !== ISSUER) return fail('iss')
  return { ok: true, rule: 'ok' }
}

function flipLastChar(sig: string): string {
  const c = sig.at(-1) === 'A' ? 'B' : 'A'
  return sig.slice(0, -1) + c
}

export const jwtSimulation: Simulation<JwtState, JwtAction> = {
  initial: () => fromToken(MAIN_TOKEN, 'main'),
  apply: (s, a) => {
    switch (a.type) {
      case 'reset':
        return fromToken(MAIN_TOKEN, 'main')
      case 'token':
        return { ...fromToken(a.which === 'main' ? MAIN_TOKEN : FOREIGN_AUD_TOKEN, a.which), last: 'token' }
      case 'sub':
        return { ...s, payload: { ...s.payload, sub: a.value }, last: 'sub' }
      case 'alg':
        return { ...s, header: { ...s.header, alg: a.value }, last: 'alg' }
      case 'kid':
        return { ...s, header: { ...s.header, kid: a.value }, last: 'kid' }
      case 'tamper-signature':
        return { ...s, signature: flipLastChar(s.signature), last: 'tamper-signature' }
      case 'clock':
        return { ...s, clockOffsetSec: s.clockOffsetSec + a.seconds, last: 'clock' }
    }
  },
  steps: (s) => [{ state: s, caption: captionFor(s) } satisfies Step<JwtState>],
}

const WHY: Record<Rule, string> = {
  ok: 'Токен прошёл все проверки: алгоритм из белого списка, известный ключ, подпись сходится, срок не истёк, аудитория наша.',
  alg: 'Отклонён до проверки подписи: сервер принимает только EdDSA. Если бы он верил полю alg из самого токена, атакующий выбрал бы none или HS256 с открытым ключом в роли секрета.',
  kid: 'Ключа с таким kid сервер не знает. Ключ выбирают по kid только из своего списка, а не из адреса, присланного в токене.',
  signature: 'Подпись не сходится: изменён хотя бы один байт заголовка, данных или подписи. Без закрытого ключа новую подпись не получить.',
  exp: 'Срок жизни истёк (exp плюс допуск 30 секунд на расхождение часов). Короткий срок — наша замена мгновенному отзыву токена.',
  iat: 'Токен выпущен «в будущем» по часам сервера дальше допуска — часы рассинхронизированы или токен подделан.',
  nbf: 'Токен ещё не действует (nbf в будущем дальше допуска).',
  aud: 'Подпись верна, но токен выпущен для другого сервиса (aud). Чужой токен нельзя предъявлять нам, даже если ключ тот же.',
  iss: 'Токен выпустил не наш сервис (iss).',
}

function captionFor(s: JwtState): Caption {
  const v = verify(s)
  return [
    term('JWT', 'jwt'),
    { text: ' — три части в base64url: заголовок, ' },
    term('claims', 'claim'),
    { text: ' и ' },
    term('подпись', 'digital-signature'),
    { text: `. Это не шифрование: прочитать может любой. ${WHY[v.rule]}` },
  ]
}
