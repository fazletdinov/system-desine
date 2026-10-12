// Один кадр визуализации. state — полный снимок, а не дельта:
// так переход назад и прыжок на любой шаг не требуют пересчёта.
export interface Step<S> {
  state: S
  caption: string
  highlight?: readonly string[]
  codeLine?: number
}

export type Scenario<S> = () => readonly Step<S>[]
