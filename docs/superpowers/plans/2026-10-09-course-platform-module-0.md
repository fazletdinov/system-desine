# Платформа курса + движок визуализаций + модуль 0 — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Работающий локально сайт курса на VitePress: глоссарий с подсказками, движок пошаговых визуализаций, три визуализации и шесть уроков модуля 0.

**Architecture:** VitePress-сайт в `course/`. Логика разделена на две части:
- чистые TS-модули без Vue — сайдбар, глоссарий, редьюсер плеера, сценарии, расчёты; покрыты Vitest;
- тонкие Vue-компоненты, которые только рисуют состояние.

Визуализации устроены по схеме «сценарий → `Step[]` → `Player` → SVG-рендерер». Фрагменты Go-кода подсвечивает сам VitePress (Shiki), а `CodePanel` лишь выделяет активную строку.

**Tech Stack:** VitePress 1.6.4, Vue 3.5.43, TypeScript 5.9.3, vue-tsc 3.3.12, Vitest 3.2.7, GSAP 3.15.0, Mermaid 11.17.2 + vitepress-plugin-mermaid 2.0.17, yaml 2.9.1, @types/node 24.19.1. Node 26, npm.

**Spec:** `docs/superpowers/specs/2026-10-09-course-platform-design.md` — читать целиком перед началом, особенно разделы 1 («Голос автора»), 2.2 и 2.3 (стандарты подробности и визуализации).

## Global Constraints

- Все команды npm выполняются из `/home/idel/DevGo/system-desine/course`.
- Версии пакетов — ровно те, что указаны в `package.json` из задачи 1, без `^`.
- Язык интерфейса и контента — русский. Английский оригинал термина указывается в скобках при первом упоминании.
- Каждый термин в уроке при первом появлении оборачивается в `<T id="...">`. Неизвестный `id` — ошибка сборки.
- Чистые модули (`*.ts` вне `.vue`) не импортируют `vue` и не обращаются к `window`/`document` на уровне модуля. Всё, что трогает DOM, выполняется только на клиенте (`onMounted`/обработчики): сайт рендерится через SSR.
- Цвета визуализаций — только через CSS-переменные `--viz-*` из `course/viz/palette.css`. Должны быть определены для светлой и тёмной темы (`.dark`).
- Анимации отключаются при `prefers-reduced-motion: reduce`.
- Вёрстка работает на ширине 375 px без горизонтальной прокрутки страницы.
- Тексты уроков пишутся по разделу 1 спецификации («Голос автора») и по чек-листам 2.2/2.3.
- Числа в уроках — со ссылкой на источник; оценочные значения явно помечены как порядок величины.
- Коммиты заканчиваются строкой `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Ширина телефона (375 px).** Ожидается: текст в SVG читаем, подсказка термина не вылезает за экран, панель управления плеера переносится, нет горизонтальной прокрутки. Покрывает задача 9, шаг 3 (Playwright-скриншоты) плюс CSS в задачах 3, 6, 7, 8.
2. **Клавиатура внутри элементов управления.** Ожидается:
   - пробел на кнопке плеера нажимает кнопку, а не переключает воспроизведение дважды;
   - стрелки на ползунке двигают ползунок, а не листают шаги дважды.

   Покрывает `keyToAction` с тестами в задаче 5.
3. **Мусор в калькуляторе.** Пустое поле, отрицательное число, `NaN`, коэффициент пика меньше 1. Ожидается: нули или минимально допустимые значения, никаких `NaN`/`Infinity` на экране. Покрывает `sanitizeInput` с тестами в задаче 8.
4. **Уход со страницы во время автопроигрывания и быстрые клики во время анимации.** Ожидается: таймер останавливается при размонтировании; новая анимация пакета прерывает предыдущую, а не накладывается на неё. Покрывают `onBeforeUnmount` в `Player.vue` и `gsap.killTweensOf` в рендерере (задача 6), проверка — задача 9.
5. **Тёмная тема.** Ожидается: узлы, пакеты, полосы и подсветка строки кода контрастны на тёмном фоне. Покрывает `.dark` в `palette.css` (задача 6), проверка скриншотом — задача 9.

---

## Структура файлов

```
course/
├── package.json                    # зависимости и скрипты
├── tsconfig.json
├── vitest.config.ts
├── index.md                        # главная страница
├── glossary.md                     # страница глоссария
├── .vitepress/
│   ├── config.ts                   # конфиг сайта
│   ├── sidebar.ts                  # генерация сайдбара из modules/
│   ├── sidebar.test.ts
│   ├── terms-check.ts              # markdown-it-правило: неизвестный <T id> → ошибка
│   ├── terms-check.test.ts
│   └── theme/index.ts              # расширение темы, глобальные компоненты
├── glossary/
│   ├── terms.yaml                  # единственный источник определений
│   ├── schema.ts                   # типы + разбор и валидация + сортировка
│   ├── schema.test.ts
│   ├── load.ts                     # чтение YAML с диска
│   └── terms.data.ts               # data loader VitePress (+ long → HTML)
├── components/
│   ├── T.vue                       # термин с подсказкой
│   ├── GlossaryList.vue            # список терминов
│   ├── Callout.vue                 # 💡 ⚠️ ⚖️ 📚
│   └── SelfCheck.vue               # вопрос + скрытый ответ
├── viz/
│   ├── README.md                   # палитра и легенда (исключён из сайта)
│   ├── palette.css                 # --viz-* для светлой и тёмной темы
│   ├── core/
│   │   ├── types.ts                # Step<S>, Scenario<S>
│   │   ├── player.ts               # редьюсер плеера, keyToAction
│   │   ├── player.test.ts
│   │   ├── format.ts               # форматирование чисел, байтов, времени
│   │   ├── format.test.ts
│   │   └── motion.ts               # prefersReducedMotion
│   ├── components/
│   │   ├── Player.vue
│   │   └── CodePanel.vue
│   ├── simulations/
│   │   ├── request-path.ts (+ .test.ts)
│   │   ├── latency.ts (+ .test.ts)
│   │   └── estimation.ts (+ .test.ts)
│   ├── renderers/
│   │   └── RequestPathView.vue
│   └── scenes/                     # то, что вставляется в уроки
│       ├── RequestPath.vue
│       ├── LatencyScale.vue
│       └── EstimationCalculator.vue
└── modules/00-foundation/
    ├── index.md
    ├── 01-how-to-study.md
    ├── 02-what-is-system-design.md
    ├── 03-request-path.md
    ├── 04-latency.md
    ├── 05-estimation.md
    └── 06-messenger-requirements.md
```

**Два осознанных отступления от раздела 4.2 спецификации:**
- Тип `Simulation<S, A>` не вводится, пока нет потребителя. Первая песочница на шагах появится в модуле 2.5. Калькулятор — песочница без шагов плеера.
- `SelfCheck` — Vue-компонент с кнопкой. Он даёт паузу «сначала подумайте», которой нет у встроенного `::: details`.

---

### Task 1: Каркас VitePress

**Files:**
- Create: `course/package.json`, `course/tsconfig.json`, `course/vitest.config.ts`, `course/.vitepress/config.ts`, `course/.vitepress/theme/index.ts`, `course/index.md`

**Interfaces:**
- Produces: скрипты `npm run dev|build|test|typecheck`; файл `.vitepress/config.ts`, который расширяют задачи 2–4; объект темы в `.vitepress/theme/index.ts`, куда задачи 3–4 добавляют компоненты.

- [ ] **Step 1: Создать `course/package.json`**

```json
{
  "name": "system-design-course",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vitepress dev",
    "build": "vitepress build",
    "preview": "vitepress preview",
    "test": "vitest run --passWithNoTests",
    "typecheck": "vue-tsc --noEmit"
  },
  "devDependencies": {
    "@types/node": "24.19.1",
    "gsap": "3.15.0",
    "mermaid": "11.17.2",
    "typescript": "5.9.3",
    "vitepress": "1.6.4",
    "vitepress-plugin-mermaid": "2.0.17",
    "vitest": "3.2.7",
    "vue": "3.5.43",
    "vue-tsc": "3.3.12",
    "yaml": "2.9.1"
  }
}
```

- [ ] **Step 2: Создать `course/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noEmit": true,
    "jsx": "preserve",
    "resolveJsonModule": true,
    "skipLibCheck": true,
    "types": ["node", "vite/client"]
  },
  "include": [
    ".vitepress/**/*.ts",
    "glossary/**/*.ts",
    "viz/**/*.ts",
    "viz/**/*.vue",
    "components/**/*.vue"
  ]
}
```

- [ ] **Step 3: Создать `course/vitest.config.ts`**

Пути с точкой (`.vitepress`) перечислены явно: глоб `**` их не находит.

```ts
import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    include: ['.vitepress/**/*.test.ts', 'glossary/**/*.test.ts', 'viz/**/*.test.ts'],
    environment: 'node',
  },
})
```

- [ ] **Step 4: Создать `course/.vitepress/config.ts`**

```ts
import { defineConfig } from 'vitepress'

export default defineConfig({
  lang: 'ru-RU',
  title: 'System Design на практике',
  description: 'Курс по проектированию высоконагруженных систем на Go: пошаговые визуализации и сквозной проект-мессенджер',
  cleanUrls: true,
  srcExclude: ['**/README.md'],
  themeConfig: {
    nav: [{ text: 'Курс', link: '/' }],
    outline: { level: [2, 3], label: 'На этой странице' },
    docFooter: { prev: 'Предыдущий урок', next: 'Следующий урок' },
    darkModeSwitchLabel: 'Тема',
    lightModeSwitchTitle: 'Светлая тема',
    darkModeSwitchTitle: 'Тёмная тема',
    sidebarMenuLabel: 'Меню',
    returnToTopLabel: 'Наверх',
    notFound: { title: 'Страница не найдена', quote: 'Такого урока нет. Проверьте адрес или вернитесь на главную.', linkText: 'На главную' },
    search: {
      provider: 'local',
      options: {
        translations: {
          button: { buttonText: 'Поиск', buttonAriaLabel: 'Поиск' },
          modal: {
            noResultsText: 'Ничего не найдено по запросу',
            resetButtonTitle: 'Сбросить',
            footer: { selectText: 'выбрать', navigateText: 'перейти', closeText: 'закрыть' },
          },
        },
      },
    },
  },
})
```

- [ ] **Step 5: Создать `course/.vitepress/theme/index.ts`**

```ts
import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'

export default {
  extends: DefaultTheme,
} satisfies Theme
```

- [ ] **Step 6: Создать `course/index.md`**

Ссылка на модуль 0 появится в задаче 2, когда модуль будет существовать. Иначе сборка упадёт на битой ссылке.

```md
---
layout: home
hero:
  name: System Design на практике
  text: От монолита до распределённой системы
  tagline: Курс по проектированию высоконагруженных систем на Go — с пошаговыми визуализациями и сквозным проектом-мессенджером
features:
  - icon: 🧠
    title: Понимание, а не заучивание
    details: Каждый механизм объясняется от проблемы к решению, каждый термин — при первом появлении.
  - icon: ▶️
    title: Визуализации по шагам
    details: Смотрите, как запрос идёт по системе, как работает балансировщик или как Kafka делит партиции, шаг за шагом.
  - icon: 💬
    title: Мессенджер на Go
    details: Сквозной проект растёт вместе с курсом — от одного сервиса до распределённой системы в Kubernetes.
---
```

- [ ] **Step 7: Установить зависимости и проверить**

Run: `cd /home/idel/DevGo/system-desine/course && npm install && npm run build && npm test && npm run typecheck`

Expected: `npm install` без ошибок разрешения зависимостей; `build complete`; Vitest: `No test files found, exiting with code 0`; `vue-tsc` без вывода.

- [ ] **Step 8: Commit**

```bash
cd /home/idel/DevGo/system-desine
git add course/package.json course/package-lock.json course/tsconfig.json course/vitest.config.ts course/.vitepress course/index.md
git commit -m "feat(course): каркас VitePress

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Сайдбар из папки `modules/`

**Files:**
- Create: `course/.vitepress/sidebar.ts`, `course/.vitepress/sidebar.test.ts`, `course/modules/00-foundation/index.md`
- Modify: `course/.vitepress/config.ts`, `course/index.md`

**Interfaces:**
- Consumes: `config.ts` из задачи 1.
- Produces:
  - `readTitle(markdown: string, fallback: string): string`;
  - `buildSidebar(modulesDir: string, base?: string): DefaultTheme.SidebarItem[]`.

  Правило именования: модуль — папка `NN-slug/` с `index.md` (frontmatter `title`), урок — файл `NN-slug.md` с frontmatter `title`.

- [ ] **Step 1: Написать падающие тесты `course/.vitepress/sidebar.test.ts`**

```ts
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { buildSidebar, readTitle } from './sidebar'

describe('readTitle', () => {
  it.each([
    ['frontmatter с title', '---\ntitle: Урок 1\n---\n# x', 'Урок 1'],
    ['frontmatter без title', '---\nlayout: doc\n---\n# x', 'fallback'],
    ['без frontmatter', '# Заголовок', 'fallback'],
    ['CRLF-переводы строк', '---\r\ntitle: Win\r\n---\r\n', 'Win'],
    ['title не строка', '---\ntitle: 42\n---\n', 'fallback'],
  ])('%s', (_name, md, expected) => {
    expect(readTitle(md, 'fallback')).toBe(expected)
  })
})

function makeTree(): string {
  const root = mkdtempSync(join(tmpdir(), 'sidebar-'))
  const m1 = join(root, '01-go')
  const m0 = join(root, '00-foundation')
  mkdirSync(m1)
  mkdirSync(m0)
  writeFileSync(join(m0, 'index.md'), '---\ntitle: Модуль 0\n---\n')
  writeFileSync(join(m0, '02-b.md'), '---\ntitle: Урок B\n---\n')
  writeFileSync(join(m0, '01-a.md'), '---\ntitle: Урок A\n---\n')
  writeFileSync(join(m0, 'notes.md'), '# не урок\n')
  writeFileSync(join(m1, '01-x.md'), '# без title\n')
  return root
}

describe('buildSidebar', () => {
  it('сортирует модули и уроки по номеру, игнорирует файлы без номера', () => {
    const items = buildSidebar(makeTree())
    expect(items).toEqual([
      {
        text: 'Модуль 0',
        link: '/modules/00-foundation/',
        collapsed: false,
        items: [
          { text: 'Урок A', link: '/modules/00-foundation/01-a' },
          { text: 'Урок B', link: '/modules/00-foundation/02-b' },
        ],
      },
      {
        text: '01-go',
        link: '/modules/01-go/',
        collapsed: false,
        items: [{ text: '01-x.md', link: '/modules/01-go/01-x' }],
      },
    ])
  })

  it('возвращает пустой список, если папки нет', () => {
    expect(buildSidebar('/nonexistent/path/xyz')).toEqual([])
  })
})
```

- [ ] **Step 2: Запустить и убедиться, что тесты падают**

Run: `npm test -- .vitepress/sidebar.test.ts`
Expected: FAIL — `Failed to resolve import "./sidebar"`.

- [ ] **Step 3: Реализовать `course/.vitepress/sidebar.ts`**

```ts
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
```

- [ ] **Step 4: Запустить тесты**

Run: `npm test -- .vitepress/sidebar.test.ts`
Expected: PASS (7 тестов).

- [ ] **Step 5: Подключить сайдбар в `config.ts`**

В начало файла добавить:

```ts
import { fileURLToPath } from 'node:url'
import { buildSidebar } from './sidebar'

const MODULES_DIR = fileURLToPath(new URL('../modules', import.meta.url))
```

В `themeConfig` заменить `nav` и добавить `sidebar`:

```ts
    nav: [{ text: 'Курс', link: '/modules/00-foundation/' }],
    sidebar: buildSidebar(MODULES_DIR),
```

- [ ] **Step 6: Создать `course/modules/00-foundation/index.md`**

```md
---
title: Модуль 0. Фундамент
---

# Модуль 0. Фундамент

Прежде чем проектировать мессенджер на миллионы пользователей, нужно договориться о языке и научиться считать.

В этом модуле вы:

- узнаете, как устроен курс и как учиться так, чтобы знания оставались надолго;
- поймёте, что такое system design и чем функциональные требования отличаются от нефункциональных;
- пройдёте по шагам путь одного запроса — от нажатия кнопки до ответа сервера;
- запомните порядки задержек, которые должен знать каждый бэкенд-разработчик;
- научитесь оценивать нагрузку «на салфетке» и посчитаете её для нашего мессенджера.

Начните с первого урока в меню слева.
```

- [ ] **Step 7: Добавить на главную кнопку начала курса**

В `course/index.md` во frontmatter внутрь `hero:` после `tagline:` добавить:

```yaml
  actions:
    - theme: brand
      text: Начать с модуля 0
      link: /modules/00-foundation/
```

- [ ] **Step 8: Проверить сборку**

Run: `npm run build && npm run typecheck`
Expected: `build complete`, ошибок типов нет.

- [ ] **Step 9: Commit**

