import { runInNewContext } from 'node:vm'
import type { ShotMark } from '../components/shot.ts'

export interface ShotRef {
  line: number
  src: string
  width: number
  height: number
  marks: ShotMark[]
}

export interface ShotScan {
  refs: ShotRef[]
  unparseable: { line: number; text: string }[]
}

const TAG = /<Shot\b([\s\S]*?)\/>/g
const ATTR = /(:?)([a-z]+)="([^"]*)"/g

// Значение привязки (:width="1400", :marks="[…]") — выражение JS из самого урока;
// считаем его в пустом контексте, без доступа к Node.
function evaluate(expr: string): unknown {
  return runInNewContext(`(${expr})`, Object.create(null), { timeout: 1000 })
}

// Теги <Shot …/> урока. Строка — номер строки, где тег начинается (с 1).
export function scanShots(md: string): ShotScan {
  const scan: ShotScan = { refs: [], unparseable: [] }
  for (const m of md.matchAll(TAG)) {
    const line = md.slice(0, m.index).split('\n').length
    const text = m[0].split('\n')[0] ?? ''
    try {
      const attrs = new Map<string, unknown>()
      for (const a of (m[1] ?? '').matchAll(ATTR)) attrs.set(a[2] ?? '', a[1] === ':' ? evaluate(a[3] ?? '') : a[3])
      const { src, width, height, marks } = Object.fromEntries(attrs)
      if (typeof src !== 'string' || typeof width !== 'number' || typeof height !== 'number' || !Array.isArray(marks)) throw new Error('нет src, :width, :height или :marks')
      scan.refs.push({ line, src, width, height, marks: marks as ShotMark[] })
    } catch {
      scan.unparseable.push({ line, text })
    }
  }
  return scan
}

// Размеры WebP из заголовка: VP8 (с потерями), VP8L (без потерь) или VP8X (расширенный).
// null — если это не WebP.
export function webpSize(buf: Buffer): { width: number; height: number } | null {
  if (buf.length < 30 || buf.toString('ascii', 0, 4) !== 'RIFF' || buf.toString('ascii', 8, 12) !== 'WEBP') return null
  const chunk = buf.toString('ascii', 12, 16)
  if (chunk === 'VP8X') return { width: 1 + buf.readUIntLE(24, 3), height: 1 + buf.readUIntLE(27, 3) }
  if (chunk === 'VP8 ') return { width: buf.readUInt16LE(26) & 0x3fff, height: buf.readUInt16LE(28) & 0x3fff }
  if (chunk === 'VP8L') {
    const bits = buf.readUInt32LE(21)
    return { width: 1 + (bits & 0x3fff), height: 1 + ((bits >> 14) & 0x3fff) }
  }
  return null
}
