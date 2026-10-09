import { readFileSync } from 'node:fs'
import { parse } from 'yaml'
import { parseGlossary, type Glossary } from './schema'

export function loadGlossaryFile(path: string): Glossary {
  return parseGlossary(parse(readFileSync(path, 'utf8')))
}