```bash
git add course/.vitepress/sidebar.ts course/.vitepress/sidebar.test.ts course/.vitepress/config.ts course/modules course/index.md
git commit -m "feat(course): сайдбар генерируется из папки modules

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Глоссарий и компонент `<T>`

**Files:**
- Create: `course/glossary/schema.ts`, `course/glossary/schema.test.ts`, `course/glossary/load.ts`, `course/glossary/terms.data.ts`, `course/glossary/terms.yaml`, `course/.vitepress/terms-check.ts`, `course/.vitepress/terms-check.test.ts`, `course/components/T.vue`, `course/components/GlossaryList.vue`, `course/glossary.md`
- Modify: `course/.vitepress/config.ts`, `course/.vitepress/theme/index.ts`

**Interfaces:**
- Consumes: `config.ts` и тема из задач 1–2.
- Produces:
  - `parseGlossary(raw: unknown): Glossary`;
  - `sortTerms<T extends Term>(terms: readonly T[]): T[]`;
  - `GlossaryError`;
  - `loadGlossaryFile(path: string): Glossary`;
  - `findTermIds(source: string): string[]`;
  - `findUnknownTerms(source: string, known: ReadonlySet<string>): string[]`;
  - `termsCheckPlugin(md: MarkdownRenderer, known: ReadonlySet<string>): void`;
  - data loader `glossary/terms.data.ts` → `data: Record<string, RenderedTerm>`;
  - глобальные компоненты `<T id="...">` и `<GlossaryList />`;
  - страница `/glossary` с якорем `#<id>` у каждого термина.

- [ ] **Step 1: Написать падающие тесты `course/glossary/schema.test.ts`**

```ts
import { describe, expect, it } from 'vitest'
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
```

- [ ] **Step 2: Запустить и убедиться, что тесты падают**

Run: `npm test -- glossary/schema.test.ts`
Expected: FAIL — `Failed to resolve import "./schema"`.

- [ ] **Step 3: Реализовать `course/glossary/schema.ts`**

```ts
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
  if (long !== undefined) term.long = long
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
```

Поле `en` в первом тесте стоит после `term`. Порядок ключей на `toEqual` не влияет.

- [ ] **Step 4: Запустить тесты**

Run: `npm test -- glossary/schema.test.ts`
Expected: PASS (9 тестов).

- [ ] **Step 5: Написать падающие тесты `course/.vitepress/terms-check.test.ts`**

```ts
import { describe, expect, it } from 'vitest'
import { findTermIds, findUnknownTerms } from './terms-check'

describe('findTermIds', () => {
  it('находит id во всех вхождениях <T>', () => {
    const src = 'Про <T id="wal">WAL</T> и <T id="rps">RPS</T>, снова <T  id="wal">журнал</T>.'
    expect(findTermIds(src)).toEqual(['wal', 'rps', 'wal'])
  })

  it('не путает <T> с другими тегами', () => {
    expect(findTermIds('<Table id="x"></Table><Tip id="y" />')).toEqual([])
  })
})

describe('findUnknownTerms', () => {
  it('возвращает уникальные неизвестные id', () => {
    const src = '<T id="wal">a</T><T id="nope">b</T><T id="nope">c</T>'
    expect(findUnknownTerms(src, new Set(['wal']))).toEqual(['nope'])
  })

  it('пусто, если все известны', () => {
    expect(findUnknownTerms('<T id="wal">a</T>', new Set(['wal']))).toEqual([])
  })
})
```

- [ ] **Step 6: Запустить и убедиться, что тесты падают**

Run: `npm test -- .vitepress/terms-check.test.ts`
Expected: FAIL — `Failed to resolve import "./terms-check"`.

- [ ] **Step 7: Реализовать `course/.vitepress/terms-check.ts`**

```ts
import type { MarkdownRenderer } from 'vitepress'

const TERM_TAG = /<T\s+id="([^"]*)"/g

export function findTermIds(source: string): string[] {
  return [...source.matchAll(TERM_TAG)].map((match) => match[1] ?? '')
}

export function findUnknownTerms(source: string, known: ReadonlySet<string>): string[] {
  return [...new Set(findTermIds(source).filter((id) => !known.has(id)))]
}

// Падаем на сборке, а не показываем пустую подсказку:
// опечатка в id термина должна быть видна сразу.
export function termsCheckPlugin(md: MarkdownRenderer, known: ReadonlySet<string>): void {
  md.core.ruler.push('check-glossary-terms', (state) => {
    const unknown = findUnknownTerms(state.src, known)
    if (unknown.length > 0) {
      const env = state.env as { relativePath?: string }
      throw new Error(`${env.relativePath ?? 'markdown'}: нет в glossary/terms.yaml: ${unknown.join(', ')}`)
    }
    return true
  })
}
```

- [ ] **Step 8: Запустить тесты**

Run: `npm test -- .vitepress/terms-check.test.ts`
Expected: PASS (4 теста).

- [ ] **Step 9: Создать `course/glossary/load.ts` и `course/glossary/terms.data.ts`**

`load.ts`:

```ts
import { readFileSync } from 'node:fs'
import { parse } from 'yaml'
import { parseGlossary, type Glossary } from './schema'

export function loadGlossaryFile(path: string): Glossary {
  return parseGlossary(parse(readFileSync(path, 'utf8')))
}
```

`terms.data.ts` — data loader VitePress. Он выполняется на этапе сборки и рендерит `long` из Markdown в HTML тем же рендерером, что и уроки.

```ts
import { createMarkdownRenderer, defineLoader, type SiteConfig } from 'vitepress'
import { loadGlossaryFile } from './load'
import type { Term } from './schema'

export interface RenderedTerm extends Term {
  longHtml?: string
}

declare const data: Record<string, RenderedTerm>
export { data }

export default defineLoader({
  watch: ['./terms.yaml'],
  async load(watchedFiles: string[]): Promise<Record<string, RenderedTerm>> {
    const file = watchedFiles[0]
    if (file === undefined) throw new Error('glossary/terms.yaml не найден')
    const config = (globalThis as { VITEPRESS_CONFIG?: SiteConfig }).VITEPRESS_CONFIG
    if (config === undefined) throw new Error('VITEPRESS_CONFIG недоступен: loader запущен вне VitePress')
    const md = await createMarkdownRenderer(config.srcDir, config.markdown, config.site.base, config.logger)
    const result: Record<string, RenderedTerm> = {}
    for (const [id, term] of Object.entries(loadGlossaryFile(file))) {
      result[id] = term.long === undefined ? term : { ...term, longHtml: md.render(term.long) }
    }
    return result
  },
})
```

Правило для `terms.yaml`: в `long` нельзя использовать `<T>`. HTML вставляется через `v-html`, и Vue-компоненты внутри него не работают. Ссылки на другие термины делаются обычной Markdown-ссылкой `[WAL](#wal)`.

- [ ] **Step 10: Создать `course/glossary/terms.yaml` со всеми терминами модуля 0**

Поле `long` пишется по стандарту 2.2:
1. Определение простыми словами.
2. Аналогия.
3. Пример с числами.
4. Где термин встретится в курсе.

Объём `long` — 2–6 предложений. Полный список терминов ниже; `term`, `en` и `short` перенести дословно.

```yaml
system-design:
  term: Проектирование систем (system design)
  en: System Design
  short: Выбор компонентов системы, связей между ними и компромиссов так, чтобы система выполняла требования при реальной нагрузке.
  long: |
    Это ответ на вопрос «из чего собрать систему и почему именно так». Если код — это кирпичи, то system design — план здания: где несущие стены, сколько этажей выдержит фундамент и что будет, если половина лифтов сломается. В курсе мы проектируем мессенджер и каждое решение фиксируем вместе с причинами.
  links:
    - title: Клеппман, «Высоконагруженные приложения», гл. 1
functional-requirements:
  term: Функциональные требования
  en: Functional requirements
  short: Что система должна делать — её возможности с точки зрения пользователя.
  long: |
    Отвечают на вопрос «что?»: отправить сообщение, создать группу, найти сообщение по слову. Обычно формулируются как «пользователь может…». Не говорят ничего о скорости и надёжности — это задача нефункциональных требований.
non-functional-requirements:
  term: Нефункциональные требования
  en: Non-functional requirements
  short: Насколько хорошо система должна работать — скорость, доступность, надёжность, масштабируемость, стоимость.
  long: |
    Отвечают на вопрос «как хорошо?»: сообщение доставляется за 1 секунду в 99% случаев, сервис доступен 99,95% времени. Именно они определяют архитектуру: функция «отправить сообщение» одинакова для 100 и для 100 миллионов пользователей, а архитектура — совершенно разная.
scalability:
  term: Масштабируемость
  en: Scalability
  short: Способность системы справляться с ростом нагрузки за счёт добавления ресурсов.
  long: |
    Вертикальное масштабирование — купить сервер мощнее; горизонтальное — добавить ещё серверов. Вертикальное проще, но упирается в потолок самой большой машины. Горизонтальное почти не ограничено, но требует, чтобы систему можно было разделить. Подробно — модули 2.5 и 9.
  links:
    - title: Клеппман, гл. 1, «Масштабируемость»
availability:
  term: Доступность
  en: Availability
  short: Доля времени, когда система отвечает на запросы корректно; обычно выражается в «девятках» (99,9%).
  long: |
    99,9% в год означает до 8,76 часа простоя, 99,99% — до 52,6 минуты. Каждая следующая «девятка» стоит заметно дороже: нужны резервирование, автоматическое переключение и дисциплина выкаток.
  links:
    - title: Google SRE Book, гл. 3 «Embracing Risk»
      url: https://sre.google/sre-book/embracing-risk/
reliability:
  term: Надёжность
  en: Reliability
  short: Способность системы продолжать работать правильно, даже когда что-то ломается.
  long: |
    Сбои неизбежны: диски умирают, сеть теряет пакеты, программисты ошибаются. Надёжная система не та, где ничего не ломается, а та, где поломка одной части не превращается в аварию для пользователя. Клеппман разделяет сбой (fault) — отказ компонента — и отказ (failure) — когда система перестала обслуживать пользователя.
  links:
    - title: Клеппман, гл. 1, «Надёжность»
durability:
  term: Долговечность данных
  en: Durability
  short: Гарантия, что подтверждённые данные не потеряются даже при сбое сервера.
  long: |
    Если мессенджер ответил «отправлено», сообщение не должно исчезнуть после перезагрузки сервера. Достигается записью на диск до подтверждения (fsync) и копиями на других машинах (репликация, модуль 8).
latency:
  term: Задержка
  en: Latency
  short: Время, которое проходит от отправки запроса до получения ответа (или одного шага этого пути).
  long: |
    Измеряется в единицах времени: наносекундах для памяти, миллисекундах для сети. Пользователь ощущает именно задержку. Описывать её нужно перцентилями, а не средним — среднее скрывает самых медленных и самых недовольных пользователей.
  links:
    - title: Клеппман, гл. 1, «Описание производительности»
throughput:
  term: Пропускная способность (фактическая)
  en: Throughput
  short: Сколько работы система выполняет за единицу времени — например, запросов в секунду.
  long: |
    Задержка — «как быстро обслуживается один клиент», throughput — «сколько клиентов обслуживается за секунду». Они связаны, но не одно и то же: можно повысить throughput пачками (батчингом), но при этом увеличить задержку отдельного запроса.
bandwidth:
  term: Ширина канала
  en: Bandwidth
  short: Максимальный объём данных, который канал может передать за единицу времени (например, 1 Гбит/с).
  long: |
    Аналогия — число полос на шоссе. Больше полос — больше машин в час, но каждая машина едет до цели не быстрее. Поэтому «более толстый канал» не уменьшает задержку запроса через океан.
rps:
  term: RPS (запросов в секунду)
  en: Requests Per Second
  short: Число запросов, которые система получает или обрабатывает за одну секунду.
  long: |
    Главная единица нагрузки. Часто встречается также QPS (queries per second) — по смыслу то же самое. Удобное правило: 1 миллион запросов в сутки ≈ 12 RPS в среднем.
dau:
  term: DAU (активные пользователи за день)
  en: Daily Active Users
  short: Число уникальных пользователей, которые хотя бы раз воспользовались продуктом за сутки.
  long: |
    Отправная точка почти любой оценки нагрузки. Рядом встречается MAU (Monthly Active Users) — за месяц; DAU всегда не больше MAU.
percentile:
  term: Перцентиль (p50, p95, p99)
  en: Percentile
  short: Значение, ниже которого лежит заданная доля измерений; p99 = 120 мс значит, что 99% запросов быстрее 120 мс.
  long: |
    p50 — медиана, «типичный» запрос. p99 — хвост: 1 из 100 запросов медленнее. Хвосты важны, потому что при загрузке одной страницы делается много запросов, и шанс встретить медленный растёт.
  links:
    - title: Клеппман, гл. 1, «Перцентили»
sli:
  term: SLI (индикатор уровня обслуживания)
  en: Service Level Indicator
  short: Измеряемая метрика качества сервиса — например, доля запросов, выполненных успешно быстрее 300 мс.
  links:
    - title: Google SRE Book, гл. 4
      url: https://sre.google/sre-book/service-level-objectives/
slo:
  term: SLO (цель уровня обслуживания)
  en: Service Level Objective
  short: Целевое значение SLI за период — например, «99,9% запросов за 30 дней успешны и быстрее 300 мс».
  links:
    - title: Google SRE Book, гл. 4
      url: https://sre.google/sre-book/service-level-objectives/
sla:
  term: SLA (соглашение об уровне обслуживания)
  en: Service Level Agreement
  short: Договор с клиентом о качестве сервиса с последствиями (например, компенсацией) при нарушении.
  long: |
    SLA — юридическое обещание, SLO — внутренняя цель. SLO обычно строже SLA, чтобы команда успела отреагировать до того, как придётся платить.
error-budget:
  term: Бюджет ошибок
  en: Error budget
  short: Допустимый объём «плохой» работы, следующий из SLO; при SLO 99,9% за 30 дней это около 43 минут.
  long: |
    Бюджет можно «тратить» на рискованные выкатки и эксперименты. Когда он исчерпан, команда переключается с новых функций на надёжность.
  links:
    - title: Google SRE Book, гл. 3
      url: https://sre.google/sre-book/embracing-risk/
ip-address:
  term: IP-адрес
  en: IP address
  short: Числовой адрес устройства в сети, по которому доставляются пакеты (например, 93.184.216.34).
dns:
  term: DNS (система доменных имён)
  en: Domain Name System
  short: Распределённая «телефонная книга» интернета — переводит имя вроде api.example.com в IP-адрес.
  long: |
    Ответы DNS кэшируются на устройстве и у провайдера на время TTL. Поэтому повторные запросы почти бесплатны, а смена IP-адреса сервера доходит до пользователей не мгновенно.
tcp:
  term: TCP
  en: Transmission Control Protocol
  short: Протокол, который поверх ненадёжной сети даёт надёжный упорядоченный поток байтов между двумя программами.
  long: |
    TCP пересылает потерянные пакеты и восстанавливает порядок. Плата за это — установка соединения («рукопожатие», 1 RTT) до передачи данных. Подробно — модуль 0b.
tls:
  term: TLS
  en: Transport Layer Security
  short: Протокол шифрования поверх TCP; именно он превращает HTTP в HTTPS.
  long: |
    TLS 1.3 договаривается о ключах за 1 RTT (TLS 1.2 — за 2). Также проверяет сертификат, то есть что вы говорите именно с тем сервером, а не с подменой.
  links:
    - title: RFC 8446 (TLS 1.3)
      url: https://www.rfc-editor.org/rfc/rfc8446
rtt:
  term: RTT (время кругового пути)
  en: Round-Trip Time
  short: Время, за которое пакет доходит до получателя и ответ возвращается обратно.
  long: |
    Внутри одного дата-центра — около 0,5 мс, между континентами — 100–150 мс. Нижняя граница — скорость света в оптоволокне (~200 000 км/с), поэтому никакие оптимизации кода не сделают запрос через океан быстрее нескольких десятков миллисекунд.
http:
  term: HTTP
  en: HyperText Transfer Protocol
  short: Протокол «запрос — ответ» поверх TCP (или QUIC), на котором работают веб и большинство API.
load-balancer:
  term: Балансировщик нагрузки
  en: Load balancer
  short: Компонент, который принимает входящие запросы и распределяет их между несколькими серверами.
  long: |
    Позволяет добавлять серверы незаметно для клиента и выводить из работы сломанные. Подробно — модуль 2.5 (в том числе Nginx).
tls-termination:
  term: TLS-терминация
  en: TLS termination
  short: Расшифровка TLS на балансировщике, после которой запрос внутри дата-центра идёт к серверам уже без шифрования (или с отдельным внутренним).
application-server:
  term: Сервер приложения
  en: Application server
  short: Процесс с бизнес-логикой (в нашем курсе — Go-сервис), который обрабатывает запрос и обращается к базе данных.
database:
  term: База данных
  en: Database
  short: Система, которая надёжно хранит данные и позволяет их эффективно искать и изменять (у нас — PostgreSQL).
fsync:
  term: fsync
  en: fsync
  short: Системный вызов, который заставляет ОС реально записать данные из памяти на диск, а не только в кэш.
  long: |
    Без fsync данные могут «жить» в кэше ОС и пропасть при отключении питания. Базы данных вызывают fsync перед тем, как подтвердить транзакцию, — это часть гарантии долговечности. Подробно — модуль 2b (WAL).
data-center:
  term: Дата-центр (ЦОД)
  en: Data center
  short: Здание с серверами, сетью и питанием; сетевые задержки внутри одного ЦОД — доли миллисекунды.
cpu-cache:
  term: Кэш процессора (L1, L2, L3)
  en: CPU cache
  short: Очень быстрая маленькая память внутри процессора; обращение к L1 — около 1 нс, в ~100 раз быстрее RAM.
ram:
  term: Оперативная память (RAM)
  en: Random Access Memory
  short: Основная рабочая память сервера; быстрая (~100 нс на обращение), но теряет данные при выключении.
ssd:
  term: SSD
  en: Solid-State Drive
  short: Твердотельный накопитель без движущихся частей; случайное чтение — десятки микросекунд.
hdd:
  term: HDD
  en: Hard Disk Drive
  short: Жёсткий диск с вращающимися пластинами; перемещение головки (seek) занимает миллисекунды.
log-scale:
  term: Логарифмическая шкала
  en: Logarithmic scale
  short: Шкала, на которой каждое деление означает умножение (×10), а не прибавление; позволяет на одном графике показать и 1 нс, и 150 мс.
littles-law:
  term: Закон Литтла
  en: Little's law
  short: L = λ × W — среднее число запросов в системе равно интенсивности потока, умноженной на среднее время пребывания.
  long: |
    Пример: 1000 RPS × 0,1 с = в среднем 100 одновременно обрабатываемых запросов. Отсюда размеры пулов соединений и число воркеров.
back-of-envelope:
  term: Оценка «на салфетке»
  en: Back-of-the-envelope estimation
  short: Быстрый приблизительный расчёт нагрузки и объёмов с округлением до порядка величины.
  links:
    - title: Сюй, «System Design», т. 1, гл. 2
peak-factor:
  term: Коэффициент пика
  en: Peak factor
  short: Во сколько раз пиковая нагрузка выше средней (например, вечером или в Новый год).
replication-factor:
  term: Коэффициент репликации
  en: Replication factor
  short: Сколько копий каждого фрагмента данных хранится (обычно 3) — ради надёжности и доступности.
retention:
  term: Срок хранения
  en: Retention
  short: Как долго система хранит данные, прежде чем удалить или перенести их в архив.
metadata:
  term: Метаданные
  en: Metadata
  short: Данные о данных — например, у сообщения это id, автор, чат, время отправки, статус.
active-recall:
  term: Активное воспроизведение
  en: Active recall
  short: Приём обучения — вспомнить и объяснить материал самому, не подглядывая, вместо перечитывания.
  links:
    - title: Brown, Roediger, McDaniel, «Make It Stick»
spaced-repetition:
  term: Интервальное повторение
  en: Spaced repetition
  short: Повторение материала с растущими паузами (через день, неделю, месяц), чтобы он перешёл в долговременную память.
  links:
    - title: Brown, Roediger, McDaniel, «Make It Stick»
adr:
  term: ADR (запись архитектурного решения)
  en: Architecture Decision Record
  short: Короткий документ — контекст, рассмотренные варианты, принятое решение и его последствия.
  links:
    - title: Michael Nygard, «Documenting Architecture Decisions»
      url: https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions
```

