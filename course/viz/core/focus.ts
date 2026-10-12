// Удержание фокуса в оверлее: Tab с последнего элемента ведёт на первый, Shift+Tab с первого — на последний.
// current = -1, если фокус сейчас вне списка.
export function nextFocusIndex(current: number, count: number, backwards: boolean): number {
  if (count <= 0) return -1
  if (current < 0) return backwards ? count - 1 : 0
  return (current + (backwards ? count - 1 : 1)) % count
}
