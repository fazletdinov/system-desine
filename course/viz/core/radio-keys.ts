// Стрелки в радиогруппе (WAI-ARIA): ←/↑ — назад, →/↓ — вперёд, по кругу.
// Возвращает индекс новой кнопки или -1, если клавиша не стрелка.
// С Alt/Ctrl/Meta стрелки принадлежат браузеру (Alt+← — «назад»): их не перехватываем.
export interface KeyModifiers {
  altKey?: boolean
  ctrlKey?: boolean
  metaKey?: boolean
}

export function nextRadioIndex(current: number, count: number, key: string, modifiers: KeyModifiers = {}): number {
  if (count <= 0) return -1
  if (modifiers.altKey || modifiers.ctrlKey || modifiers.metaKey) return -1
  if (key === 'ArrowRight' || key === 'ArrowDown') return (current + 1) % count
  if (key === 'ArrowLeft' || key === 'ArrowUp') return (current + count - 1) % count
  return -1
}