Для терминов, у которых в списке выше нет `long`, дописать `long` по тому же шаблону (2–6 предложений: аналогия, число, где встретится в курсе). Исключение — термины, где `short` исчерпывает смысл: `ip-address`, `http`, `database`, `data-center`. Для них `long` можно не писать.

- [ ] **Step 11: Подключить проверку терминов в `config.ts`**

Добавить импорты и константы:

```ts
import { loadGlossaryFile } from '../glossary/load'
import { termsCheckPlugin } from './terms-check'

const GLOSSARY_FILE = fileURLToPath(new URL('../glossary/terms.yaml', import.meta.url))
const KNOWN_TERMS: ReadonlySet<string> = new Set(Object.keys(loadGlossaryFile(GLOSSARY_FILE)))
```

На верхний уровень `defineConfig({...})` добавить:

```ts
  markdown: {
    config: (md) => termsCheckPlugin(md, KNOWN_TERMS),
  },
```

В `nav` добавить `{ text: 'Глоссарий', link: '/glossary' }`.

`sidebar` заменить на:

```ts
    sidebar: [
      ...buildSidebar(MODULES_DIR),
      { text: 'Справочник', items: [{ text: 'Глоссарий', link: '/glossary' }] },
    ],
```

- [ ] **Step 12: Создать `course/components/T.vue`**

```vue
<script setup lang="ts">
import { computed, useId } from 'vue'
import { withBase } from 'vitepress'
import { data as glossary } from '../glossary/terms.data'

const props = defineProps<{ id: string }>()
const term = computed(() => glossary[props.id])
const href = computed(() => withBase(`/glossary#${props.id}`))
const tipId = useId()
</script>

<template>
  <span class="term">
    <a class="term-link" :href="href" :aria-describedby="term ? tipId : undefined"><slot /></a>
    <span v-if="term" :id="tipId" role="tooltip" class="term-tip">
      <strong>{{ term.term }}</strong>
      <span v-if="term.en" class="term-en"> ({{ term.en }})</span>
      <span class="term-short">{{ term.short }}</span>
    </span>
  </span>
</template>

<style scoped>
.term { position: relative; }
.term-link {
  color: inherit;
  text-decoration: underline dotted var(--vp-c-brand-1);
  text-decoration-thickness: 2px;
  text-underline-offset: 3px;
  font-weight: inherit;
}
.term-tip {
  display: none;
  position: absolute;
  z-index: 20;
  left: 0;
  top: calc(100% + 6px);
  width: max-content;
  max-width: min(340px, 80vw);
  padding: 10px 12px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  background: var(--vp-c-bg-elv);
  box-shadow: var(--vp-shadow-3);
  color: var(--vp-c-text-1);
  font-size: 14px;
  line-height: 1.5;
  font-weight: 400;
}
.term-en { color: var(--vp-c-text-2); }
.term-short { display: block; margin-top: 4px; }
.term:hover .term-tip,
.term:focus-within .term-tip { display: block; }

/* На узком экране подсказка прижимается к низу окна — иначе уходит за край. */
@media (max-width: 640px) {
  .term-tip { position: fixed; left: 16px; right: 16px; top: auto; bottom: 16px; width: auto; max-width: none; }
}
</style>
```

- [ ] **Step 13: Создать `course/components/GlossaryList.vue`**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { data as glossary } from '../glossary/terms.data'
import { sortTerms } from '../glossary/schema'

const terms = computed(() => sortTerms(Object.values(glossary)))
</script>

<template>
  <dl class="glossary">
    <template v-for="t in terms" :key="t.id">
      <dt :id="t.id">
        <a class="anchor" :href="`#${t.id}`" :aria-label="`Ссылка на термин ${t.term}`">#</a>
        {{ t.term }}<span v-if="t.en" class="en"> — {{ t.en }}</span>
      </dt>
      <dd>
        <p>{{ t.short }}</p>
        <div v-if="t.longHtml" class="long" v-html="t.longHtml" />
        <ul v-if="t.links.length" class="links">
          <li v-for="link in t.links" :key="link.title">
            <a v-if="link.url" :href="link.url" target="_blank" rel="noopener">{{ link.title }}</a>
            <span v-else>{{ link.title }}</span>
          </li>
        </ul>
      </dd>
    </template>
  </dl>
</template>

<style scoped>
.glossary dt { margin-top: 28px; font-size: 18px; font-weight: 600; scroll-margin-top: calc(var(--vp-nav-height) + 16px); }
.glossary dt:target { color: var(--vp-c-brand-1); }
.anchor { margin-right: 6px; color: var(--vp-c-text-3); text-decoration: none; }
.en { color: var(--vp-c-text-2); font-weight: 400; }
.glossary dd { margin: 6px 0 0; }
.links { font-size: 14px; color: var(--vp-c-text-2); }
</style>
```

- [ ] **Step 14: Создать `course/glossary.md`**

```md
---
title: Глоссарий
outline: false
---

# Глоссарий

Все термины курса в одном месте. В уроках термины подчёркнуты пунктиром: наведите на них курсор (или нажмите на телефоне), чтобы увидеть короткое определение, а клик приведёт сюда за подробным объяснением.

<GlossaryList />
```

- [ ] **Step 15: Зарегистрировать компоненты в теме**

Заменить `course/.vitepress/theme/index.ts` целиком:

```ts
import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import T from '../../components/T.vue'
import GlossaryList from '../../components/GlossaryList.vue'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('T', T)
    app.component('GlossaryList', GlossaryList)
  },
} satisfies Theme
```

- [ ] **Step 16: Проверить, что сборка падает на неизвестном термине**

Временно дописать в конец `course/modules/00-foundation/index.md` строку `Проверка <T id="no-such-term">x</T>.`

Run: `npm run build`
Expected: FAIL с текстом `modules/00-foundation/index.md: нет в glossary/terms.yaml: no-such-term`.

Затем удалить эту строку.

- [ ] **Step 17: Полная проверка**

Run: `npm test && npm run typecheck && npm run build`
Expected: все тесты PASS, типы без ошибок, `build complete`.

Затем `npm run dev` и вручную:
- `/glossary` показывает все термины по алфавиту;
- переход на `/glossary#rtt` прокручивает к термину;
- `long` отрендерен как HTML.

- [ ] **Step 18: Commit**

```bash
git add course/glossary course/glossary.md course/components course/.vitepress
git commit -m "feat(course): глоссарий, компонент термина и проверка терминов при сборке

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Компоненты урока, Mermaid, уроки 0.1 и 0.2

**Files:**
- Create: `course/components/Callout.vue`, `course/components/SelfCheck.vue`, `course/modules/00-foundation/01-how-to-study.md`, `course/modules/00-foundation/02-what-is-system-design.md`
- Modify: `course/.vitepress/config.ts`, `course/.vitepress/theme/index.ts`

**Interfaces:**
- Consumes: `<T>` из задачи 3.
- Produces:
  - `<Callout type="idea|mistake|tradeoff|source" title?="...">` со слотом на Markdown;
  - `<SelfCheck question="...">` со слотом-ответом;
  - блоки ` ```mermaid ` в уроках.

- [ ] **Step 1: Создать `course/components/Callout.vue`**

```vue
<script setup lang="ts">
import { computed } from 'vue'

type Kind = 'idea' | 'mistake' | 'tradeoff' | 'source'

const KINDS: Record<Kind, { icon: string; title: string }> = {
  idea: { icon: '💡', title: 'Ключевая идея' },
  mistake: { icon: '⚠️', title: 'Типичная ошибка' },
  tradeoff: { icon: '⚖️', title: 'Компромисс' },
  source: { icon: '📚', title: 'Источник' },
}

const props = defineProps<{ type: Kind; title?: string }>()
const meta = computed(() => KINDS[props.type])
</script>

<template>
  <aside class="callout" :class="`callout-${type}`">
    <p class="callout-title"><span aria-hidden="true">{{ meta.icon }}</span> {{ title ?? meta.title }}</p>
    <div class="callout-body"><slot /></div>
  </aside>
</template>

<style scoped>
.callout { margin: 20px 0; padding: 14px 18px; border-left: 4px solid; border-radius: 8px; }
.callout-title { margin: 0 0 6px; font-weight: 600; }
.callout-body :deep(p:last-child) { margin-bottom: 0; }
.callout-idea { border-color: var(--vp-c-tip-1); background: var(--vp-c-tip-soft); }
.callout-mistake { border-color: var(--vp-c-danger-1); background: var(--vp-c-danger-soft); }
.callout-tradeoff { border-color: var(--vp-c-warning-1); background: var(--vp-c-warning-soft); }
.callout-source { border-color: var(--vp-c-gray-1); background: var(--vp-c-gray-soft); }
</style>
```

- [ ] **Step 2: Создать `course/components/SelfCheck.vue`**

```vue
<script setup lang="ts">
import { ref } from 'vue'

defineProps<{ question: string }>()
const open = ref(false)
</script>

<template>
  <div class="self-check">
    <p class="question"><strong>Вопрос.</strong> {{ question }}</p>
    <button type="button" class="toggle" :aria-expanded="open" @click="open = !open">
      {{ open ? 'Скрыть ответ' : 'Сначала ответьте сами, затем откройте ответ' }}
    </button>
    <div v-show="open" class="answer"><slot /></div>
  </div>
</template>

<style scoped>
.self-check { margin: 16px 0; padding: 14px 18px; border: 1px solid var(--vp-c-divider); border-radius: 8px; }
.question { margin: 0 0 10px; }
.toggle { padding: 6px 12px; border: 1px solid var(--vp-c-brand-1); border-radius: 6px; color: var(--vp-c-brand-1); font-size: 14px; }
.toggle:hover { background: var(--vp-c-brand-soft); }
.answer { margin-top: 12px; }
.answer :deep(p:last-child) { margin-bottom: 0; }
</style>
```

- [ ] **Step 3: Зарегистрировать компоненты и подключить Mermaid**

В `course/.vitepress/theme/index.ts` добавить импорты и регистрацию:

```ts
import Callout from '../../components/Callout.vue'
import SelfCheck from '../../components/SelfCheck.vue'
// внутри enhanceApp:
    app.component('Callout', Callout)
    app.component('SelfCheck', SelfCheck)
```

В `course/.vitepress/config.ts`:
- добавить `import { withMermaid } from 'vitepress-plugin-mermaid'`;
- обернуть экспорт: `export default withMermaid(defineConfig({ ... }))`.

- [ ] **Step 4: Написать урок `01-how-to-study.md`**

Frontmatter: `title: 0.1. Как устроен курс и как учиться`.

Писать по «Голосу автора» (спецификация, раздел 1): обращение на «вы», короткие абзацы. Обязательные разделы в этом порядке:

1. **Зачем этот курс.** Кто автор (голос из спецификации), для кого курс, что вы сможете в конце: три критерия успеха из спецификации, раздел 1.
2. **Карта курса.** Mermaid-схема (`flowchart LR`):
   - сплошные стрелки — модули ядра по порядку: 0 → 0b → 1 → 1.5 → 2a → 2b → 2.5 → 3 → 4 → 5 → 6 → 6.5 → 6.7 → 7 → 8 → 9 → 12 → 13a → 15;
   - пунктир (`-.->`) — модули углубления от тех, после которых их можно проходить: 9 -.-> 9.5, 9 -.-> 10, 7 -.-> 11, 13a -.-> 13b, 12 -.-> 14.

   Под схемой — абзац про «ядро» (Middle+) и «углубление» (Senior).
3. **Как устроен урок.** Восемь частей шаблона (спецификация, 2.2): одной строкой, зачем каждая.
4. **Как учиться, чтобы помнить.**
   - <T id="active-recall">активное воспроизведение</T>, <T id="spaced-repetition">интервальное повторение</T>, «объясните своими словами» — со ссылкой на «Make It Stick»;
   - почему перечитывание создаёт иллюзию знания;
   - конкретный ритуал: после урока закрыть страницу и за 5 минут записать главное, через день — ответить на вопросы самопроверки.
5. **Как пользоваться сайтом.**
   - подчёркнутые термины и глоссарий;
   - визуализации: кнопки и клавиши ← → и пробел;
   - блоки 💡 ⚠️ ⚖️ 📚.

   Каждый тип блока показать живым примером `<Callout>`.
6. **Сквозные практики.** <T id="adr">ADR</T>, мини-интервью в конце модуля, разборы реальных систем (спецификация, 2.1).
7. **Самопроверка.** 3 `<SelfCheck>`:
   - «Почему перечитывание конспекта — слабый способ учиться?»;
   - «Чем модули ядра отличаются от модулей углубления и когда браться за углубление?»;
   - «Что такое ADR и зачем записывать причины решения, а не только само решение?»
8. **Источники.** `<Callout type="source">`: Brown, Roediger, McDaniel «Make It Stick: The Science of Successful Learning» (2014); M. Nygard, «Documenting Architecture Decisions» (2011).

- [ ] **Step 5: Написать урок `02-what-is-system-design.md`**

Frontmatter: `title: 0.2. Что такое system design и требования`.

Обязательные разделы:

1. **Проблема.** История: сервис заметок работал у 1 000 пользователей. После рекламы пришли 200 000, и база упала от числа соединений. Функции те же — архитектура нет.
2. **Что такое <T id="system-design">system design</T>.** High-level design (компоненты и связи) против low-level design (классы, схемы таблиц). В курсе есть оба.
3. **Требования.**
   - <T id="functional-requirements">функциональные</T> и <T id="non-functional-requirements">нефункциональные</T> — по 5 примеров для мессенджера;
   - <Callout type="idea">: «архитектуру определяют нефункциональные требования».
4. **Нефункциональные требования по одному.** <T id="scalability">масштабируемость</T>, <T id="availability">доступность</T>, <T id="reliability">надёжность</T>, <T id="durability">долговечность</T>, <T id="latency">задержка</T>, <T id="throughput">пропускная способность</T>, согласованность (одной фразой со ссылкой вперёд на модули 8 и 10), безопасность, стоимость. Для каждого — определение, пример из мессенджера, вопрос, который надо задать заказчику.
5. **Доступность в «девятках».** Таблица допустимого простоя:

   | Доступность | В год | В месяц (30 дней) |
   |---|---|---|
   | 99% | 3,65 дня | 7,2 часа |
   | 99,9% | 8,76 часа | 43,2 минуты |
   | 99,99% | 52,6 минуты | 4,32 минуты |
   | 99,999% | 5,26 минуты | 25,9 секунды |

   Вывод цифр: (1 − 0,999) × 365 × 24 = 8,76 ч.
6. **Доступность цепочки компонентов.**
   - последовательно: 0,999 × 0,999 × 0,999 ≈ 0,997 — три сервиса по «три девятки» дают меньше трёх девяток;
   - параллельно (резерв): 1 − (1 − 0,99)² = 0,9999.

   Mermaid-схема двух вариантов (`flowchart LR`).
7. **SLI, SLO, SLA и бюджет ошибок.** Термины <T id="sli">SLI</T>, <T id="slo">SLO</T>, <T id="sla">SLA</T>, <T id="error-budget">бюджет ошибок</T>.

   Пример для мессенджера:
   - SLI — доля запросов `POST /messages`, успешных и быстрее 300 мс;
   - SLO — 99,9% за 30 дней;
   - бюджет — 43 минуты «плохой» работы.

   <Callout type="mistake">: путать SLA и SLO; ставить SLO 100%.
