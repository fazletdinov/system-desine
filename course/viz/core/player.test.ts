import { describe, expect, it } from 'vitest'
import { BASE_STEP_MS, initPlayer, keyToAction, playerReducer, stepDelayMs, type PlayerAction, type PlayerState } from './player'

const at = (index: number, playing = false): PlayerState => ({ index, total: 5, playing, speed: 1 })

describe('initPlayer', () => {
  it('начинает с первого шага на паузе', () => {
    expect(initPlayer(5)).toEqual({ index: 0, total: 5, playing: false, speed: 1 })
  })
  it.each([0, -1, 2.5, Number.NaN])('отклоняет total=%s', (total) => {
    expect(() => initPlayer(total)).toThrow(RangeError)
  })
})

describe('playerReducer', () => {
  it.each<[string, PlayerState, PlayerAction, Partial<PlayerState>]>([
    ['next двигает вперёд', at(1), { type: 'next' }, { index: 2 }],
    ['next не уходит за конец', at(4), { type: 'next' }, { index: 4 }],
    ['prev двигает назад', at(2), { type: 'prev' }, { index: 1 }],
    ['prev не уходит за начало', at(0), { type: 'prev' }, { index: 0 }],
    ['first', at(3), { type: 'first' }, { index: 0 }],
    ['last', at(1), { type: 'last' }, { index: 4 }],
    ['seek внутрь', at(0), { type: 'seek', index: 3 }, { index: 3 }],
    ['seek за границу зажимается', at(0), { type: 'seek', index: 99 }, { index: 4 }],
    ['seek отрицательный зажимается', at(2), { type: 'seek', index: -5 }, { index: 0 }],
    ['ручной шаг ставит паузу', at(1, true), { type: 'next' }, { index: 2, playing: false }],
    ['play', at(1), { type: 'play' }, { index: 1, playing: true }],
    ['play на последнем шаге начинает сначала', at(4), { type: 'play' }, { index: 0, playing: true }],
    ['pause', at(1, true), { type: 'pause' }, { playing: false }],
    ['toggle с паузы', at(1), { type: 'toggle' }, { playing: true }],
    ['toggle с воспроизведения', at(1, true), { type: 'toggle' }, { playing: false }],
    ['tick на паузе ничего не делает', at(1), { type: 'tick' }, { index: 1, playing: false }],
    ['tick двигает вперёд', at(1, true), { type: 'tick' }, { index: 2, playing: true }],
    ['tick на предпоследнем доходит до конца и останавливается', at(3, true), { type: 'tick' }, { index: 4, playing: false }],
    ['speed', at(1), { type: 'speed', speed: 2 }, { speed: 2 }],
    ['reset при смене шагов', at(3, true), { type: 'reset', total: 8 }, { index: 0, total: 8, playing: false }],
  ])('%s', (_name, state, action, expected) => {
    expect(playerReducer(state, action)).toMatchObject(expected)
  })
})

describe('stepDelayMs', () => {
  it('обратно пропорциональна скорости', () => {
    expect(stepDelayMs(1)).toBe(BASE_STEP_MS)
    expect(stepDelayMs(2)).toBe(BASE_STEP_MS / 2)
    expect(stepDelayMs(0.5)).toBe(BASE_STEP_MS * 2)
  })
})

describe('keyToAction', () => {
  const div = { tagName: 'FIGURE' }
  it.each<[string, { tagName: string } | null, PlayerAction | null]>([
    ['ArrowRight', div, { type: 'next' }],
    ['ArrowLeft', div, { type: 'prev' }],
    [' ', div, { type: 'toggle' }],
    ['Home', div, { type: 'first' }],
    ['End', div, { type: 'last' }],
    ['a', div, null],
    // Пробел на кнопке нажимает саму кнопку — иначе получится двойное срабатывание.
    [' ', { tagName: 'BUTTON' }, null],
    ['ArrowRight', { tagName: 'BUTTON' }, { type: 'next' }],
    // Внутри ползунка и полей клавиши принадлежат им.
    ['ArrowRight', { tagName: 'INPUT' }, null],
    ['ArrowLeft', { tagName: 'SELECT' }, null],
    ['ArrowRight', null, { type: 'next' }],
  ])('%j на %j → %j', (key, target, expected) => {
    expect(keyToAction(key, target)).toEqual(expected)
  })
})
