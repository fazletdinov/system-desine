import { describe, expect, it } from 'vitest'
import { encodeToken, jwtSimulation as sim, MAIN_TOKEN, verify, type JwtAction } from './jwt'

const run = (...actions: JwtAction[]) => actions.reduce(sim.apply, sim.initial())

describe('jwt', () => {
  it('песочница кодирует ровно те байты, что подписаны', () => {
    expect(encodeToken(sim.initial())).toBe(MAIN_TOKEN)
  })
  it('исходный токен проходит', () => {
    expect(verify(sim.initial())).toEqual({ ok: true, rule: 'ok' })
  })
  it.each<[string, JwtAction, string]>([
    ['подмена sub ломает подпись', { type: 'sub', value: 'eve' }, 'signature'],
    ['байт подписи изменён', { type: 'tamper-signature' }, 'signature'],
    ['alg: none отклоняется до проверки подписи', { type: 'alg', value: 'none' }, 'alg'],
    ['HS256 с открытым ключом как секретом', { type: 'alg', value: 'HS256' }, 'alg'],
    ['неизвестный kid', { type: 'kid', value: 'chuzhoi-kluch' }, 'kid'],
    ['часы ушли на 16 минут вперёд', { type: 'clock', seconds: 16 * 60 }, 'exp'],
    ['часы отстали на 5 минут', { type: 'clock', seconds: -5 * 60 }, 'iat'],
    ['токен выпущен для другого сервиса', { type: 'token', which: 'foreign-aud' }, 'aud'],
  ])('%s', (_, action, rule) => {
    expect(verify(run(action))).toEqual({ ok: false, rule })
  })
  it('допуск 30 секунд: 15 минут 20 секунд ещё проходят', () => {
    expect(verify(run({ type: 'clock', seconds: 15 * 60 + 20 })).ok).toBe(true)
  })
  it('сброс возвращает исходный токен', () => {
    expect(encodeToken(run({ type: 'sub', value: 'eve' }, { type: 'reset' }))).toBe(MAIN_TOKEN)
  })
})