8. **Почему среднее врёт: <T id="percentile">перцентили</T>.**
   - 10 замеров: 20, 22, 25, 25, 28, 30, 31, 35, 40, 900 мс;
   - среднее = 115,6 мс — его не видел ни один пользователь;
   - p50 = 29 мс (среднее 5-го и 6-го значений), p90 = 40 мс, максимум 900 мс;
   - объяснить, почему хвост важен: страница из 20 запросов.
9. **Компромисс.** <Callout type="tradeoff">: каждая «девятка» дорожает. Подать как эвристику, а не закон: резервирование, автоматический failover, дисциплина выкаток.
10. **Самопроверка.** 5 `<SelfCheck>`:
    - «“Пользователь может удалить сообщение” — какое это требование?» (функциональное);
    - «Почему функциональные требования мессенджера на 100 и на 100 млн пользователей одинаковы, а архитектура — нет?»;
    - «Три сервиса с доступностью 99,9% вызываются последовательно. Какая итоговая доступность и сколько это минут простоя в месяц?» (≈ 99,7%, ≈ 130 минут);
    - «Чем SLO отличается от SLA?»;
    - «Почему p99 важнее среднего для пользовательского опыта?»
11. **Источники.** Клеппман, гл. 1; Сюй, т. 1, гл. 1; Google SRE Book, гл. 3–4 (со ссылками https://sre.google/sre-book/embracing-risk/ и https://sre.google/sre-book/service-level-objectives/).

- [ ] **Step 6: Проверить**

Run: `npm run build`
Expected: `build complete`, без ошибок терминов.

Затем `npm run dev` и вручную:
- обе Mermaid-схемы рендерятся в светлой и тёмной теме;
- `SelfCheck` раскрывается;
- подсказки терминов появляются при наведении и при Tab-фокусе.

Если dev-сервер ругается на ESM-зависимость mermaid (`dayjs`, `@braintree/sanitize-url`), добавить в конфиг `vite: { optimizeDeps: { include: ['mermaid'] } }` и перепроверить.

Дальше — чек-лист стандарта 2.2 по каждому из двух уроков: пройти все 9 пунктов и исправить недостающее.

- [ ] **Step 7: Commit**

```bash
git add course/components course/.vitepress course/modules/00-foundation
git commit -m "feat(course): компоненты урока, Mermaid, уроки 0.1 и 0.2

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Ядро движка визуализаций — типы, плеер, форматирование

**Files:**
- Create: `course/viz/core/types.ts`, `course/viz/core/player.ts`, `course/viz/core/player.test.ts`, `course/viz/core/format.ts`, `course/viz/core/format.test.ts`, `course/viz/core/motion.ts`

**Interfaces:**
- Produces:
  - `Step<S>`, `Scenario<S>`;
  - `SPEEDS`, `Speed`, `BASE_STEP_MS`, `PlayerState`, `PlayerAction`;
  - `initPlayer(total: number): PlayerState`;
  - `playerReducer(s: PlayerState, a: PlayerAction): PlayerState`;
  - `stepDelayMs(speed: Speed): number`;
  - `keyToAction(key: string, target: { tagName: string } | null): PlayerAction | null`;
  - `formatNumber(value: number, fractionDigits?: number): string`;
  - `formatMs(ms: number): string`;
  - `formatNs(ns: number): string`;
  - `formatBytes(bytes: number): string`;
  - `formatSeconds(seconds: number): string`;
  - `prefersReducedMotion(): boolean`.

- [ ] **Step 1: Создать `course/viz/core/types.ts`**

```ts
// Один кадр визуализации. state — полный снимок, а не дельта:
// так переход назад и прыжок на любой шаг не требуют пересчёта.
export interface Step<S> {
  state: S
  caption: string
  highlight?: readonly string[]
  codeLine?: number
}

export type Scenario<S> = () => readonly Step<S>[]
```

- [ ] **Step 2: Написать падающие тесты `course/viz/core/player.test.ts`**

```ts
import { describe, expect, it } from 'vitest'
import { BASE_STEP_MS, initPlayer, keyToAction, playerReducer, stepDelayMs, type PlayerAction, type PlayerState } from './player'

const at = (index: number, playing = false): PlayerState => ({ index, total: 5, playing, speed: 1 })

describe('initPlayer', () => {
  it('начинает с первого шага на паузе', () => {
    expect(initPlayer(5)).toEqual({ index: 0, total: 5, playing: false, speed: 1 })
  })
  it.each([0, -1, 2.5, Number.NaN])('отклоняет total=%s', (total) => {
    expect(() => initPlayer(total)).toThrow(RangeError)
  })
})

describe('playerReducer', () => {
  it.each<[string, PlayerState, PlayerAction, Partial<PlayerState>]>([
    ['next двигает вперёд', at(1), { type: 'next' }, { index: 2 }],
    ['next не уходит за конец', at(4), { type: 'next' }, { index: 4 }],
    ['prev двигает назад', at(2), { type: 'prev' }, { index: 1 }],
    ['prev не уходит за начало', at(0), { type: 'prev' }, { index: 0 }],
    ['first', at(3), { type: 'first' }, { index: 0 }],
    ['last', at(1), { type: 'last' }, { index: 4 }],
    ['seek внутрь', at(0), { type: 'seek', index: 3 }, { index: 3 }],
    ['seek за границу зажимается', at(0), { type: 'seek', index: 99 }, { index: 4 }],
    ['seek отрицательный зажимается', at(2), { type: 'seek', index: -5 }, { index: 0 }],
    ['ручной шаг ставит паузу', at(1, true), { type: 'next' }, { index: 2, playing: false }],
    ['play', at(1), { type: 'play' }, { index: 1, playing: true }],
    ['play на последнем шаге начинает сначала', at(4), { type: 'play' }, { index: 0, playing: true }],
    ['pause', at(1, true), { type: 'pause' }, { playing: false }],
    ['toggle с паузы', at(1), { type: 'toggle' }, { playing: true }],
    ['toggle с воспроизведения', at(1, true), { type: 'toggle' }, { playing: false }],
    ['tick на паузе ничего не делает', at(1), { type: 'tick' }, { index: 1, playing: false }],
    ['tick двигает вперёд', at(1, true), { type: 'tick' }, { index: 2, playing: true }],
    ['tick на предпоследнем доходит до конца и останавливается', at(3, true), { type: 'tick' }, { index: 4, playing: false }],
    ['speed', at(1), { type: 'speed', speed: 2 }, { speed: 2 }],
    ['reset при смене шагов', at(3, true), { type: 'reset', total: 8 }, { index: 0, total: 8, playing: false }],
  ])('%s', (_name, state, action, expected) => {
    expect(playerReducer(state, action)).toMatchObject(expected)
  })
})

describe('stepDelayMs', () => {
  it('обратно пропорциональна скорости', () => {
    expect(stepDelayMs(1)).toBe(BASE_STEP_MS)
    expect(stepDelayMs(2)).toBe(BASE_STEP_MS / 2)
    expect(stepDelayMs(0.5)).toBe(BASE_STEP_MS * 2)
  })
})

describe('keyToAction', () => {
  const div = { tagName: 'FIGURE' }
  it.each<[string, { tagName: string } | null, PlayerAction | null]>([
    ['ArrowRight', div, { type: 'next' }],
    ['ArrowLeft', div, { type: 'prev' }],
    [' ', div, { type: 'toggle' }],
    ['Home', div, { type: 'first' }],
    ['End', div, { type: 'last' }],
    ['a', div, null],
    // Пробел на кнопке нажимает саму кнопку — иначе получится двойное срабатывание.
    [' ', { tagName: 'BUTTON' }, null],
    ['ArrowRight', { tagName: 'BUTTON' }, { type: 'next' }],
    // Внутри ползунка и полей клавиши принадлежат им.
    ['ArrowRight', { tagName: 'INPUT' }, null],
    ['ArrowLeft', { tagName: 'SELECT' }, null],
    ['ArrowRight', null, { type: 'next' }],
  ])('%j на %j → %j', (key, target, expected) => {
    expect(keyToAction(key, target)).toEqual(expected)
  })
})
```

- [ ] **Step 3: Запустить и убедиться, что тесты падают**

Run: `npm test -- viz/core/player.test.ts`
Expected: FAIL — `Failed to resolve import "./player"`.

- [ ] **Step 4: Реализовать `course/viz/core/player.ts`**

```ts
export const SPEEDS = [0.5, 1, 2] as const
export type Speed = (typeof SPEEDS)[number]

// 1,8 с на шаг — успеть прочитать пояснение из 1–2 предложений.
export const BASE_STEP_MS = 1800

export interface PlayerState {
  index: number
  total: number
  playing: boolean
  speed: Speed
}

export type PlayerAction =
  | { type: 'next' }
  | { type: 'prev' }
  | { type: 'first' }
  | { type: 'last' }
  | { type: 'seek'; index: number }
  | { type: 'play' }
  | { type: 'pause' }
  | { type: 'toggle' }
  | { type: 'tick' }
  | { type: 'speed'; speed: Speed }
  | { type: 'reset'; total: number }

export function initPlayer(total: number): PlayerState {
  if (!Number.isInteger(total) || total < 1) throw new RangeError(`player: число шагов должно быть целым ≥ 1, получено ${total}`)
  return { index: 0, total, playing: false, speed: 1 }
}

function clampIndex(index: number, total: number): number {
  return Math.min(Math.max(Math.trunc(index), 0), total - 1)
}

function moveTo(s: PlayerState, index: number): PlayerState {
  return { ...s, index: clampIndex(index, s.total), playing: false }
}

export function playerReducer(s: PlayerState, a: PlayerAction): PlayerState {
  const isLast = s.index === s.total - 1
  switch (a.type) {
    case 'next':
      return moveTo(s, s.index + 1)
    case 'prev':
      return moveTo(s, s.index - 1)
    case 'first':
      return moveTo(s, 0)
    case 'last':
      return moveTo(s, s.total - 1)
    case 'seek':
      return moveTo(s, a.index)
    case 'play':
      return isLast ? { ...s, index: 0, playing: true } : { ...s, playing: true }
    case 'pause':
      return { ...s, playing: false }
    case 'toggle':
      return playerReducer(s, { type: s.playing ? 'pause' : 'play' })
    case 'tick': {
      if (!s.playing) return s
      if (isLast) return { ...s, playing: false }
      const index = s.index + 1
      return { ...s, index, playing: index < s.total - 1 }
    }
    case 'speed':
      return { ...s, speed: a.speed }
    case 'reset':
      return initPlayer(a.total)
  }
}

export function stepDelayMs(speed: Speed): number {
  return BASE_STEP_MS / speed
}

const KEY_ACTIONS: Record<string, PlayerAction> = {
  ArrowRight: { type: 'next' },
  ArrowLeft: { type: 'prev' },
  ' ': { type: 'toggle' },
  Home: { type: 'first' },
  End: { type: 'last' },
}

const OWN_KEYS_TAGS = new Set(['INPUT', 'SELECT', 'TEXTAREA'])

export function keyToAction(key: string, target: { tagName: string } | null): PlayerAction | null {
  const action = KEY_ACTIONS[key]
  if (action === undefined) return null
  const tag = target?.tagName.toUpperCase() ?? ''
  if (OWN_KEYS_TAGS.has(tag)) return null
  if (key === ' ' && tag === 'BUTTON') return null
  return action
}
```

- [ ] **Step 5: Запустить тесты**

Run: `npm test -- viz/core/player.test.ts`
Expected: PASS.

- [ ] **Step 6: Написать падающие тесты `course/viz/core/format.test.ts`**

`NBSP` — неразрывный пробел: в русской локали им разделяются разряды.

```ts
import { describe, expect, it } from 'vitest'
import { formatBytes, formatMs, formatNs, formatNumber, formatSeconds } from './format'

const NBSP = ' '

describe('formatNumber', () => {
  it.each<[number, number | undefined, string]>([
    [0, undefined, '0'],
    [1234567, undefined, `1${NBSP}234${NBSP}567`],
    [46296.296, 1, `46${NBSP}296,3`],
    [0.5, 1, '0,5'],
  ])('%s (%s знаков) → %s', (value, digits, expected) => {
    expect(formatNumber(value, digits)).toBe(expected)
  })
})

describe('formatMs', () => {
  it.each([
    [0, `0${NBSP}мс`],
    [0.5, `0,5${NBSP}мс`],
    [2, `2${NBSP}мс`],
    [144, `144${NBSP}мс`],
  ])('%s → %s', (ms, expected) => expect(formatMs(ms)).toBe(expected))
})

describe('formatNs', () => {
  it.each([
    [1, `1${NBSP}нс`],
    [100, `100${NBSP}нс`],
    [3_000, `3${NBSP}мкс`],
    [16_000, `16${NBSP}мкс`],
    [500_000, `500${NBSP}мкс`],
    [2_000_000, `2${NBSP}мс`],
    [150_000_000, `150${NBSP}мс`],
    [2_500_000_000, `2,5${NBSP}с`],
  ])('%s → %s', (ns, expected) => expect(formatNs(ns)).toBe(expected))
})

describe('formatBytes', () => {
  it.each([
    [0, `0${NBSP}Б`],
    [999, `999${NBSP}Б`],
    [1_500, `1,5${NBSP}КБ`],
    [4e11, `400${NBSP}ГБ`],
    [4.38e15, `4,4${NBSP}ПБ`],
    [2.78e7, `28${NBSP}МБ`],
  ])('%s → %s', (bytes, expected) => expect(formatBytes(bytes)).toBe(expected))
})

describe('formatSeconds', () => {
  it.each([
    [1, `1${NBSP}с`],
    [100, `1,7${NBSP}мин`],
    [16_000, `4,4${NBSP}ч`],
    [2_000_000, `23${NBSP}дн.`],
    [150_000_000, `4,8${NBSP}г.`],
  ])('%s → %s', (s, expected) => expect(formatSeconds(s)).toBe(expected))
})
```

- [ ] **Step 7: Запустить и убедиться, что тесты падают**

Run: `npm test -- viz/core/format.test.ts`
Expected: FAIL — `Failed to resolve import "./format"`.

- [ ] **Step 8: Реализовать `course/viz/core/format.ts`**

```ts
const NBSP = ' '

// ICU в разных версиях Node использует для разрядов то U+00A0, то U+202F.
// Приводим к одному символу, чтобы вывод был стабильным.
export function formatNumber(value: number, fractionDigits = 0): string {
  return new Intl.NumberFormat('ru-RU', { maximumFractionDigits: fractionDigits })
    .format(value)
    .replace(/[   ]/g, NBSP)
}

// Один знак после запятой для значений меньше 10: «2,5 с» информативнее «3 с»,
// а «144,3 мс» — ложная точность.
function compact(value: number): string {
  return formatNumber(value, Math.abs(value) < 10 ? 1 : 0)
}

interface Unit {
  size: number
  label: string
}

function inUnits(value: number, units: readonly Unit[]): string {
  let chosen = units[0] as Unit
  for (const unit of units) if (Math.abs(value) >= unit.size) chosen = unit
  return `${compact(value / chosen.size)}${NBSP}${chosen.label}`
}

const NS_UNITS: readonly Unit[] = [
  { size: 1, label: 'нс' },
  { size: 1e3, label: 'мкс' },
  { size: 1e6, label: 'мс' },
  { size: 1e9, label: 'с' },
]

// Десятичные единицы (1 КБ = 1000 Б): для оценок на салфетке это проще,
// а разница с двоичными (КиБ) — меньше 10% на любом порядке до ТБ.
const BYTE_UNITS: readonly Unit[] = ['Б', 'КБ', 'МБ', 'ГБ', 'ТБ', 'ПБ', 'ЭБ'].map((label, i) => ({ size: 1000 ** i, label }))

const SECOND_UNITS: readonly Unit[] = [
  { size: 1, label: 'с' },
  { size: 60, label: 'мин' },
  { size: 3_600, label: 'ч' },
  { size: 86_400, label: 'дн.' },
  { size: 31_536_000, label: 'г.' },
]

export function formatMs(ms: number): string {
  return `${compact(ms)}${NBSP}мс`
}

export function formatNs(ns: number): string {
  return inUnits(ns, NS_UNITS)
}

export function formatBytes(bytes: number): string {
  return inUnits(bytes, BYTE_UNITS)
}

export function formatSeconds(seconds: number): string {
  return inUnits(seconds, SECOND_UNITS)
}
```

- [ ] **Step 9: Запустить тесты**

Run: `npm test -- viz/core/format.test.ts`
Expected: PASS.

- [ ] **Step 10: Создать `course/viz/core/motion.ts`**

```ts
// Вызывать только на клиенте (onMounted, обработчики событий): при SSR window нет.
export function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}
```

- [ ] **Step 11: Проверить типы и коммит**

Run: `npm test && npm run typecheck`
Expected: PASS, без ошибок типов.

```bash
git add course/viz/core
git commit -m "feat(viz): ядро движка — типы шагов, редьюсер плеера, форматирование

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Первая визуализация — путь запроса (плеер, панель кода, палитра, урок 0.3)

**Files:**
- Create: `course/viz/simulations/request-path.ts`, `course/viz/simulations/request-path.test.ts`, `course/viz/components/Player.vue`, `course/viz/components/CodePanel.vue`, `course/viz/renderers/RequestPathView.vue`, `course/viz/scenes/RequestPath.vue`, `course/viz/palette.css`, `course/viz/README.md`, `course/modules/00-foundation/03-request-path.md`
- Modify: `course/.vitepress/theme/index.ts`

