import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { loadGlossaryFile } from './load'
import { GlossaryError, parseGlossary, sortTerms } from './schema'

const valid = {
  wal: {
    term: 'WAL (журнал предзаписи)',
    en: 'Write-Ahead Log',
    short: 'Журнал, куда изменение пишется до изменения данных.',
    long: 'Подробно.',
    links: [{ title: 'Клеппман, гл. 3' }, { title: 'Документация', url: 'https://www.postgresql.org/docs/current/wal-intro.html' }],
  },
  rps: { term: 'RPS', short: 'Запросов в секунду.' },
}

describe('parseGlossary', () => {
  it('разбирает корректный глоссарий и проставляет id', () => {
    const g = parseGlossary(valid)
    expect(g.wal).toEqual({ id: 'wal', ...valid.wal })
    expect(g.rps).toEqual({ id: 'rps', term: 'RPS', short: 'Запросов в секунду.', links: [] })
  })

  it.each([
    ['корень не объект', [1, 2]],
    ['id не в kebab-case', { Wal: { term: 'x', short: 'y' } }],
    ['нет term', { wal: { short: 'y' } }],
    ['пустой short', { wal: { term: 'x', short: '  ' } }],
    ['en не строка', { wal: { term: 'x', short: 'y', en: 5 } }],
    ['links не массив', { wal: { term: 'x', short: 'y', links: 'a' } }],
    ['<T> внутри long', { wal: { term: 'x', short: 'y', long: 'см. <T id="rps">RPS</T>' } }],
    ['ссылка без title', { wal: { term: 'x', short: 'y', links: [{ url: 'https://a' }] } }],
  ])('отклоняет: %s', (_name, raw) => {
    expect(() => parseGlossary(raw)).toThrow(GlossaryError)
  })
})

describe('sortTerms', () => {
  it('сортирует по полю term без учёта регистра, по правилам русской локали', () => {
    const g = parseGlossary({
      b: { term: 'балансировщик', short: '.' },
      a: { term: 'Задержка', short: '.' },
      c: { term: 'Аудит', short: '.' },
    })
    expect(sortTerms(Object.values(g)).map((t) => t.id)).toEqual(['c', 'b', 'a'])
  })
})

describe('terms.yaml', () => {
  it('реальный глоссарий проходит валидацию', () => {
    const path = fileURLToPath(new URL('./terms.yaml', import.meta.url))
    expect(Object.keys(loadGlossaryFile(path)).length).toBeGreaterThan(0)
  })
})
