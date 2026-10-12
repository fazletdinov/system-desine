export type FullscreenMode = 'off' | 'native' | 'overlay'

export interface FullscreenApi {
  enabled: boolean
  request(el: HTMLElement): Promise<void>
  exit(): Promise<void>
}

// Если настоящий полноэкранный режим недоступен или браузер отказал,
// показываем визуализацию оверлеем поверх страницы — визуально то же самое.
export async function enterFullscreen(el: HTMLElement, api: FullscreenApi): Promise<FullscreenMode> {
  if (!api.enabled) return 'overlay'
  try {
    await api.request(el)
    return 'native'
  } catch {
    return 'overlay'
  }
}

export async function exitFullscreen(mode: FullscreenMode, api: FullscreenApi): Promise<'off'> {
  if (mode !== 'native') return 'off'
  try {
    await api.exit()
  } catch {
    // Браузер мог уже выйти сам (Esc) — тогда exit отклоняется, а цель и так достигнута.
  }
  return 'off'
}