**Interfaces:**
- Consumes: `Step`, `initPlayer`, `playerReducer`, `stepDelayMs`, `keyToAction`, `SPEEDS`, `Speed`, `PlayerAction` (задача 5); `formatMs`, `prefersReducedMotion` (задача 5).
- Produces:
  - `NodeId`, `NODES`, `Packet`, `RequestPathState`, `LATENCY_MS`, `CODE_LINE`, `TOTAL_MS`;
  - `requestPathScenario(): Step<RequestPathState>[]`;
  - `<Player :steps label>` с default-слотом `{ step, index }` и опциональным слотом `#code="{ line }"`;
  - `<CodePanel :line>` со слотом под отрендеренный блок кода;
  - CSS-переменные `--viz-*`;
  - `<RequestPath>` — сцена для урока, default-слот = Markdown-блок Go-кода.

- [ ] **Step 1: Написать падающие тесты `course/viz/simulations/request-path.test.ts`**

```ts
import { describe, expect, it } from 'vitest'
import { LATENCY_MS, NODES, TOTAL_MS, requestPathScenario } from './request-path'

const steps = requestPathScenario()
const nodeIds = new Set(NODES.map((n) => n.id))

describe('requestPathScenario', () => {
  it('состоит из 15 шагов', () => {
    expect(steps).toHaveLength(15)
  })

  it('начинается на клиенте без пакета и с нулевым временем', () => {
    expect(steps[0]?.state).toEqual({ active: 'client', packet: null, elapsedMs: 0 })
  })

  it('время не убывает от шага к шагу', () => {
    for (let i = 1; i < steps.length; i++) {
      expect(steps[i]!.state.elapsedMs).toBeGreaterThanOrEqual(steps[i - 1]!.state.elapsedMs)
    }
  })

  it('пакет всегда прибывает в активный узел и ходит между разными существующими узлами', () => {
    for (const { state } of steps) {
      expect(nodeIds.has(state.active)).toBe(true)
      if (state.packet === null) continue
      expect(state.packet.to).toBe(state.active)
      expect(state.packet.from).not.toBe(state.packet.to)
      expect(nodeIds.has(state.packet.from)).toBe(true)
    }
  })

  it('заканчивается ответом на клиенте, итоговое время = сумма задержек', () => {
    const last = steps.at(-1)!
    expect(last.state.active).toBe('client')
    expect(last.state.packet?.kind).toBe('response')
    expect(last.state.elapsedMs).toBeCloseTo(TOTAL_MS)
    expect(TOTAL_MS).toBeCloseTo(LATENCY_MS.dnsLookup + 3 * LATENCY_MS.rtt + 4 * LATENCY_MS.internalHop + LATENCY_MS.diskFsync)
  })

  it('у каждого шага есть пояснение; строка кода — только на сервере приложения и в БД', () => {
    for (const step of steps) {
      expect(step.caption.trim().length).toBeGreaterThan(20)
      const onServer = step.state.active === 'app' || step.state.active === 'db'
      if (!onServer) expect(step.codeLine).toBeUndefined()
    }
  })

  it('итоговое время упомянуто в последнем пояснении', () => {
    expect(steps.at(-1)!.caption).toContain('144')
  })
})
```

- [ ] **Step 2: Запустить и убедиться, что тесты падают**

Run: `npm test -- viz/simulations/request-path.test.ts`
Expected: FAIL — `Failed to resolve import "./request-path"`.

- [ ] **Step 3: Реализовать `course/viz/simulations/request-path.ts`**

```ts
import type { Step } from '../core/types'
import { formatMs } from '../core/format'

export type NodeId = 'client' | 'dns' | 'lb' | 'app' | 'db'
export type NodeKind = 'client' | 'network' | 'service' | 'db'

export const NODES: readonly { id: NodeId; label: string; kind: NodeKind }[] = [
  { id: 'client', label: 'Телефон', kind: 'client' },
  { id: 'dns', label: 'DNS', kind: 'network' },
  { id: 'lb', label: 'Балансировщик', kind: 'network' },
  { id: 'app', label: 'Go-сервис', kind: 'service' },
  { id: 'db', label: 'PostgreSQL', kind: 'db' },
]

export interface Packet {
  from: NodeId
  to: NodeId
  kind: 'request' | 'response'
}

export interface RequestPathState {
  active: NodeId
  packet: Packet | null
  elapsedMs: number
}

// Порядки величин для мобильного интернета и сервера в соседней стране.
// Источник диапазонов — урок 0.4 и Grigorik, «High Performance Browser Networking», гл. 1.
export const LATENCY_MS = { dnsLookup: 20, rtt: 40, internalHop: 0.5, diskFsync: 2 } as const

export const TOTAL_MS = LATENCY_MS.dnsLookup + 3 * LATENCY_MS.rtt + 4 * LATENCY_MS.internalHop + LATENCY_MS.diskFsync

// Номера строк в Go-обработчике из урока 0.3.
export const CODE_LINE = { entry: 1, auth: 2, execute: 12, respond: 17 } as const

interface Segment {
  active: NodeId
  packet: Packet | null
  costMs: number
  caption: string
  codeLine?: number
}

const req = (from: NodeId, to: NodeId): Packet => ({ from, to, kind: 'request' })
const res = (from: NodeId, to: NodeId): Packet => ({ from, to, kind: 'response' })
const half = LATENCY_MS.rtt / 2

const SEGMENTS: readonly Segment[] = [
  { active: 'client', packet: null, costMs: 0, caption: 'Вы нажали «Отправить». Чтобы доставить сообщение, телефону нужен IP-адрес сервера, а он знает только имя api.messenger.example.' },
  { active: 'dns', packet: req('client', 'dns'), costMs: LATENCY_MS.dnsLookup / 2, caption: 'DNS-запрос: «какой IP-адрес у api.messenger.example?». Если адрес уже лежит в кэше телефона, этого шага не будет.' },
  { active: 'client', packet: res('dns', 'client'), costMs: LATENCY_MS.dnsLookup / 2, caption: 'DNS вернул IP-адрес балансировщика. Около 20 мс ушло только на то, чтобы узнать, куда стучаться.' },
  { active: 'lb', packet: req('client', 'lb'), costMs: half, caption: 'TCP-рукопожатие, часть 1: телефон отправляет SYN — «хочу установить соединение».' },
  { active: 'client', packet: res('lb', 'client'), costMs: half, caption: 'TCP, часть 2: балансировщик отвечает SYN-ACK. Прошёл один полный RTT, а полезных данных ещё не отправлено ни байта.' },
  { active: 'lb', packet: req('client', 'lb'), costMs: half, caption: 'TLS 1.3: телефон отправляет ACK и сразу ClientHello — поддерживаемые шифры и свою часть ключа.' },
  { active: 'client', packet: res('lb', 'client'), costMs: half, caption: 'Балансировщик отвечает ServerHello и сертификатом. Ещё один RTT — канал зашифрован. Позади два RTT подготовки.' },
  { active: 'lb', packet: req('client', 'lb'), costMs: half, caption: 'Наконец сам запрос: зашифрованный POST /messages с текстом сообщения.' },
  { active: 'app', packet: req('lb', 'app'), costMs: LATENCY_MS.internalHop, codeLine: CODE_LINE.entry, caption: 'Балансировщик расшифровал запрос (TLS-терминация) и передал его одному из Go-сервисов. Внутри дата-центра это доли миллисекунды.' },
  { active: 'app', packet: null, costMs: 0, codeLine: CODE_LINE.auth, caption: 'Go-сервис проверяет токен и разбирает JSON. Это микросекунды работы процессора — на фоне сети почти ноль.' },
  { active: 'db', packet: req('app', 'db'), costMs: LATENCY_MS.internalHop, codeLine: CODE_LINE.execute, caption: 'Сервис отправляет INSERT в PostgreSQL.' },
  { active: 'db', packet: null, costMs: LATENCY_MS.diskFsync, codeLine: CODE_LINE.execute, caption: 'PostgreSQL записывает изменение на диск (fsync) и только потом подтверждает. Так подтверждённое сообщение переживёт сбой питания.' },
  { active: 'app', packet: res('db', 'app'), costMs: LATENCY_MS.internalHop, codeLine: CODE_LINE.respond, caption: 'База подтвердила запись. Сервис формирует ответ 201 Created.' },
  { active: 'lb', packet: res('app', 'lb'), costMs: LATENCY_MS.internalHop, caption: 'Ответ 201 Created возвращается на балансировщик, тот шифрует его для телефона.' },
]

export function requestPathScenario(): Step<RequestPathState>[] {
  let elapsedMs = 0
  const steps: Step<RequestPathState>[] = SEGMENTS.map((segment) => {
    elapsedMs += segment.costMs
    return {
      state: { active: segment.active, packet: segment.packet, elapsedMs },
      caption: segment.caption,
      highlight: [segment.active],
      ...(segment.codeLine === undefined ? {} : { codeLine: segment.codeLine }),
    }
  })
  elapsedMs += half
  const network = LATENCY_MS.dnsLookup + 3 * LATENCY_MS.rtt
  steps.push({
    state: { active: 'client', packet: res('lb', 'client'), elapsedMs },
    caption: `Телефон получил ответ — галочка «отправлено». Итого ≈ ${formatMs(elapsedMs)}, из них ${formatMs(network)} — сеть до телефона, а работа сервера и базы — всего ${formatMs(elapsedMs - network)}.`,
    highlight: ['client'],
  })
  return steps
}
```

- [ ] **Step 4: Запустить тесты**

Run: `npm test -- viz/simulations/request-path.test.ts`
Expected: PASS (7 тестов). Итог 144 мс: 20 + 3 × 40 + 4 × 0,5 + 2.

- [ ] **Step 5: Создать `course/viz/palette.css`**

```css
:root {
  --viz-client: #2563eb;
  --viz-network: #64748b;
  --viz-service: #7c3aed;
  --viz-db: #059669;
  --viz-cache: #d97706;
  --viz-broker: #0891b2;
  --viz-request: #2563eb;
  --viz-response: #059669;
  --viz-error: #dc2626;
  --viz-active: #f59e0b;
  --viz-node-bg: #ffffff;
  --viz-edge: #cbd5e1;
  --viz-text: #1e293b;
  --viz-muted: #64748b;
  --viz-active-line-bg: rgba(245, 158, 11, 0.2);
}

.dark {
  --viz-client: #60a5fa;
  --viz-network: #94a3b8;
  --viz-service: #a78bfa;
  --viz-db: #34d399;
  --viz-cache: #fbbf24;
  --viz-broker: #22d3ee;
  --viz-request: #60a5fa;
  --viz-response: #34d399;
  --viz-error: #f87171;
  --viz-active: #fbbf24;
  --viz-node-bg: #1e293b;
  --viz-edge: #475569;
  --viz-text: #e2e8f0;
  --viz-muted: #94a3b8;
  --viz-active-line-bg: rgba(251, 191, 36, 0.18);
}
```

- [ ] **Step 6: Создать `course/viz/README.md`**

Страницей сайта этот файл не станет: `srcExclude: ['**/README.md']`.

```md
# Визуальный язык курса

Все визуализации используют одни и те же цвета и обозначения. Цвета задаются только через переменные из `palette.css` — для светлой и тёмной темы.

| Сущность | Переменная | Обозначение |
|---|---|---|
| Клиент (телефон, браузер) | `--viz-client` | прямоугольник со скруглением |
| Сетевой узел (DNS, балансировщик) | `--viz-network` | прямоугольник со скруглением |
| Сервис (Go) | `--viz-service` | прямоугольник со скруглением |
| База данных | `--viz-db` | прямоугольник со скруглением |
| Кэш (Redis) | `--viz-cache` | прямоугольник со скруглением |
| Брокер (Kafka) | `--viz-broker` | прямоугольник со скруглением |
| Запрос | `--viz-request` | движущийся круг |
| Ответ | `--viz-response` | движущийся круг |
| Ошибка | `--viz-error` | движущийся круг + красная рамка узла |
| Активный узел | `--viz-active` | толстая рамка |
| Связь | `--viz-edge` | линия |

Правила:
- у каждого шага есть пояснение на русском;
- анимация отключается при `prefers-reduced-motion`;
- текст внутри SVG — не меньше 18 единиц `viewBox`, чтобы оставаться читаемым на ширине 375 px;
- всё, что трогает DOM или GSAP, выполняется только на клиенте (`onMounted`, `watch` без `immediate`).
```

- [ ] **Step 7: Создать `course/viz/components/Player.vue`**

```vue
<script setup lang="ts" generic="S">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { Step } from '../core/types'
import { SPEEDS, initPlayer, keyToAction, playerReducer, stepDelayMs, type PlayerAction } from '../core/player'

const props = defineProps<{ steps: readonly Step<S>[]; label: string }>()

const state = ref(initPlayer(props.steps.length))
const dispatch = (action: PlayerAction): void => {
  state.value = playerReducer(state.value, action)
}
const step = computed(() => props.steps[state.value.index] as Step<S>)
const isFirst = computed(() => state.value.index === 0)
const isLast = computed(() => state.value.index === state.value.total - 1)

watch(() => props.steps, (steps) => dispatch({ type: 'reset', total: steps.length }))

let timer: ReturnType<typeof setTimeout> | undefined
watch(
  () => [state.value.playing, state.value.index, state.value.speed] as const,
  ([playing, , speed]) => {
    clearTimeout(timer)
    if (playing) timer = setTimeout(() => dispatch({ type: 'tick' }), stepDelayMs(speed))
  },
)
onBeforeUnmount(() => clearTimeout(timer))

function onKey(event: KeyboardEvent): void {
  const action = keyToAction(event.key, event.target instanceof HTMLElement ? event.target : null)
  if (action === null) return
  event.preventDefault()
  dispatch(action)
}

function onSeek(event: Event): void {
  dispatch({ type: 'seek', index: Number((event.target as HTMLInputElement).value) - 1 })
}

function onSpeed(event: Event): void {
  const speed = SPEEDS.find((s) => s === Number((event.target as HTMLSelectElement).value))
  if (speed !== undefined) dispatch({ type: 'speed', speed })
}
</script>

<template>
  <figure class="viz-player" tabindex="0" :aria-label="label" @keydown="onKey">
    <div class="viz-stage" :class="{ 'has-code': $slots.code }">
      <div class="viz-canvas"><slot :step="step" :index="state.index" /></div>
      <div v-if="$slots.code" class="viz-code"><slot name="code" :line="step.codeLine" /></div>
    </div>
    <figcaption class="viz-caption" aria-live="polite">
      <span class="viz-step-no">Шаг {{ state.index + 1 }} из {{ state.total }}</span>
      {{ step.caption }}
    </figcaption>
    <div class="viz-controls" role="group" aria-label="Управление визуализацией">
      <button type="button" :disabled="isFirst" aria-label="В начало" @click="dispatch({ type: 'first' })">⏮</button>
      <button type="button" :disabled="isFirst" aria-label="Шаг назад" @click="dispatch({ type: 'prev' })">◀</button>
      <button type="button" class="viz-play" :aria-label="state.playing ? 'Пауза' : 'Воспроизвести'" @click="dispatch({ type: 'toggle' })">
        {{ state.playing ? '⏸' : '▶' }}
      </button>
      <button type="button" :disabled="isLast" aria-label="Шаг вперёд" @click="dispatch({ type: 'next' })">▶▏</button>
      <button type="button" :disabled="isLast" aria-label="В конец" @click="dispatch({ type: 'last' })">⏭</button>
      <input class="viz-progress" type="range" min="1" :max="state.total" :value="state.index + 1" aria-label="Перейти к шагу" @input="onSeek" />
      <label class="viz-speed">
        Скорость
        <select :value="state.speed" @change="onSpeed">
          <option v-for="s in SPEEDS" :key="s" :value="s">{{ String(s).replace('.', ',') }}×</option>
        </select>
      </label>
    </div>
    <p class="viz-hint">Клавиши: ← → — шаги, пробел — пуск и пауза, Home / End — в начало и в конец.</p>
  </figure>
</template>

<style scoped>
.viz-player { margin: 24px 0; padding: 16px; border: 1px solid var(--vp-c-divider); border-radius: 12px; background: var(--vp-c-bg-soft); }
.viz-player:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
.viz-stage { display: grid; gap: 16px; }
@media (min-width: 960px) { .viz-stage.has-code { grid-template-columns: 3fr 2fr; } }
.viz-canvas, .viz-code { min-width: 0; }
.viz-caption { min-height: 4.5em; margin-top: 12px; line-height: 1.6; }
.viz-step-no { display: block; color: var(--vp-c-text-2); font-size: 13px; }
.viz-controls { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 12px; }
.viz-controls button { min-width: 40px; min-height: 40px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); }
.viz-controls button:disabled { opacity: 0.4; cursor: not-allowed; }
.viz-controls .viz-play { border-color: var(--vp-c-brand-1); color: var(--vp-c-brand-1); }
.viz-progress { flex: 1 1 140px; min-width: 0; }
.viz-speed { display: flex; align-items: center; gap: 6px; font-size: 14px; }
.viz-speed select { padding: 4px 6px; border: 1px solid var(--vp-c-divider); border-radius: 6px; background: var(--vp-c-bg); }
.viz-hint { margin: 8px 0 0; color: var(--vp-c-text-3); font-size: 12px; }
</style>
```

- [ ] **Step 8: Создать `course/viz/components/CodePanel.vue`**

VitePress отрисовывает блок кода через Shiki и каждую строку оборачивает в `span.line`. Панель лишь переключает класс у нужной строки.

