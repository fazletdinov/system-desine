import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import type { DefaultTheme } from 'vitepress'
import { parse } from 'yaml'

const LESSON_FILE = /^\d{2}-.+\.md$/
const FRONTMATTER = /^---\r?\n([\s\S]*?)\r?\n---/

export function readTitle(markdown: string, fallback: string): string {
  const match = FRONTMATTER.exec(markdown)
  if (!match) return fallback
  const data: unknown = parse(match[1] ?? '')
  if (typeof data === 'object' && data !== null && 'title' in data && typeof data.title === 'string') {
    return data.title
  }
  return fallback
}

export function buildSidebar(modulesDir: string, base = '/modules'): DefaultTheme.SidebarItem[] {
  if (!existsSync(modulesDir)) return []
  return readdirSync(modulesDir, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort()
    .map((dir) => buildModule(join(modulesDir, dir), `${base}/${dir}`, dir))
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
  return { text, link: `${link}/`, collapsed: false, items }
}
