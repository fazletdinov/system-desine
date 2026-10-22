// Логика строки «Запустить самому» отдельно от Vue: её проверяют тесты без DOM.

// Состояние урока по тегу живёт в отдельном рабочем дереве рядом с репозиторием, а не
// через git checkout: иначе вместе с кодом откатился бы и сам курс, открытый из этой копии.
export function worktreePath(tag: string): string {
  return `../sd-${tag}`
}

export function worktreeAdd(tag: string): string {
  return `git worktree add ${worktreePath(tag)} ${tag}`
}

// --force: после прогона в дереве остаются ключ, профили и бинари, а само дерево одноразовое.
export function worktreeRemove(tag: string): string {
  return `git worktree remove --force ${worktreePath(tag)}`
}

// Каталог, из которого выполняются и подготовка, и команда: dir в репозитории
// или тот же dir в рабочем дереве тега.
export function runDir(dir: string, tag?: string): string {
  return tag === undefined ? dir : `${worktreePath(tag)}/${dir}`
}

// Строка начинается с cd от корня репозитория: её можно вставить в терминал,
// открытый в корне, и она сработает без правок.
export function runLine(dir: string, cmd: string, tag?: string): string {
  return `cd ${runDir(dir, tag)} && ${cmd}`
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