```vue
<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'

const props = defineProps<{ line?: number }>()
const root = ref<HTMLElement>()

function highlight(): void {
  const lines = root.value?.querySelectorAll<HTMLElement>('span.line') ?? []
  lines.forEach((el, i) => el.classList.toggle('viz-active-line', i + 1 === props.line))
}

onMounted(highlight)
watch(() => props.line, () => nextTick(highlight))
</script>

<template>
  <div ref="root" class="viz-code-panel"><slot /></div>
</template>

<style scoped>
.viz-code-panel :deep(div[class*='language-']) { margin: 0; }
.viz-code-panel :deep(span.line) { display: inline-block; width: 100%; transition: background-color 0.2s; }
.viz-code-panel :deep(span.line.viz-active-line) { background: var(--viz-active-line-bg); box-shadow: inset 3px 0 0 var(--viz-active); }
</style>
```

- [ ] **Step 9: Создать `course/viz/renderers/RequestPathView.vue`**

```vue
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { gsap } from 'gsap'
import { NODES, type NodeId, type NodeKind, type RequestPathState } from '../simulations/request-path'
import { formatMs } from '../core/format'
import { prefersReducedMotion } from '../core/motion'

const props = defineProps<{ state: RequestPathState }>()

const W = 140
const H = 56
const POS: Record<NodeId, { x: number; y: number }> = {
  client: { x: 90, y: 230 },
  dns: { x: 90, y: 80 },
  lb: { x: 300, y: 230 },
  app: { x: 510, y: 230 },
  db: { x: 720, y: 230 },
}
const EDGES: readonly [NodeId, NodeId][] = [['client', 'dns'], ['client', 'lb'], ['lb', 'app'], ['app', 'db']]
const KIND_COLOR: Record<NodeKind, string> = {
  client: 'var(--viz-client)',
  network: 'var(--viz-network)',
  service: 'var(--viz-service)',
  db: 'var(--viz-db)',
}

const packetEl = ref<SVGCircleElement>()

function animatePacket(): void {
  const el = packetEl.value
  const packet = props.state.packet
  if (!el || !packet) return
  const from = POS[packet.from]
  const to = POS[packet.to]
  // Быстрые клики не должны накапливать анимации: прерываем предыдущую.
  gsap.killTweensOf(el)
  if (prefersReducedMotion()) {
    gsap.set(el, { attr: { cx: to.x, cy: to.y } })
    return
  }
  gsap.fromTo(el, { attr: { cx: from.x, cy: from.y } }, { attr: { cx: to.x, cy: to.y }, duration: 0.7, ease: 'power1.inOut' })
}

onMounted(animatePacket)
watch(() => props.state, () => requestAnimationFrame(animatePacket))
onBeforeUnmount(() => {
  if (packetEl.value) gsap.killTweensOf(packetEl.value)
})
</script>

<template>
  <svg class="rp" viewBox="0 0 810 300" role="img" aria-label="Схема: телефон, DNS, балансировщик, Go-сервис, PostgreSQL">
    <line v-for="[a, b] in EDGES" :key="`${a}-${b}`" :x1="POS[a].x" :y1="POS[a].y" :x2="POS[b].x" :y2="POS[b].y" class="edge" />
    <g v-for="node in NODES" :key="node.id" :transform="`translate(${POS[node.id].x - W / 2}, ${POS[node.id].y - H / 2})`">
      <rect :width="W" :height="H" rx="10" class="node" :class="{ active: state.active === node.id }" :style="{ stroke: state.active === node.id ? 'var(--viz-active)' : KIND_COLOR[node.kind] }" />
      <text :x="W / 2" :y="H / 2" class="label">{{ node.label }}</text>
    </g>
    <circle v-if="state.packet" ref="packetEl" r="11" :cx="POS[state.packet.to].x" :cy="POS[state.packet.to].y" class="packet" :class="state.packet.kind" />
    <text x="800" y="36" class="clock">⏱ {{ formatMs(state.elapsedMs) }}</text>
  </svg>
</template>

<style scoped>
.rp { width: 100%; height: auto; display: block; }
.edge { stroke: var(--viz-edge); stroke-width: 3; }
.node { fill: var(--viz-node-bg); stroke-width: 3; transition: stroke 0.2s, stroke-width 0.2s; }
.node.active { stroke-width: 6; }
.label { fill: var(--viz-text); font-size: 20px; text-anchor: middle; dominant-baseline: central; }
.packet.request { fill: var(--viz-request); }
.packet.response { fill: var(--viz-response); }
.clock { fill: var(--viz-text); font-size: 26px; font-weight: 600; text-anchor: end; font-variant-numeric: tabular-nums; }
</style>
```

- [ ] **Step 10: Создать сцену `course/viz/scenes/RequestPath.vue`**

```vue
<script setup lang="ts">
import Player from '../components/Player.vue'
import CodePanel from '../components/CodePanel.vue'
import RequestPathView from '../renderers/RequestPathView.vue'
import { requestPathScenario } from '../simulations/request-path'

const steps = requestPathScenario()
</script>

<template>
  <Player :steps="steps" label="Путь запроса «Отправить сообщение»">
    <template #default="{ step }">
      <RequestPathView :state="step.state" />
    </template>
    <template v-if="$slots.default" #code="{ line }">
      <CodePanel :line="line"><slot /></CodePanel>
    </template>
  </Player>
</template>
```

- [ ] **Step 11: Подключить палитру в теме**

В `course/.vitepress/theme/index.ts` добавить строку `import '../../viz/palette.css'` после импортов компонентов.

- [ ] **Step 12: Написать урок `03-request-path.md`**

Frontmatter: `title: 0.3. Путь запроса: что происходит, когда вы нажимаете «Отправить»`.

Подключение сцены — сразу после frontmatter:

```md
<script setup>
import RequestPath from '../../viz/scenes/RequestPath.vue'
</script>
```

Визуализация вставляется так. Пустые строки вокруг блока кода обязательны: без них Markdown внутри компонента не обработается.

````md
<RequestPath>

```go
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.UserID(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	msg, err := h.send.Execute(r.Context(), userID, req.ChatID, req.Text)
	if err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toResponse(msg))
}
```

</RequestPath>
````

Строки 1, 2, 12 и 17 этого фрагмента должны совпадать с `CODE_LINE` в `request-path.ts`. Если фрагмент меняется, обновить `CODE_LINE`.

Обязательные разделы урока:

1. **Проблема.** «Код обработчика работает 1 мс, а пользователь ждёт 150 мс. Куда уходит время?»
2. **Участники.** Статичная Mermaid-схема структуры (`flowchart LR`: Телефон → DNS; Телефон → Балансировщик → Go-сервис → PostgreSQL). Кратко о каждом: <T id="dns">DNS</T>, <T id="ip-address">IP-адрес</T>, <T id="load-balancer">балансировщик</T>, <T id="application-server">сервер приложения</T>, <T id="database">база данных</T>.
3. **Визуализация.** `<RequestPath>` с Go-кодом (см. выше) и фраза-инструкция: «пройдите все шаги, следите за часами в правом верхнем углу».
4. **Разбор по шагам** (текстом, по подразделам):
   - DNS и кэш (TTL);
   - <T id="tcp">TCP</T> и рукопожатие — что гарантирует TCP и почему ему нужен <T id="rtt">RTT</T> до данных;
   - <T id="tls">TLS 1.3</T> — 1 RTT, у TLS 1.2 — 2; упомянуть 0-RTT с оговоркой о повторе запросов;
   - <T id="http">HTTP</T>-запрос;
   - <T id="tls-termination">TLS-терминация</T> на балансировщике;
   - обработчик Go (ссылка вперёд на модуль 1 — что такое `h.send.Execute` и зачем слои);
   - запись в БД и <T id="fsync">fsync</T>; связь с <T id="durability">долговечностью</T> из урока 0.2;
   - ответ.
5. **Таблица допущений.** Значения `LATENCY_MS` с пояснением, откуда порядки (мобильная сеть, соседняя страна). Пометка: «это порядки величин, а не измерения».
6. **Главный вывод.** <Callout type="idea">:
   - 140 из 144 мс — сеть до клиента;
   - поэтому соединения переиспользуют (keep-alive), и второе сообщение по открытому соединению стоит ≈ 1 RTT + 4 мс ≈ 44 мс;
   - ссылки вперёд на модуль 0b (сеть) и модуль 3 (WebSocket держит соединение открытым).
7. **Типичные ошибки.** <Callout type="mistake">:
   - оптимизировать код, когда тормозит сеть;
   - забыть про TTL DNS при переезде сервера.
8. **Компромисс.** <Callout type="tradeoff">: TLS-терминация на балансировщике.
   - плюсы: сертификаты в одном месте, разгрузка сервисов;
   - минус: внутри ЦОД трафик идёт без шифрования, если не принять мер (mTLS — модуль 6.7).
9. **Самопроверка.** 4 `<SelfCheck>`:
   - «Сколько займёт второе сообщение по уже открытому соединению и почему?» (≈ 44 мс: 1 RTT + внутренние 4 мс);
   - «Что изменится с TLS 1.2 вместо 1.3?» (+1 RTT ≈ +40 мс);
   - «Зачем БД делает fsync до подтверждения и чем она за это платит?»;
   - «Сервер переехал на новый IP, а часть пользователей ходит на старый. Почему?» (кэш DNS, TTL).
