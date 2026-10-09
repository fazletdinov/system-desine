export interface TermLink {
  title: string
  url?: string
}

export interface Term {
  id: string
  term: string
  en?: string
  short: string
  long?: string
  links: TermLink[]
}

export type Glossary = Record<string, Term>

export class GlossaryError extends Error {}

const ID = /^[a-z0-9]+(-[a-z0-9]+)*$/
// long вставляется через v-html: компоненты Vue внутри него не работают.
const T_TAG = /<T[\s>]/

type Dict = Record<string, unknown>

function isDict(value: unknown): value is Dict {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function requireString(id: string, obj: Dict, field: string): string {
  const value = obj[field]
  if (typeof value !== 'string' || value.trim() === '') {
    throw new GlossaryError(`glossary: у термина "${id}" нет обязательного поля "${field}"`)
  }
  return value
}

function optionalString(id: string, obj: Dict, field: string): string | undefined {
  const value = obj[field]
  if (value === undefined) return undefined
  if (typeof value !== 'string') throw new GlossaryError(`glossary: поле "${field}" у "${id}" должно быть строкой`)
  return value
}

function parseLinks(id: string, raw: unknown): TermLink[] {
  if (raw === undefined) return []
  if (!Array.isArray(raw)) throw new GlossaryError(`glossary: links у "${id}" должен быть списком`)
  return raw.map((link) => {
    if (!isDict(link)) throw new GlossaryError(`glossary: ссылка у "${id}" должна быть объектом`)
    const title = requireString(id, link, 'title')
    const url = optionalString(id, link, 'url')
    return url === undefined ? { title } : { title, url }
  })
}

function parseTerm(id: string, raw: unknown): Term {
  if (!ID.test(id)) throw new GlossaryError(`glossary: id "${id}" должен быть в kebab-case латиницей`)
  if (!isDict(raw)) throw new GlossaryError(`glossary: термин "${id}" должен быть объектом`)
  const term: Term = {
    id,
    term: requireString(id, raw, 'term'),
    short: requireString(id, raw, 'short'),
    links: parseLinks(id, raw.links),
  }
  const en = optionalString(id, raw, 'en')
  const long = optionalString(id, raw, 'long')
  if (en !== undefined) term.en = en
  if (long !== undefined) {
    if (T_TAG.test(long)) {
      throw new GlossaryError(`glossary: в long у "${id}" нельзя использовать <T>; используйте ссылку [текст](#id)`)
    }
    term.long = long
  }
  return term
}

export function parseGlossary(raw: unknown): Glossary {
  if (!isDict(raw)) throw new GlossaryError('glossary: корень YAML должен быть объектом «id → термин»')
  const result: Glossary = {}
  for (const [id, value] of Object.entries(raw)) result[id] = parseTerm(id, value)
  return result
}

const collator = new Intl.Collator('ru', { sensitivity: 'base' })

export function sortTerms<T extends Term>(terms: readonly T[]): T[] {
  return [...terms].sort((a, b) => collator.compare(a.term, b.term))
}
