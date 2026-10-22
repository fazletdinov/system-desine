// Логика строки «Запустить самому» отдельно от Vue: её проверяют тесты без DOM.

// Каталог — от корня репозитория, поэтому строка начинается с cd: её можно вставить
// в терминал, открытый в корне, и она сработает без правок.
export function runLine(dir: string, cmd: string): string {
  return `cd ${dir} && ${cmd}`
}

export interface InlinePart {
  code: boolean
  text: string
}

// Подготовка пишется как обычный текст урока: команды и пути — в `обратных кавычках`.
// Непарная кавычка остаётся текстом, чтобы опечатка не съела конец строки.
export function inlineCode(text: string): InlinePart[] {
  const parts: InlinePart[] = []
  let rest = text
  while (rest.length > 0) {
    const open = rest.indexOf('`')
    const close = open === -1 ? -1 : rest.indexOf('`', open + 1)
    if (close === -1) {
      parts.push({ code: false, text: rest })
      break
    }
    if (open > 0) parts.push({ code: false, text: rest.slice(0, open) })
    parts.push({ code: true, text: rest.slice(open + 1, close) })
    rest = rest.slice(close + 1)
  }
  return parts
}