10. **Источники.**
    - Kurose & Ross, «Computer Networking: A Top-Down Approach», гл. 1–3;
    - I. Grigorik, «High Performance Browser Networking», гл. 1–4 (бесплатно: https://hpbn.co);
    - RFC 8446 (TLS 1.3).

- [ ] **Step 13: Проверить**

Run: `npm test && npm run typecheck && npm run build`
Expected: PASS, без ошибок типов, `build complete`.

Затем `npm run dev`, открыть урок 0.3 и вручную проверить:
- пакет «летит» между узлами;
- часы растут до 144 мс;
- на шагах сервиса и БД в коде подсвечивается нужная строка (1, 2, 12, 17);
- ← → и пробел работают, когда плеер в фокусе (Tab);
- пробел на кнопке «Шаг вперёд» делает ровно один шаг;
- автопроигрывание доходит до конца и останавливается;
- в тёмной теме всё контрастно;
- при эмуляции `prefers-reduced-motion` (DevTools → Rendering) пакет появляется сразу, без полёта.

Затем пройти чек-листы 2.2 и 2.3 по уроку.

- [ ] **Step 14: Commit**

```bash
git add course/viz course/.vitepress/theme/index.ts course/modules/00-foundation/03-request-path.md
git commit -m "feat(viz): плеер, панель кода, визуализация пути запроса и урок 0.3

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Шкала задержек и урок 0.4

**Files:**
- Create: `course/viz/simulations/latency.ts`, `course/viz/simulations/latency.test.ts`, `course/viz/scenes/LatencyScale.vue`, `course/modules/00-foundation/04-latency.md`

**Interfaces:**
- Consumes: `formatNs`, `formatSeconds`, `formatNumber` (задача 5); `--viz-*` (задача 6).
- Produces:
  - `LatencyKind`, `LatencyItem`, `LATENCIES`, `LATENCY_SOURCE`, `MIN_BAR`;
  - `barWidth(ns: number): number` — доля 0..1 на логарифмической шкале;
  - `humanSeconds(ns: number): number`;
  - `compare(item: LatencyItem, baseline: LatencyItem): string`;
  - сцена `<LatencyScale />`.

- [ ] **Step 1: Написать падающие тесты `course/viz/simulations/latency.test.ts`**

```ts
import { describe, expect, it } from 'vitest'
import { LATENCIES, MIN_BAR, barWidth, compare, humanSeconds } from './latency'

const byId = (id: string) => LATENCIES.find((l) => l.id === id)!

describe('LATENCIES', () => {
  it('отсортированы по возрастанию, id уникальны', () => {
    const ns = LATENCIES.map((l) => l.ns)
    expect(ns).toEqual([...ns].sort((a, b) => a - b))
    expect(new Set(LATENCIES.map((l) => l.id)).size).toBe(LATENCIES.length)
  })
})

describe('barWidth', () => {
  it('минимум для самого быстрого, 1 для самого медленного, монотонно растёт', () => {
    const widths = LATENCIES.map((l) => barWidth(l.ns))
    expect(widths[0]).toBe(MIN_BAR)
    expect(widths.at(-1)).toBe(1)
    for (let i = 1; i < widths.length; i++) expect(widths[i]!).toBeGreaterThan(widths[i - 1]!)
  })
})

describe('humanSeconds', () => {
  it('1 нс превращается в 1 с', () => {
    expect(humanSeconds(1)).toBe(1)
    expect(humanSeconds(150_000_000)).toBe(150_000_000)
  })
})

describe('compare', () => {
  it.each([
    ['ssd-random', 'ram', 'в 160 раз медленнее'],
    ['l1', 'ram', 'в 100 раз быстрее'],
    ['ram', 'ram', 'это и есть точка отсчёта'],
    ['intercontinental', 'dc-rtt', 'в 300 раз медленнее'],
  ])('%s относительно %s → %s', (a, b, expected) => {
    expect(compare(byId(a), byId(b))).toBe(expected)
  })
})
```

- [ ] **Step 2: Запустить и убедиться, что тесты падают**

Run: `npm test -- viz/simulations/latency.test.ts`
Expected: FAIL — `Failed to resolve import "./latency"`.

- [ ] **Step 3: Реализовать `course/viz/simulations/latency.ts`**

```ts
import { formatNumber } from '../core/format'

export type LatencyKind = 'cpu' | 'memory' | 'disk' | 'network'

export interface LatencyItem {
  id: string
  label: string
  ns: number
  kind: LatencyKind
}

// Порядки величин по Colin Scott (оценки на 2020 г., продолжение таблицы Jeff Dean / Peter Norvig).
// Это ориентиры, а не измерения конкретного железа.
export const LATENCIES: readonly LatencyItem[] = [
  { id: 'l1', label: 'Обращение к кэшу L1 процессора', ns: 1, kind: 'cpu' },
  { id: 'l2', label: 'Обращение к кэшу L2', ns: 4, kind: 'cpu' },
  { id: 'ram', label: 'Обращение к оперативной памяти', ns: 100, kind: 'memory' },
  { id: 'ram-1mb', label: 'Последовательно прочитать 1 МБ из памяти', ns: 3_000, kind: 'memory' },
  { id: 'ssd-random', label: 'Случайное чтение 4 КБ с SSD', ns: 16_000, kind: 'disk' },
  { id: 'ssd-1mb', label: 'Последовательно прочитать 1 МБ с SSD', ns: 49_000, kind: 'disk' },
  { id: 'dc-rtt', label: 'Round trip внутри одного дата-центра', ns: 500_000, kind: 'network' },
  { id: 'hdd-1mb', label: 'Последовательно прочитать 1 МБ с HDD', ns: 825_000, kind: 'disk' },
  { id: 'hdd-seek', label: 'Позиционирование головки HDD', ns: 2_000_000, kind: 'disk' },
  { id: 'intercontinental', label: 'Round trip Калифорния → Нидерланды → Калифорния', ns: 150_000_000, kind: 'network' },
]

export const LATENCY_SOURCE = {
  title: 'Colin Scott, «Latency Numbers Every Programmer Should Know» (оценки на 2020 г.)',
  url: 'https://colin-scott.github.io/personal_website/research/interactive_latency.html',
} as const

// Ненулевая минимальная ширина, чтобы самая короткая полоса была видна.
export const MIN_BAR = 0.02

const logMin = Math.log10(LATENCIES[0]!.ns)
const logMax = Math.log10(LATENCIES.at(-1)!.ns)

export function barWidth(ns: number): number {
  const share = (Math.log10(ns) - logMin) / (logMax - logMin)
  return MIN_BAR + (1 - MIN_BAR) * Math.min(Math.max(share, 0), 1)
}

// «Человеческий масштаб»: 1 нс реального времени = 1 с ощущаемого.
const HUMAN_SECONDS_PER_NS = 1

export function humanSeconds(ns: number): number {
  return ns * HUMAN_SECONDS_PER_NS
}

export function compare(item: LatencyItem, baseline: LatencyItem): string {
  if (item.id === baseline.id) return 'это и есть точка отсчёта'
  const ratio = item.ns / baseline.ns
  return ratio >= 1
    ? `в ${formatNumber(ratio, ratio < 10 ? 1 : 0)} раз медленнее`
    : `в ${formatNumber(1 / ratio, 1 / ratio < 10 ? 1 : 0)} раз быстрее`
}
```

- [ ] **Step 4: Запустить тесты**

Run: `npm test -- viz/simulations/latency.test.ts`
Expected: PASS.

- [ ] **Step 5: Создать сцену `course/viz/scenes/LatencyScale.vue`**

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { LATENCIES, LATENCY_SOURCE, barWidth, compare, humanSeconds, type LatencyKind } from '../simulations/latency'
import { formatNs, formatSeconds } from '../core/format'

type Mode = 'real' | 'human'
const mode = ref<Mode>('real')
const baselineId = ref('ram')
const baseline = computed(() => LATENCIES.find((l) => l.id === baselineId.value) ?? LATENCIES[0]!)

const KIND_COLOR: Record<LatencyKind, string> = {
  cpu: 'var(--viz-service)',
  memory: 'var(--viz-cache)',
  disk: 'var(--viz-db)',
  network: 'var(--viz-network)',
}
const KIND_LABEL: Record<LatencyKind, string> = { cpu: 'процессор', memory: 'память', disk: 'диск', network: 'сеть' }

const value = (ns: number): string => (mode.value === 'real' ? formatNs(ns) : formatSeconds(humanSeconds(ns)))
</script>

<template>
  <section class="latency" aria-label="Шкала задержек">
    <div class="toolbar">
      <div class="modes" role="radiogroup" aria-label="Масштаб">
        <button type="button" role="radio" :aria-checked="mode === 'real'" :class="{ on: mode === 'real' }" @click="mode = 'real'">Реальное время</button>
        <button type="button" role="radio" :aria-checked="mode === 'human'" :class="{ on: mode === 'human' }" @click="mode = 'human'">Человеческий масштаб: 1 нс = 1 с</button>
      </div>
      <label class="baseline">
        Сравнивать с:
        <select v-model="baselineId">
          <option v-for="l in LATENCIES" :key="l.id" :value="l.id">{{ l.label }}</option>
        </select>
      </label>
    </div>
    <ol class="rows">
      <li v-for="l in LATENCIES" :key="l.id" :class="{ base: l.id === baselineId }">
        <div class="row-head">
          <span class="label">{{ l.label }}</span>
          <strong class="value">{{ value(l.ns) }}</strong>
        </div>
        <div class="track" aria-hidden="true">
          <div class="bar" :style="{ width: `${barWidth(l.ns) * 100}%`, background: KIND_COLOR[l.kind] }" />
        </div>
        <span class="meta">{{ KIND_LABEL[l.kind] }} · {{ compare(l, baseline) }}</span>
      </li>
    </ol>
    <p class="note">
      Длина полос — в логарифмической шкале: каждые ~12% ширины означают ×10. Значения — порядки величин, источник:
      <a :href="LATENCY_SOURCE.url" target="_blank" rel="noopener">{{ LATENCY_SOURCE.title }}</a>.
    </p>
  </section>
</template>

<style scoped>
.latency { margin: 24px 0; padding: 16px; border: 1px solid var(--vp-c-divider); border-radius: 12px; background: var(--vp-c-bg-soft); }
.toolbar { display: flex; flex-wrap: wrap; gap: 12px; justify-content: space-between; align-items: center; }
.modes { display: flex; flex-wrap: wrap; gap: 6px; }
.modes button { padding: 6px 12px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); font-size: 14px; }
.modes button.on { border-color: var(--vp-c-brand-1); color: var(--vp-c-brand-1); background: var(--vp-c-brand-soft); }
.baseline { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; font-size: 14px; }
.baseline select { max-width: 100%; padding: 4px 6px; border: 1px solid var(--vp-c-divider); border-radius: 6px; background: var(--vp-c-bg); }
.rows { list-style: none; padding: 0; margin: 16px 0 0; }
.rows li { margin: 0 0 14px; padding: 6px 8px; border-radius: 8px; }
.rows li.base { background: var(--viz-active-line-bg); }
.row-head { display: flex; justify-content: space-between; gap: 12px; font-size: 14px; }
.value { font-variant-numeric: tabular-nums; white-space: nowrap; }
.track { height: 10px; margin: 6px 0 4px; border-radius: 5px; background: var(--vp-c-divider); }
.bar { height: 100%; border-radius: 5px; transition: width 0.3s; }
.meta { color: var(--vp-c-text-2); font-size: 12px; }
.note { color: var(--vp-c-text-2); font-size: 13px; }
@media (prefers-reduced-motion: reduce) { .bar { transition: none; } }
</style>
```

Откуда «~12%»: диапазон 1 нс … 150 мс — это 8,2 порядка, а 1/8,2 ≈ 12% ширины. Если `LATENCIES` изменится, число в подписи пересчитать.

- [ ] **Step 6: Написать урок `04-latency.md`**

Frontmatter: `title: 0.4. Задержки и пропускная способность`.

Подключение сцены: `<script setup>import LatencyScale from '../../viz/scenes/LatencyScale.vue'</script>`.

Обязательные разделы:

1. **Проблема.** «Почему кэш в Redis ускоряет ответ в десятки раз, а перенос сервера на другой континент делает приложение “ватным”?»
2. **Три слова, которые путают.** <T id="latency">задержка</T>, <T id="throughput">пропускная способность</T>, <T id="bandwidth">ширина канала</T>. Аналогия с шоссе:
   - задержка — время в пути одной машины;
   - ширина канала — число полос;
   - throughput — сколько машин реально проехало за час.

   Где аналогия ломается: в сети при перегрузке растёт и задержка (очереди).
3. **Визуализация.** `<LatencyScale />`. Перед ней — объяснение <T id="log-scale">логарифмической шкалы</T>. После — три задания:
   - переключитесь в «человеческий масштаб» и найдите, сколько «длится» запрос через океан (≈ 4,8 года против 1 с у L1);
   - выберите точкой отсчёта RAM и посмотрите, во сколько раз медленнее SSD;
   - сравните round trip внутри ЦОД и через океан.
4. **Что из этого следует для бэкенда:**
   - <T id="cpu-cache">кэш процессора</T> и <T id="ram">RAM</T> против <T id="ssd">SSD</T> и <T id="hdd">HDD</T> — почему базы держат горячие данные в памяти;
   - последовательное чтение против случайного — зачем журналы (WAL, Kafka) пишут последовательно (ссылки вперёд на модули 2b и 6.5);
   - внутри <T id="data-center">дата-центра</T> сеть дешёвая, между континентами — дорогая.
5. **Скорость света как нижняя граница.**
   - ~200 000 км/с в оптоволокне;
   - 9 000 км (Москва — Нью-Йорк ≈ 7 500 км по прямой, кабель длиннее) → не меньше ~45 мс в одну сторону, ~90 мс RTT;
   - вывод: близость к пользователю (CDN, регионы) — ссылка на модуль 11.
6. **<T id="littles-law">Закон Литтла</T>.** L = λ × W.
   - пример: 1 000 RPS × 0,1 с = 100 одновременных запросов;
   - если W вырастет до 1 с из-за медленной БД, одновременных станет 1 000, и пул из 100 соединений закончится;
   - связь с перцентилями из 0.2.
7. **Компромисс.** <Callout type="tradeoff">: батчинг — throughput растёт, задержка отдельной операции тоже (ссылка вперёд на Kafka).
8. **Типичные ошибки.** <Callout type="mistake">:
   - «купим канал толще — станет быстрее»;
   - считать среднее вместо перцентилей;
   - верить числам из таблицы как точным: на своём железе нужно мерить.
9. **Самопроверка.** 4 `<SelfCheck>`:
   - «Почему увеличение ширины канала не ускоряет одиночный маленький запрос через океан?»;
   - «Сервис получает 2 000 RPS, ответ занимает 50 мс. Сколько запросов обрабатывается одновременно?» (100);
   - «Во сколько раз случайное чтение с SSD медленнее обращения к RAM?» (~160);
   - «Почему базы данных пишут журнал последовательно?»
10. **Источники.**
    - Colin Scott (ссылка);
    - Jeff Dean, «Numbers Everyone Should Know» (LADIS 2009);
    - Клеппман, гл. 1;
    - Grigorik, гл. 1;
    - J. D. C. Little, «A Proof for the Queuing Formula: L = λW» (1961).

- [ ] **Step 7: Проверить**

Run: `npm test && npm run typecheck && npm run build`
Expected: PASS, `build complete`.

Затем вручную в dev:
- переключение режимов;
- смена точки отсчёта подсвечивает строку и пересчитывает «в N раз»;
- на ширине 375 px подписи переносятся, страница не прокручивается по горизонтали.

Затем чек-листы 2.2 и 2.3.

- [ ] **Step 8: Commit**

```bash
git add course/viz/simulations/latency.ts course/viz/simulations/latency.test.ts course/viz/scenes/LatencyScale.vue course/modules/00-foundation/04-latency.md
git commit -m "feat(viz): шкала задержек и урок 0.4

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Калькулятор оценок и уроки 0.5–0.6

**Files:**
- Create: `course/viz/simulations/estimation.ts`, `course/viz/simulations/estimation.test.ts`, `course/viz/scenes/EstimationCalculator.vue`, `course/modules/00-foundation/05-estimation.md`, `course/modules/00-foundation/06-messenger-requirements.md`

**Interfaces:**
- Consumes: `formatNumber`, `formatBytes` (задача 5).
- Produces:
  - `EstimationInput`, `EstimationStep`, `PresetId`, `PRESETS`, `SECONDS_PER_DAY`;
  - `sanitizeInput(raw: EstimationInput): EstimationInput`;
  - `estimate(raw: EstimationInput): EstimationStep[]`;
  - `sliderToDau(position: number): number`, `dauToSlider(dau: number): number`;
  - `formatStepValue(step: EstimationStep): string`;
  - сцена `<EstimationCalculator preset="default|messenger" />`.

- [ ] **Step 1: Написать падающие тесты `course/viz/simulations/estimation.test.ts`**

```ts
import { describe, expect, it } from 'vitest'
import { PRESETS, dauToSlider, estimate, formatStepValue, sanitizeInput, sliderToDau, type EstimationInput } from './estimation'

const valueOf = (input: EstimationInput, id: string) => estimate(input).find((s) => s.id === id)!.value

describe('estimate: пресет мессенджера', () => {
  const m = PRESETS.messenger
  it.each([
    ['messages-per-day', 4e9],
    ['avg-rps', 4e9 / 86_400],
    ['peak-rps', (4e9 / 86_400) * 3],
    ['storage-per-day', 8e11],
    ['storage-total', 8e11 * 1825 * 3],
    ['peak-ingress', (4e9 / 86_400) * 3 * 200],
  ])('%s = %s', (id, expected) => {
    expect(valueOf(m, id)).toBeCloseTo(expected, 3)
  })

  it('шаги идут в фиксированном порядке', () => {
    expect(estimate(m).map((s) => s.id)).toEqual(['messages-per-day', 'avg-rps', 'peak-rps', 'storage-per-day', 'storage-total', 'peak-ingress'])
  })

  it('выражение показывает подставленные числа', () => {
    expect(estimate(m)[0]!.expression).toBe('100 000 000 × 40 = 4 000 000 000')
  })
})

describe('sanitizeInput', () => {
  it('отрицательные, NaN и бесконечность → 0; коэффициенты пика и репликации не меньше 1', () => {
    expect(
      sanitizeInput({ dau: -5, messagesPerUser: Number.NaN, messageBytes: Number.POSITIVE_INFINITY, peakFactor: 0.5, retentionDays: -1, replicationFactor: 0 }),
    ).toEqual({ dau: 0, messagesPerUser: 0, messageBytes: 0, peakFactor: 1, retentionDays: 0, replicationFactor: 1 })
  })

  it('нулевая аудитория даёт нули, а не NaN', () => {
    for (const step of estimate({ ...PRESETS.default, dau: 0 })) {
      expect(step.value).toBe(0)
      expect(formatStepValue(step)).not.toMatch(/NaN|Infinity/)
    }
  })
})

describe('слайдер DAU (логарифмический)', () => {
  it.each([
    [3, 1_000],
    [6, 1_000_000],
    [9, 1_000_000_000],
  ])('позиция %s → %s', (pos, dau) => {
    expect(sliderToDau(pos)).toBe(dau)
    expect(dauToSlider(dau)).toBeCloseTo(pos)
  })

  it('округляет до двух значащих цифр', () => {
    expect(sliderToDau(7.5)).toBe(32_000_000)
  })

  it('dauToSlider зажимает ноль и отрицательные к минимуму', () => {
    expect(dauToSlider(0)).toBe(3)
  })
})

describe('formatStepValue', () => {
  it.each([
    ['messages-per-day', '4 000 000 000 сообщ./день'],
    ['avg-rps', '46 296 RPS'],
    ['storage-total', '4,4 ПБ'],
    ['peak-ingress', '28 МБ/с'],
  ])('%s → %s', (id, expected) => {
    const step = estimate(PRESETS.messenger).find((s) => s.id === id)!
    expect(formatStepValue(step)).toBe(expected)
  })
})
```

- [ ] **Step 2: Запустить и убедиться, что тесты падают**

Run: `npm test -- viz/simulations/estimation.test.ts`
Expected: FAIL — `Failed to resolve import "./estimation"`.

- [ ] **Step 3: Реализовать `course/viz/simulations/estimation.ts`**

```ts
import { formatBytes, formatNumber } from '../core/format'

export interface EstimationInput {
  dau: number
  messagesPerUser: number
  messageBytes: number
  peakFactor: number
  retentionDays: number
  replicationFactor: number
}

export type StepId = 'messages-per-day' | 'avg-rps' | 'peak-rps' | 'storage-per-day' | 'storage-total' | 'peak-ingress'
export type StepUnit = 'messages' | 'rps' | 'bytes' | 'bytes-per-sec'

export interface EstimationStep {
  id: StepId
  label: string
  formula: string
  expression: string
  value: number
  unit: StepUnit
}

export const SECONDS_PER_DAY = 86_400

export type PresetId = 'default' | 'messenger'

export const PRESETS: Record<PresetId, EstimationInput> = {
  default: { dau: 1_000_000, messagesPerUser: 20, messageBytes: 200, peakFactor: 2, retentionDays: 365, replicationFactor: 3 },
  // Обоснование каждого числа — в уроке 0.6.
  messenger: { dau: 100_000_000, messagesPerUser: 40, messageBytes: 200, peakFactor: 3, retentionDays: 1825, replicationFactor: 3 },
}

function nonNegative(value: number): number {
  return Number.isFinite(value) && value > 0 ? value : 0
}

export function sanitizeInput(raw: EstimationInput): EstimationInput {
  return {
    dau: nonNegative(raw.dau),
    messagesPerUser: nonNegative(raw.messagesPerUser),
    messageBytes: nonNegative(raw.messageBytes),
    peakFactor: Math.max(nonNegative(raw.peakFactor), 1),
    retentionDays: nonNegative(raw.retentionDays),
    replicationFactor: Math.max(nonNegative(raw.replicationFactor), 1),
  }
}

const n = (value: number): string => formatNumber(value, value < 10 ? 1 : 0)

export function estimate(raw: EstimationInput): EstimationStep[] {
  const i = sanitizeInput(raw)
  const perDay = i.dau * i.messagesPerUser
  const avgRps = perDay / SECONDS_PER_DAY
  const peakRps = avgRps * i.peakFactor
  const storageDay = perDay * i.messageBytes
  const storageTotal = storageDay * i.retentionDays * i.replicationFactor
  const peakIngress = peakRps * i.messageBytes
  return [
    { id: 'messages-per-day', label: 'Сообщений в день', formula: 'DAU × сообщений на пользователя', expression: `${n(i.dau)} × ${n(i.messagesPerUser)} = ${n(perDay)}`, value: perDay, unit: 'messages' },
    { id: 'avg-rps', label: 'Средняя нагрузка на запись', formula: 'сообщений в день ÷ 86 400 секунд', expression: `${n(perDay)} ÷ ${n(SECONDS_PER_DAY)} ≈ ${n(avgRps)}`, value: avgRps, unit: 'rps' },
    { id: 'peak-rps', label: 'Пиковая нагрузка на запись', formula: 'средняя × коэффициент пика', expression: `${n(avgRps)} × ${n(i.peakFactor)} ≈ ${n(peakRps)}`, value: peakRps, unit: 'rps' },
    { id: 'storage-per-day', label: 'Новых данных в день', formula: 'сообщений в день × размер сообщения', expression: `${n(perDay)} × ${n(i.messageBytes)} Б = ${formatBytes(storageDay)}`, value: storageDay, unit: 'bytes' },
    { id: 'storage-total', label: 'Хранилище за весь срок с репликами', formula: 'в день × дней хранения × коэффициент репликации', expression: `${formatBytes(storageDay)} × ${n(i.retentionDays)} × ${n(i.replicationFactor)} = ${formatBytes(storageTotal)}`, value: storageTotal, unit: 'bytes' },
    { id: 'peak-ingress', label: 'Входящий трафик в пик', formula: 'пиковый RPS × размер сообщения', expression: `${n(peakRps)} × ${n(i.messageBytes)} Б ≈ ${formatBytes(peakIngress)}/с`, value: peakIngress, unit: 'bytes-per-sec' },
  ]
}

export function formatStepValue(step: EstimationStep): string {
  switch (step.unit) {
    case 'messages':
      return `${formatNumber(step.value)} сообщ./день`
    case 'rps':
      return `${formatNumber(step.value, step.value < 10 ? 1 : 0)} RPS`
    case 'bytes':
      return formatBytes(step.value)
    case 'bytes-per-sec':
      return `${formatBytes(step.value)}/с`
  }
}

// Аудитория меняется на порядки (тысяча … миллиард), поэтому слайдер логарифмический:
// позиция — степень десяти от 3 до 9.
export const DAU_SLIDER = { min: 3, max: 9, step: 0.1 } as const

export function sliderToDau(position: number): number {
  return Number((10 ** position).toPrecision(2))
}

export function dauToSlider(dau: number): number {
  return dau > 0 ? Math.min(Math.max(Math.log10(dau), DAU_SLIDER.min), DAU_SLIDER.max) : DAU_SLIDER.min
}
```

- [ ] **Step 4: Запустить тесты**

Run: `npm test -- viz/simulations/estimation.test.ts`
Expected: PASS.

Проверка контрольных значений:
- `46 296` RPS = 4·10⁹ / 86 400;
- 4,38·10¹⁵ Б → «4,4 ПБ»;
- пик: 138 888,9 × 200 = 2,78·10⁷ Б/с → «28 МБ/с».

- [ ] **Step 5: Создать сцену `course/viz/scenes/EstimationCalculator.vue`**

```vue
<script setup lang="ts">
import { computed, reactive } from 'vue'
import { DAU_SLIDER, PRESETS, dauToSlider, estimate, formatStepValue, sliderToDau, type EstimationInput, type PresetId } from '../simulations/estimation'
import { formatNumber } from '../core/format'

const props = withDefaults(defineProps<{ preset?: PresetId }>(), { preset: 'default' })
const input = reactive<EstimationInput>({ ...PRESETS[props.preset] })
const steps = computed(() => estimate(input))
const dauPosition = computed({
  get: () => dauToSlider(input.dau),
  set: (pos: number) => { input.dau = sliderToDau(pos) },
})

type NumericField = Exclude<keyof EstimationInput, 'dau'>
const FIELDS: readonly { key: NumericField; label: string; min: number; max: number; step: number; hint: string }[] = [
  { key: 'messagesPerUser', label: 'Сообщений на пользователя в день', min: 1, max: 200, step: 1, hint: 'Сколько сообщений в среднем отправляет один активный пользователь.' },
  { key: 'messageBytes', label: 'Размер сообщения с метаданными, байт', min: 10, max: 5_000, step: 10, hint: 'Текст плюс id, автор, чат, время, статус.' },
  { key: 'peakFactor', label: 'Коэффициент пика', min: 1, max: 10, step: 0.5, hint: 'Во сколько раз пик выше среднего.' },
  { key: 'retentionDays', label: 'Срок хранения, дней', min: 1, max: 3_650, step: 1, hint: '365 — год, 1825 — пять лет.' },
  { key: 'replicationFactor', label: 'Коэффициент репликации', min: 1, max: 5, step: 1, hint: 'Сколько копий данных храним.' },
]

function reset(): void {
  Object.assign(input, PRESETS[props.preset])
}
</script>

<template>
  <section class="calc" aria-label="Калькулятор оценки нагрузки">
    <div class="inputs">
      <label class="field">
        <span class="name">Активных пользователей в день (DAU): <strong>{{ formatNumber(input.dau) }}</strong></span>
        <input v-model.number="dauPosition" type="range" :min="DAU_SLIDER.min" :max="DAU_SLIDER.max" :step="DAU_SLIDER.step" />
        <span class="hint">Шкала логарифмическая: от тысячи до миллиарда.</span>
      </label>
      <label v-for="f in FIELDS" :key="f.key" class="field">
        <span class="name">{{ f.label }}: <strong>{{ formatNumber(input[f.key], 1) }}</strong></span>
        <span class="pair">
          <input v-model.number="input[f.key]" type="range" :min="f.min" :max="f.max" :step="f.step" />
          <input v-model.number="input[f.key]" type="number" :min="f.min" :max="f.max" :step="f.step" :aria-label="f.label" />
        </span>
        <span class="hint">{{ f.hint }}</span>
      </label>
      <button type="button" class="reset" @click="reset">Вернуть исходные значения</button>
    </div>
    <ol class="steps" aria-live="polite">
      <li v-for="(s, i) in steps" :key="s.id">
        <span class="no">{{ i + 1 }}</span>
        <div>
          <p class="label">{{ s.label }}: <strong>{{ formatStepValue(s) }}</strong></p>
          <p class="formula">{{ s.formula }}</p>
          <p class="expr">{{ s.expression }}</p>
        </div>
      </li>
    </ol>
  </section>
</template>

<style scoped>
.calc { display: grid; gap: 20px; margin: 24px 0; padding: 16px; border: 1px solid var(--vp-c-divider); border-radius: 12px; background: var(--vp-c-bg-soft); }
@media (min-width: 960px) { .calc { grid-template-columns: 1fr 1fr; } }
.field { display: flex; flex-direction: column; gap: 4px; margin-bottom: 14px; font-size: 14px; }
.pair { display: flex; gap: 8px; align-items: center; }
.pair input[type='range'], .field > input[type='range'] { flex: 1; min-width: 0; }
.pair input[type='number'] { width: 96px; padding: 4px 6px; border: 1px solid var(--vp-c-divider); border-radius: 6px; background: var(--vp-c-bg); }
.hint { color: var(--vp-c-text-3); font-size: 12px; }
.reset { padding: 6px 12px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg); font-size: 14px; }
.steps { list-style: none; margin: 0; padding: 0; }
.steps li { display: flex; gap: 12px; margin-bottom: 14px; }
.no { flex: none; display: grid; place-items: center; width: 28px; height: 28px; border-radius: 50%; background: var(--vp-c-brand-soft); color: var(--vp-c-brand-1); font-weight: 600; }
.steps p { margin: 0; }
.formula { color: var(--vp-c-text-2); font-size: 13px; }
.expr { font-family: var(--vp-font-family-mono); font-size: 13px; overflow-wrap: anywhere; }
</style>
```

Пустое поле `type="number"` даёт `''`, а `v-model.number` превращает его в `''` или `NaN`. Дальше `estimate` → `sanitizeInput` превращает это в 0 или 1, поэтому на экране не появится `NaN`. Это проверено тестом `sanitizeInput`.

- [ ] **Step 6: Написать урок `05-estimation.md`**

Frontmatter: `title: 0.5. Оценки на салфетке`.

Подключение: `<script setup>import EstimationCalculator from '../../viz/scenes/EstimationCalculator.vue'</script>`.

Обязательные разделы:

1. **Проблема.** На собеседовании и на работе спрашивают «сколько серверов нужно?» раньше, чем есть код. Нужна не точность, а правильный порядок величины.
2. **Что такое <T id="back-of-envelope">оценка на салфетке</T>.** Пять шагов:
   1. выписать допущения;
   2. агрессивно округлить;
   3. посчитать <T id="rps">RPS</T> — средний и пиковый;
   4. посчитать хранилище;
   5. посчитать трафик и проверить на здравый смысл.
3. **Шпаргалка степеней.**
   - таблица 2¹⁰ ≈ 10³ (КБ), 2²⁰ ≈ 10⁶ (МБ), 2³⁰ ≈ 10⁹ (ГБ), 2⁴⁰ ≈ 10¹² (ТБ), 2⁵⁰ ≈ 10¹⁵ (ПБ);
   - пометка о десятичных (КБ) и двоичных (КиБ) единицах и почему в оценках это неважно;
   - сутки = 86 400 с ≈ 10⁵ с;
   - правило: 1 млн запросов в сутки ≈ 12 RPS.
4. **Разбор примера руками.** Пресет `default` (1 млн <T id="dau">DAU</T>, 20 сообщений, 200 байт, пик ×2, 365 дней, ×3):
   1. 20 млн сообщений в день;
   2. ≈ 231 RPS;
   3. ≈ 463 RPS в пик;
   4. 4 ГБ в день;
   5. 4,38 ТБ за год с репликами;
   6. ≈ 93 КБ/с в пик.

   Каждое число вывести с округлением «на салфетке» (10⁵ с), а затем точно — и показать, что разница не меняет выводов.
5. **Понятия.** <T id="peak-factor">коэффициент пика</T>, <T id="retention">срок хранения</T>, <T id="replication-factor">коэффициент репликации</T>, <T id="metadata">метаданные</T>. Для каждого — откуда берётся типичное значение.
6. **Калькулятор.** `<EstimationCalculator />`. Три задания:
   - увеличьте DAU в 10 раз — что выросло линейно?
   - каким должен быть срок хранения, чтобы хранилище превысило 1 ПБ?
   - во сколько раз растёт хранилище от коэффициента репликации?
7. **Типичные ошибки.** <Callout type="mistake">:
   - забыть пик;
   - забыть реплики, индексы и метаданные;
   - ложная точность («231,48 RPS»);
   - смешивать биты и байты в трафике (×8).
8. **Компромисс.** <Callout type="tradeoff">: точность против скорости. Когда оценки уже недостаточно и нужен нагрузочный тест (ссылка на модуль 14).
9. **Самопроверка.** 4 `<SelfCheck>` с расчётами:
   - «10 млн DAU по 5 запросов — средний RPS?» (≈ 580);
   - «Сервис хранит 1 КБ на пользователя, 50 млн пользователей, ×3 — сколько?» (150 ГБ);
   - «Почему пиковый RPS важнее среднего при выборе числа серверов?»;
   - «1 Гбит/с — это сколько мегабайт в секунду?» (125 МБ/с).
10. **Источники.** Сюй, т. 1, гл. 2 «Back-of-the-envelope estimation»; Jeff Dean, LADIS 2009.

- [ ] **Step 7: Написать урок `06-messenger-requirements.md`**

Frontmatter: `title: 0.6. Требования к нашему мессенджеру`.

Подключение калькулятора — как в 0.5, использование `<EstimationCalculator preset="messenger" />`.

Обязательные разделы:

1. **Зачем писать требования до кода.** Связь с уроком 0.2. Результат урока — документ требований, к которому мы будем возвращаться в каждом модуле.
2. **Функциональные требования (MVP курса)** — нумерованный список с модулем, где реализуем:
   1. регистрация и вход (1, 6.7);
   2. личные чаты 1:1 (1);
   3. групповые чаты до 1 000 участников (1, 7);
   4. каналы с неограниченным числом подписчиков (7);
   5. история сообщений с пагинацией (2a, 2b);
   6. доставка в реальном времени (3);
   7. статусы «доставлено» и «прочитано» (3);
   8. «в сети» и «печатает…» (3);
   9. несколько устройств у одного пользователя (3);
   10. вложения (11);
   11. поиск по сообщениям (11);
   12. push-уведомления (11).
3. **Что сознательно НЕ делаем.** Звонки, полноценное E2E-шифрование (только идея в 6.7), боты, платежи, стикеры. Объяснить, почему фиксировать «не делаем» так же важно, как «делаем».
4. **Нефункциональные требования с числами.** Таблица:

   | Требование | Значение | Почему так |
   |---|---|---|
   | DAU | 100 млн | ориентир; в задаче Сюя про чат — 50 млн |
   | Доставка онлайн-получателю | p99 < 1 с внутри региона | |
   | API отправки | p99 < 300 мс | |
   | Доступность отправки | 99,95% (≈ 22 мин простоя в месяц) | |
   | Долговечность | подтверждённое сообщение не теряется | |
   | Порядок | сообщения внутри одного чата видны всем в одном порядке | |
   | Хранение | 5 лет | продуктовое решение; упомянуть, что многие мессенджеры хранят историю бессрочно |

   Колонку «Почему так» заполнить для каждой строки.
5. **Обоснование пресета.**
   - 40 сообщений в день на пользователя — оценка, а не факт;
   - 200 байт: ~100 байт текста + ~100 байт метаданных;
   - пик ×3 — вечер и праздники;
   - 5 лет, ×3 реплики.
6. **Калькулятор.** `<EstimationCalculator preset="messenger" />`.
7. **Интерпретация результатов** — главная часть урока:
   - ≈ 46 тыс. RPS в среднем и ≈ 139 тыс. в пик на запись. Один сервер PostgreSQL такой поток записи в одиночку, скорее всего, не выдержит (оценка; точный потолок зависит от железа и схемы) → шардирование, модуль 9;
   - ≈ 4,4 ПБ за 5 лет с репликами → разбиение данных и «горячее/холодное» хранение (модули 9, 9.5);
   - каждое сообщение читают все участники чата — нагрузка на чтение в разы выше записи → fan-out, модуль 7;
   - одновременные соединения: если онлайн 30% DAU (допущение), это 30 млн WebSocket-соединений. При ~100 тыс. на сервер (допущение, проверим в модуле 1.5) — ≈ 300 серверов шлюза (модуль 3).
8. **Мини-интервью модуля 0** (сквозная практика 2.1).

   Задание: «Сформулируйте требования и оцените нагрузку для корпоративного мессенджера вроде Slack: 10 млн DAU, рабочие часы, много каналов». 45 минут.

   Эталонный разбор — в `<SelfCheck question="Эталонный разбор">`:
   - пиковость рабочих часов (×4–5);
   - поиск по истории как ключевое функциональное требование;
   - расчёт по калькулятору;
   - что изменилось по сравнению с нашим мессенджером.
9. **Самопроверка.** 3 `<SelfCheck>`:
   - «Почему “канал на миллион подписчиков” — нефункциональный вызов, хотя звучит как функция?»;
   - «Какое из наших требований сильнее всего влияет на выбор хранилища?»;
   - «Что изменится в расчёте, если хранить историю бессрочно?»
10. **Источники.** Сюй, т. 1, гл. 12 «Design a Chat System»; Клеппман, гл. 1 (пример с Twitter и fan-out).

- [ ] **Step 8: Проверить**

Run: `npm test && npm run typecheck && npm run build`
Expected: PASS, `build complete`.

Затем вручную в dev:
- ползунок DAU идёт от 1 000 до 1 000 000 000;
- если стереть число в поле, на экране нули, а не `NaN`;
- в уроке 0.6 калькулятор стартует с пресета мессенджера и выдаёт 46 296 RPS и 4,4 ПБ;
- кнопка сброса возвращает пресет.

Затем чек-листы 2.2 и 2.3 по обоим урокам.

- [ ] **Step 9: Commit**

```bash
git add course/viz/simulations/estimation.ts course/viz/simulations/estimation.test.ts course/viz/scenes/EstimationCalculator.vue course/modules/00-foundation/05-estimation.md course/modules/00-foundation/06-messenger-requirements.md
git commit -m "feat(viz): калькулятор оценок, уроки 0.5 и 0.6

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Финальная проверка, README, синхронизация спецификации

