// Строка над заголовком урока: «Модуль 0 · Урок 3». Номера берутся из пути файла,
// поэтому уроки ничего не указывают сами. Папки модулей: 00-…, 00b-…, 01-5-… (= 1.5), 13a-….
const MODULE_DIR = /^modules\/(\d{2})([a-z])?(?:-(\d))?-[^/]+\/(.+)$/
const LESSON_FILE = /^(\d{2})-.+\.md$/

export function lessonKicker(relativePath: string): string | null {
  const m = MODULE_DIR.exec(relativePath)
  if (m === null) return null
  const [, major, letter, minor, file] = m
  const module = `${Number(major)}${minor === undefined ? '' : `.${minor}`}${letter ?? ''}`
  if (file === 'index.md') return `Модуль ${module}`
  const lesson = LESSON_FILE.exec(file ?? '')
  return lesson === null ? null : `Модуль ${module} · Урок ${Number(lesson[1])}`
}
