import { execFileSync } from 'node:child_process'
import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

// Собирает курс под подпутём, как на GitHub Pages, и ищет ссылки мимо него: такой
// href работает локально и ведёт в 404 на Pages.
const COURSE = fileURLToPath(new URL('..', import.meta.url))
const BASE = '/check-base/'
const DIST = join(COURSE, '.vitepress', 'dist')

execFileSync('npx', ['vitepress', 'build'], { cwd: COURSE, env: { ...process.env, COURSE_BASE: BASE }, stdio: 'inherit', timeout: 600_000 })

function files(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) return files(p)
    // Только HTML и CSS: JS-чанки страниц хранят текст урока строками, и пример
    // `href="/glossary"` из урока выглядел бы в них как настоящая ссылка. Ссылки,
    // которые строит JS, попадают в отрисованный HTML — его и проверяем.
    return /\.(html|css)$/.test(e.name) ? [p] : []
  })
}

// Абсолютный путь без базы: href="/x", src="/x", url(/x). Протокол-относительные
// (//cdn) и якоря не считаются.
const BAD = /(?:href|src)=["']\/(?!\/|check-base\/)|url\(\s*["']?\/(?!\/|check-base\/)/g
const problems: string[] = []
for (const f of files(DIST)) {
  const text = readFileSync(f, 'utf8')
  for (const m of text.matchAll(BAD)) {
    const at = m.index ?? 0
    problems.push(`${relative(DIST, f)}: …${text.slice(Math.max(0, at - 40), at + 60)}…`)
  }
}
if (problems.length > 0) {
  console.error(`Ссылки мимо базового пути ${BASE}:\n${problems.slice(0, 50).join('\n')}`)
  process.exit(1)
}
console.log(`Курс под ${BASE}: ссылок мимо базового пути нет.`)
