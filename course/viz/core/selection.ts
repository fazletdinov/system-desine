// Рендерер помечает кликабельное `data-explain`; клик может прийти во вложенный
// узел (текст, <span>), поэтому поднимаемся к ближайшей метке, а не читаем target.
export function explainIdFromEvent(e: Event): string | null {
  const target = e.target as { closest?: (selector: string) => Element | null } | null
  if (typeof target?.closest !== 'function') return null
  return target.closest('[data-explain]')?.getAttribute('data-explain') ?? null
}

// Клик мимо размеченных элементов (по фону, отступу, подписи) выбор не сбрасывает:
// иначе панель гасла бы от случайного клика между строками.
export function pickExplainId(e: Event, current: string | null): string | null {
  return explainIdFromEvent(e) ?? current
}

// Enter и пробел выбирают только сам размеченный элемент (role="button"): если фокус
// на вложенной ссылке-термине, Enter должен открыть глоссарий, а не перехватиться строкой.
export function isExplainActivationKey(e: KeyboardEvent): boolean {
  return (e.key === 'Enter' || e.key === ' ') && e.target === e.currentTarget
}
