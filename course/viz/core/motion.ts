// Вызывать только на клиенте (onMounted, обработчики событий): при SSR window нет.
export function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}
