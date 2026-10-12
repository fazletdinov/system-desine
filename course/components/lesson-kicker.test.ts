import { describe, expect, it } from 'vitest'
import { lessonKicker } from './lesson-kicker'

describe('lessonKicker', () => {
  it.each([
    ['modules/00-foundation/03-request-path.md', 'Модуль 0 · Урок 3'],
    ['modules/00-foundation/index.md', 'Модуль 0'],
    ['modules/00b-network/01-tcp.md', 'Модуль 0b · Урок 1'],
    ['modules/01-5-concurrency/02-gmp.md', 'Модуль 1.5 · Урок 2'],
    ['modules/13a-k8s/10-probes.md', 'Модуль 13a · Урок 10'],
    ['glossary.md', null],
    ['index.md', null],
  ])('%s → %s', (path, expected) => {
    expect(lessonKicker(path)).toBe(expected)
  })
})
