export const SPEEDS = [0.5, 1, 2] as const
export type Speed = (typeof SPEEDS)[number]

// 1,8 с на шаг — успеть прочитать пояснение из 1–2 предложений.
export const BASE_STEP_MS = 1800

export interface PlayerState {
  index: number
  total: number
  playing: boolean
  speed: Speed
}

export type PlayerAction =
  | { type: 'next' }
  | { type: 'prev' }
  | { type: 'first' }
  | { type: 'last' }
  | { type: 'seek'; index: number }
  | { type: 'play' }
  | { type: 'pause' }
  | { type: 'toggle' }
  | { type: 'tick' }
  | { type: 'speed'; speed: Speed }
  | { type: 'reset'; total: number }

export function initPlayer(total: number): PlayerState {
  if (!Number.isInteger(total) || total < 1) throw new RangeError(`player: число шагов должно быть целым ≥ 1, получено ${total}`)
  return { index: 0, total, playing: false, speed: 1 }
}

function clampIndex(index: number, total: number): number {
  return Math.min(Math.max(Math.trunc(index), 0), total - 1)
}

function moveTo(s: PlayerState, index: number): PlayerState {
  return { ...s, index: clampIndex(index, s.total), playing: false }
}

export function playerReducer(s: PlayerState, a: PlayerAction): PlayerState {
  const isLast = s.index === s.total - 1
  switch (a.type) {
    case 'next':
      return moveTo(s, s.index + 1)
    case 'prev':
      return moveTo(s, s.index - 1)
    case 'first':
      return moveTo(s, 0)
    case 'last':
      return moveTo(s, s.total - 1)
    case 'seek':
      return moveTo(s, a.index)
    case 'play':
      return isLast ? { ...s, index: 0, playing: true } : { ...s, playing: true }
    case 'pause':
      return { ...s, playing: false }
    case 'toggle':
      return playerReducer(s, { type: s.playing ? 'pause' : 'play' })
    case 'tick': {
      if (!s.playing) return s
      if (isLast) return { ...s, playing: false }
      const index = s.index + 1
      return { ...s, index, playing: index < s.total - 1 }
    }
    case 'speed':
      return { ...s, speed: a.speed }
    case 'reset':
      return initPlayer(a.total)
  }
}

export function stepDelayMs(speed: Speed): number {
  return BASE_STEP_MS / speed
}

const KEY_ACTIONS: Record<string, PlayerAction> = {
  ArrowRight: { type: 'next' },
  ArrowLeft: { type: 'prev' },
  ' ': { type: 'toggle' },
  Home: { type: 'first' },
  End: { type: 'last' },
}

const OWN_KEYS_TAGS = new Set(['INPUT', 'SELECT', 'TEXTAREA'])

export function keyToAction(key: string, target: { tagName: string } | null): PlayerAction | null {
  const action = KEY_ACTIONS[key]
  if (action === undefined) return null
  const tag = target?.tagName.toUpperCase() ?? ''
  if (OWN_KEYS_TAGS.has(tag)) return null
  if (key === ' ' && tag === 'BUTTON') return null
  return action
}
