import { describe, expect, it, vi } from 'vitest'
import { enterFullscreen, exitFullscreen, type FullscreenApi } from './fullscreen'

const el = {} as HTMLElement
const api = (over: Partial<FullscreenApi> = {}): FullscreenApi => ({
  enabled: true,
  request: vi.fn().mockResolvedValue(undefined),
  exit: vi.fn().mockResolvedValue(undefined),
  ...over,
})

describe('enterFullscreen', () => {
  it('настоящий полноэкранный режим, если браузер разрешил', async () => {
    expect(await enterFullscreen(el, api())).toBe('native')
  })
  it('оверлей, если API недоступен (iPhone Safari)', async () => {
    const a = api({ enabled: false })
    expect(await enterFullscreen(el, a)).toBe('overlay')
    expect(a.request).not.toHaveBeenCalled()
  })
  it('оверлей, если браузер отказал', async () => {
    expect(await enterFullscreen(el, api({ request: vi.fn().mockRejectedValue(new Error('denied')) }))).toBe('overlay')
  })
})

describe('exitFullscreen', () => {
  it('из native вызывает exit', async () => {
    const a = api()
    expect(await exitFullscreen('native', a)).toBe('off')
    expect(a.exit).toHaveBeenCalledOnce()
  })
  it('из overlay не трогает API', async () => {
    const a = api()
    expect(await exitFullscreen('overlay', a)).toBe('off')
    expect(a.exit).not.toHaveBeenCalled()
  })
  it('если браузер уже вышел сам (Esc), ошибка exit не мешает', async () => {
    expect(await exitFullscreen('native', api({ exit: vi.fn().mockRejectedValue(new TypeError('not in fullscreen')) }))).toBe('off')
  })
})
