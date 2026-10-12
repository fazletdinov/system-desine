import type { Caption } from './caption'

// Один кадр визуализации. state — полный снимок, а не дельта:
// так переход назад и прыжок на любой шаг не требуют пересчёта.
export interface Step<S> {
  state: S
  caption: Caption
  highlight?: readonly string[]
  codeLine?: number
}

export type Scenario<S> = () => readonly Step<S>[]

// Песочница: действие пользователя даёт новое состояние, состояние — шаги для Player.
// F — состояние кадра, если кадры отличаются от состояния песочницы (например, TCP: настройки → кадры по тикам).
export interface Simulation<S, A, F = S> {
  initial: () => S
  apply: (state: S, action: A) => S
  steps: (state: S) => readonly Step<F>[]
}
