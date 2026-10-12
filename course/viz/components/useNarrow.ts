import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue'

// SSR и первый кадр — широкая раскладка; узкая включается только на клиенте после монтирования,
// когда ResizeObserver сообщил ширину контейнера (родителя SVG).
export function useNarrow(svg: Ref<Element | undefined>, thresholdPx: number): Ref<boolean> {
  const narrow = ref(false)
  let observer: ResizeObserver | undefined
  onMounted(() => {
    const container = svg.value?.parentElement
    if (!container) return
    observer = new ResizeObserver(([entry]) => {
      if (entry) narrow.value = entry.contentRect.width < thresholdPx
    })
    observer.observe(container)
  })
  onBeforeUnmount(() => observer?.disconnect())
  return narrow
}
