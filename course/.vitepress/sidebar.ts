import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import type { DefaultTheme } from 'vitepress'
import { parse } from 'yaml'

const LESSON_FILE = /^\d{2}-.+\.md$/
const MODULE_DIR = /^\d{2}/
const FRONTMATTER = /^---\r?\n([\s\S]*?)\r?\n---/

function readFrontmatter(markdown: string): Record<string, unknown> {
  const match = FRONTMATTER.exec(markdown)
  if (!match) return {}
  const data: unknown = parse(match[1] ?? '')
  return typeof data === 'object' && data !== null ? (data as Record<string, unknown>) : {}
}

export function readTitle(markdown: string, fallback: string): string {
  const { title } = readFrontmatter(markdown)
  return typeof title === 'string' ? title : fallback
}

// Номера вроде 0b, 1.5, 2a в имени папки не сортируются как числа, поэтому порядок задаёт `order` в index.md.
export function readOrder(markdown: string, fallback: number): number {
  const { order } = readFrontmatter(markdown)
  return typeof order === 'number' ? order : fallback
}

export function buildSidebar(modulesDir: string, base = '/modules'): DefaultTheme.SidebarItem[] {
  if (!existsSync(modulesDir)) return []
  return readdirSync(modulesDir, { withFileTypes: true })
    .filter((entry) => entry.isDirectory() && MODULE_DIR.test(entry.name))
    .map((entry) => ({ dir: entry.name, order: moduleOrder(join(modulesDir, entry.name, 'index.md'), entry.name) }))
    .sort((a, b) => a.order - b.order || a.dir.localeCompare(b.dir))
    .map(({ dir }) => buildModule(join(modulesDir, dir), `${base}/${dir}`, dir))
}

function moduleOrder(indexPath: string, dirName: string): number {
  const fromName = Number(dirName.slice(0, 2))
  return existsSync(indexPath) ? readOrder(readFileSync(indexPath, 'utf8'), fromName) : fromName
}

function buildModule(dirPath: string, link: string, dirName: string): DefaultTheme.SidebarItem {
  const indexPath = join(dirPath, 'index.md')
  const text = existsSync(indexPath) ? readTitle(readFileSync(indexPath, 'utf8'), dirName) : dirName
  const items = readdirSync(dirPath)
    .filter((file) => LESSON_FILE.test(file))
    .sort()
    .map((file) => ({
      text: readTitle(readFileSync(join(dirPath, file), 'utf8'), file),
      link: `${link}/${file.replace(/\.md$/, '')}`,
    }))
  return { text, link: `${link}/`, collapsed: true, items }
}
