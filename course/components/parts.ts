// Метки частей урока. Цвета — в parts.css, по одному набору на тему.
export const PARTS = {
  problem: 'Проблема',
  theory: 'Теория',
  viz: 'Визуализация',
  mistakes: 'Ошибки',
  tradeoff: 'Компромисс',
  code: 'Код',
  selfcheck: 'Самопроверка',
  sources: 'Источники',
  deep: 'Под капотом',
  practice: 'Практика',
} as const

export type PartKind = keyof typeof PARTS
