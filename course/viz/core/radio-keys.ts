// Стрелки в радиогруппе (WAI-ARIA): ←/↑ — назад, →/↓ — вперёд, по кругу.
// Возвращает индекс новой кнопки или -1, если клавиша не стрелка.
export function nextRadioIndex(current: number, count: number, key: string): number {
  if (count <= 0) return -1
  if (key === 'ArrowRight' || key === 'ArrowDown') return (current + 1) % count
  if (key === 'ArrowLeft' || key === 'ArrowUp') return (current + count - 1) % count
  return -1
}
