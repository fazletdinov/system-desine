// Нижняя граница масштаба схемы mermaid. Без неё широкая схема (ER-диаграмма урока 2.1 —
// 2000 px) ужимается до ширины колонки: на 375 px это 15 % и текст в 2 px. При 0,6 шрифт
// mermaid (14–16 px) остаётся не меньше 8–10 px, а то, что не влезло, прокручивается внутри
// рамки, а не всей страницей.
export const MIN_DIAGRAM_SCALE = 0.6

// Минимальная ширина SVG в пикселях по ширине его viewBox, или null, если ширины нет.
// Округление вниз: вверх схема ровно по колонке получала бы прокрутку в 1 px.
export function diagramMinWidth(viewBoxWidth: number): number | null {
  if (!Number.isFinite(viewBoxWidth) || viewBoxWidth <= 0) return null
  return Math.floor(viewBoxWidth * MIN_DIAGRAM_SCALE)
}