**Files:**
- Create: `README.md` (в корне репозитория)
- Modify: `docs/superpowers/specs/2026-10-09-course-platform-design.md` (раздел 4.2)

**Interfaces:**
- Consumes: всё из задач 1–8.

- [ ] **Step 1: Полный прогон**

Run: `cd /home/idel/DevGo/system-desine/course && npm test && npm run typecheck && npm run build`
Expected: все тесты PASS, ошибок типов нет, `build complete` без предупреждений о битых ссылках.

- [ ] **Step 2: Покрытие терминов**

Run: `grep -ohE '<T id="[^"]+"' modules -r | sort -u | wc -l` и `grep -cE '^[a-z0-9-]+:$' glossary/terms.yaml`

Expected:
- первое число > 0;
- каждый термин из `terms.yaml` используется хотя бы в одном уроке. Проверка: `for id in $(grep -oE '^[a-z0-9-]+' glossary/terms.yaml); do grep -rq "id=\"$id\"" modules || echo "не используется: $id"; done` — пустой вывод.

Неиспользуемые термины либо вставить в уроки по смыслу, либо удалить из `terms.yaml`.

- [ ] **Step 3: Проверка вёрстки и тем через Playwright**

Запустить `npm run preview` в фоне (порт 4173). Через инструменты Playwright для каждого из 6 уроков и для `/glossary`:
1. `browser_resize` 375×812, `browser_navigate`, `browser_evaluate` с `document.documentElement.scrollWidth <= window.innerWidth` → должно быть `true`;
2. `browser_take_screenshot` в светлой теме;
3. `browser_evaluate` `document.documentElement.classList.add('dark')` → скриншот в тёмной теме;
4. просмотреть скриншоты: подписи в SVG читаемы, полосы и пакеты контрастны, панель плеера переносится.

Дополнительно на уроке 0.3 при ширине 1280:
- Tab до плеера, `browser_press_key` ArrowRight ×3 → подпись «Шаг 4 из 15»;
- Tab до кнопки «Шаг вперёд», пробел → «Шаг 5 из 15» (а не 6);
- кликнуть «Воспроизвести», затем перейти на урок 0.4 и вернуться назад → в консоли (`browser_console_messages`) нет ошибок.

Найденные проблемы исправить в соответствующих компонентах, затем повторить шаги 1–3.

- [ ] **Step 4: Создать `README.md` в корне**

````md
# System Design на практике

Личный курс по проектированию высоконагруженных систем на Go: сайт курса с пошаговыми визуализациями (`course/`) и сквозной проект-мессенджер (`messenger/`, появится в модуле 1).

## Запуск курса

```bash
cd course
npm install
npm run dev        # http://localhost:5173
```

## Проверки

```bash
npm test           # Vitest: логика плеера, сценарии, расчёты, глоссарий, сайдбар
npm run typecheck  # vue-tsc
npm run build      # сборка; падает на неизвестном термине <T id="...">
```

## Как добавить урок

1. Файл `course/modules/NN-slug/NN-lesson.md` с frontmatter `title:` — он сам появится в меню.
2. Термины: `<T id="...">слово</T>`, определение — в `course/glossary/terms.yaml`.
3. Визуализация: сценарий в `course/viz/simulations/` (чистая функция + тест), рендерер в `course/viz/renderers/`, сцена в `course/viz/scenes/`; цвета — из `course/viz/palette.css` (см. `course/viz/README.md`).
4. Перед коммитом — чек-листы подробности и визуализации из спецификации (разделы 2.2 и 2.3).

Спецификация: `docs/superpowers/specs/2026-10-09-course-platform-design.md`.
````

- [ ] **Step 5: Синхронизировать спецификацию с реализацией**

В разделе 4.2 спецификации:
- блок с `type Simulation<S, A> = ...` заменить предложением: «Тип `Simulation<S, A>` (песочница, порождающая шаги по действию пользователя) вводится вместе с первой такой визуализацией — в модуле 2.5 (алгоритмы балансировки). В модуле 0 песочница-калькулятор работает без шагов плеера.»;
- рядом с `Step<S>` дописать, что `codeLine` подсвечивает строку в блоке кода, который VitePress рендерит через Shiki, а `CodePanel` только переключает класс у `span.line`.

- [ ] **Step 6: Commit**

```bash
cd /home/idel/DevGo/system-desine
git add README.md docs/superpowers/specs/2026-10-09-course-platform-design.md course
git commit -m "docs: README, синхронизация спецификации; финальные правки модуля 0

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
