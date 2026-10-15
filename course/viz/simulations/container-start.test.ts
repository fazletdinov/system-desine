import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from '../../glossary/load'
import { captionTermIds, captionText } from '../core/caption'
import { START_PHASES, containerStartScenario } from './container-start'

const steps = containerStartScenario()
const indexOf = (phase: string) => steps.findIndex((s) => s.state.phase === phase)

describe('containerStartScenario', () => {
  it('девять кадров, фазы идут в заданном порядке', () => {
    expect(steps).toHaveLength(9)
    expect(steps.map((s) => s.state.phase)).toEqual([...START_PHASES])
  })

  it('у каждого кадра есть подпись', () => {
    for (const s of steps) expect(captionText(s.caption).trim().length).toBeGreaterThan(20)
  })

  it('после child у процесса два PID: хостовый не 1, внутри namespace 1', () => {
    for (const s of steps.slice(indexOf('child'))) {
      const node = s.state.host.find((p) => p.nsPid !== undefined)
      expect(node).toBeDefined()
      expect(node?.pid).not.toBe(1)
      expect(node?.nsPid).toBe(1)
    }
  })

  it('до child внутри контейнера процессов нет', () => {
    for (const s of steps.slice(0, indexOf('child'))) expect(s.state.container).toBeNull()
  })

  it('старый корень виден только между pivot-root и umount-oldroot', () => {
    for (const s of steps) {
      const between = indexOf('pivot-root') <= steps.indexOf(s) && steps.indexOf(s) < indexOf('umount-oldroot')
      expect(s.state.mounts.oldroot).toBe(between)
    }
  })

  it('hostname контейнера появляется на фазе hostname, хостовый не меняется', () => {
    const at = indexOf('hostname')
    steps.forEach((s, i) => expect(s.state.hostname.container === null).toBe(i < at))
    expect(new Set(steps.map((s) => s.state.hostname.host)).size).toBe(1)
  })

  it('/proc монтируется на фазе mount-proc, не раньше', () => {
    steps.forEach((s, i) => expect(s.state.mounts.proc).toBe(i >= indexOf('mount-proc')))
  })

  it('после exec PID 1 - sh, и на хосте тот же PID показывает sh', () => {
    const last = steps[steps.length - 1]!.state
    expect(last.container?.[0]).toMatchObject({ pid: 1, cmd: 'sh' })
    expect(last.host.find((p) => p.nsPid === 1)?.cmd).toBe('sh')
  })
})

it('в подписях есть термины namespace, pivot-root, rootfs, pid-1 и все они из глоссария', () => {
  const ids = steps.flatMap((s) => captionTermIds(s.caption))
  for (const id of ['namespace', 'pivot-root', 'rootfs', 'pid-1']) expect(ids).toContain(id)
  const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
  for (const id of ids) expect(known.has(id)).toBe(true)
})
