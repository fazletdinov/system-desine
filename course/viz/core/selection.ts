// Рендерер помечает кликабельное `data-explain`; клик может прийти во вложенный
// узел (текст, <span>), поэтому поднимаемся к ближайшей метке, а не читаем target.
export function explainIdFromEvent(e: Event): string | null {
  const target = e.target as { closest?: (selector: string) => Element | null } | null
  if (typeof target?.closest !== 'function') return null
  return target.closest('[data-explain]')?.getAttribute('data-explain') ?? null
}
