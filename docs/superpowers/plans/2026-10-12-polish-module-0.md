# Полировка модуля 0 — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Привести платформу и модуль 0 к новым стандартам:
- стиль «Ночной терминал»;
- самодостаточная теория с блоками «🔬 Под капотом»;
- Go-фрагменты уровня прода, проверяемые автоматически;
- полноэкранный режим для визуализаций и схем.

**Architecture:** Новые компоненты урока (`Part`, `N`, `Deep`) и компонент `VizFrame` — тонкие Vue-обёртки. Логика вынесена в чистые TS-модули с тестами:
- палитра Mermaid;
- режимы полноэкранного показа;
- сегменты подписей;
- разбор Go-фрагментов.

Mermaid переезжает с `vitepress-plugin-mermaid` на собственный компонент, который грузит библиотеку динамически. Go-фрагменты уроков проверяет скрипт `npm run check:go` (`gofmt` + `go vet`) через шаблоны-обвязки.

**Tech Stack:** VitePress 1.6.4, Vue 3.5.43, TypeScript 5.9.3, Vitest 3.2.7, GSAP 3.15.0, Mermaid 11.17.2, `@fontsource-variable/inter` 5.3.0, `@fontsource-variable/jetbrains-mono` 5.3.0, Go 1.26, Node 26.

**Spec:**
- `docs/superpowers/specs/2026-10-12-polish-module-0-design.md` — главный документ этого плана.
- `docs/superpowers/specs/2026-10-09-course-platform-design.md` — разделы 1, 2.2 (п. 10), 2.3, 2.4, 2.5.

**Осознанные отступления от спецификации полировки:**
- **Подсказка термина (п. 6.4).** Вместо смены роли на `dialog` из подсказки убирается интерактивная ссылка. Подсказка становится неинтерактивной, и `role="tooltip"` для неё корректна. Это решает ту же проблему проще. На телефоне второе нажатие по термину по-прежнему открывает глоссарий.
- **Размер чанков (п. 8).** Собственный чанк Mermaid сам больше 500 КБ, поэтому предупреждение Rollup останется, даже когда основной чанк станет лёгким. Порог `chunkSizeWarningLimit` поднимается с комментарием. Проверяется другое: в основном чанке (`app.*.js`) нет Mermaid.

## Global Constraints

- Все команды npm выполняются из `/home/idel/DevGo/system-desine/course`. Версии пакетов пишутся точно, без `^`.
- Язык интерфейса и контента — русский. Английский оригинал термина — в скобках при первом упоминании.
- Каждый термин в уроке при первом появлении оборачивается в `<T id="...">` с id в двойных кавычках. Неизвестный id — ошибка сборки.
- Чистые модули (`*.ts` вне `.vue`) не импортируют `vue` и не трогают `window`/`document` на уровне модуля. DOM — только в `onMounted`, обработчиках и `watch` без `immediate`. Все слушатели и наблюдатели снимаются в `onBeforeUnmount`.
- Тёмная тема по умолчанию (`appearance: 'dark'`), светлая — переключателем. Каждый цвет определён для обеих тем.
- Цвета — только через CSS-переменные: `--vp-c-*` и `--course-*` в `.vitepress/theme/custom.css`, `--viz-*` в `viz/palette.css`, `--part-*` в `components/parts.css`. Хардкод цвета в компонентах запрещён, кроме палитры Mermaid в `viz/core/mermaid-palette.ts`: Mermaid не читает CSS-переменные.
- Контраст текста — не меньше 4,5:1 в обеих темах.
- Шрифты только из пакетов `@fontsource-variable/*`, без внешних запросов.
- Анимации и переходы отключаются при `prefers-reduced-motion: reduce`.
- Ширина 375 px — без горизонтальной прокрутки страницы.
- Тексты уроков пишутся по разделу 1 основной спецификации («Голос автора») и по чек-листам 2.2 (10 пунктов) и 2.3. Числа даются с источником, оценки явно помечены.
- Каждый блок кода ` ```go ` в `modules/` помечен `<!-- go-check -->` (или `<!-- go-check: harness=имя -->`) и проходит `npm run check:go`.
- Коммиты на русском и заканчиваются строкой `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Смена темы, когда на странице уже нарисована схема Mermaid** (в том числе в полноэкранном режиме). Ожидается: схема перерисовывается в цветах новой темы, на странице не остаётся двух SVG с одинаковым `id`. Покрывают задача 2 (счётчик рендеров в `id`) и задача 9 (Playwright).
2. **Уход со страницы, когда визуализация открыта оверлеем.** Ожидается: прокрутка страницы снова работает, на `<html>` не висит `viz-scroll-lock`. Покрывают `onBeforeUnmount` в `VizFrame` (задача 4) и задача 9.
3. **Выход из настоящего полноэкранного режима браузерной клавишей Esc.** Ожидается: кнопка снова показывает ⛶, повторный вход работает. Покрывают обработчик `fullscreenchange` (задача 4) и задача 9.
4. **Якоря заголовков после добавления меток `<Part>`.** Ожидается: `id` заголовков и пункты оглавления не содержат текста метки, старые ссылки вида `#...` продолжают работать. Покрывают шаг проверки в задаче 1 и задача 9.
5. **Мусорный или неполный Go-фрагмент.** Например, фрагмент без маркера или маркер с несуществующей обвязкой. Ожидается: `check:go` падает с понятным сообщением, где указаны файл и строка. Покрывают тесты `go-snippets.test.ts` (задача 5).

---

## Структура файлов

```
course/
├── package.json                       # + шрифты, − vitepress-plugin-mermaid, + check:go
├── tsconfig.json                      # + scripts/**, allowImportingTsExtensions
├── vitest.config.ts                   # + scripts/**/*.test.ts
├── glossary.md                        # GlossaryList импортируется локально
├── .vitepress/
│   ├── config.ts                      # appearance dark, mermaid-fence, chunk limit
│   ├── mermaid-fence.ts (+ .test.ts)  # ```mermaid → <MermaidDiagram>
│   ├── sidebar.ts (+ test)            # ошибка YAML называет файл
│   └── theme/
│       ├── index.ts                   # шрифты, новые компоненты
│       └── custom.css                 # токены «Ночного терминала»
├── components/
│   ├── parts.ts, parts.css, Part.vue  # метки частей урока
│   ├── N.vue                          # подсветка чисел
│   ├── Deep.vue                       # «🔬 Под капотом»
│   ├── MermaidDiagram.vue             # ленивый Mermaid в палитре
│   ├── Callout.vue                    # + тип simplified
│   ├── T.vue                          # лёгкие данные, без ссылки в подсказке
│   └── GlossaryList.vue               # алфавитный указатель
├── glossary/
│   ├── schema.ts (+ test)             # toShortTerms, groupByLetter
│   └── terms-short.data.ts            # лёгкий loader для T
├── viz/
│   ├── palette.css                    # тёмные значения под #0b1220
│   ├── README.md                      # легенда: задержки, Mermaid-классы, VizFrame
│   ├── core/
│   │   ├── caption.ts (+ test)        # сегменты подписей
│   │   ├── fullscreen.ts (+ test)     # режимы off/native/overlay
│   │   ├── mermaid-palette.ts (+ test)
│   │   ├── player.ts (+ test)         # seek(NaN) → 0
│   │   ├── format.ts (+ test)         #   вместо литерала
│   │   └── types.ts                   # Step.caption: Caption
│   ├── components/
│   │   ├── Player.vue                 # подписи с <T>
│   │   └── VizFrame.vue               # ⛶
│   ├── simulations/request-path.ts (+ test)  # сегменты, новый CODE_LINE
│   └── scenes/*.vue                   # обёрнуты в VizFrame
├── scripts/
│   ├── go-snippets.ts (+ test)        # извлечение и сборка фрагментов
│   ├── check-go-snippets.ts           # gofmt + go vet
│   └── go-harness/
│       ├── send-message.go.tmpl
│       └── sql-pool.go.tmpl
└── modules/00-foundation/*.md         # метки, числа, «Под капотом», источники
```

---

### Task 1: Тема «Ночной терминал» и компоненты Part, N, Deep

**Files:**
- Create: `course/components/parts.ts`, `course/components/parts.css`, `course/components/Part.vue`, `course/components/N.vue`, `course/components/Deep.vue`, `course/components/parts.test.ts`
- Modify: `course/package.json`, `course/vitest.config.ts`, `course/.vitepress/config.ts`, `course/.vitepress/theme/index.ts`, `course/.vitepress/theme/custom.css`, `course/viz/palette.css`

**Interfaces:**
- Produces:
  - `PARTS: Record<PartKind, string>`, `PartKind`;
  - глобальные компоненты `<Part kind="...">`, `<N tone?="num|brand|accent">`, `<Deep title="...">`;
  - CSS-переменные `--course-accent`, `--course-num-fg`, `--course-num-bg`, `--course-deep-bg`, `--part-<kind>-fg`, `--part-<kind>-bg`.

- [ ] **Step 1: Добавить шрифты**

В `course/package.json` в `devDependencies` добавить `"@fontsource-variable/inter": "5.3.0"` и `"@fontsource-variable/jetbrains-mono": "5.3.0"`.

Run: `npm install`
Expected: установка без конфликтов peer-зависимостей.

- [ ] **Step 2: Написать падающий тест `course/components/parts.test.ts`**

В `course/vitest.config.ts` добавить в `include` шаблон `'components/**/*.test.ts'`.

```ts
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { PARTS } from './parts'

const css = readFileSync(new URL('./parts.css', import.meta.url), 'utf8')

describe('PARTS', () => {
  it('у каждой метки есть подпись', () => {
    for (const label of Object.values(PARTS)) expect(label.trim().length).toBeGreaterThan(0)
  })

  it.each(Object.keys(PARTS))('у «%s» есть цвета в обеих темах', (kind) => {
    // Переменные объявлены дважды: в :root (светлая тема) и в .dark.
    expect(css.match(new RegExp(`--part-${kind}-fg:`, 'g'))?.length).toBe(2)
    expect(css.match(new RegExp(`--part-${kind}-bg:`, 'g'))?.length).toBe(2)
  })
})
```

- [ ] **Step 3: Запустить и убедиться, что тест падает**

Run: `npm test -- components/parts.test.ts`
Expected: FAIL — нет `./parts`.

- [ ] **Step 4: Создать `course/components/parts.ts` и `course/components/parts.css`**

```ts
// Метки частей урока. Цвета — в parts.css, по одному набору на тему.
export const PARTS = {
  problem: 'Проблема',
  theory: 'Теория',
  viz: 'Визуализация',
  mistakes: 'Ошибки',
  tradeoff: 'Компромисс',
  code: 'Код',
  selfcheck: 'Самопроверка',
  sources: 'Источники',
  deep: 'Под капотом',
} as const

export type PartKind = keyof typeof PARTS
```

```css
:root {
  --part-problem-fg: #be123c; --part-problem-bg: #ffe4e6;
  --part-theory-fg: #0369a1; --part-theory-bg: #e0f2fe;
  --part-viz-fg: #0f766e; --part-viz-bg: #ccfbf1;
  --part-mistakes-fg: #c2410c; --part-mistakes-bg: #ffedd5;
  --part-tradeoff-fg: #b45309; --part-tradeoff-bg: #fef3c7;
  --part-code-fg: #334155; --part-code-bg: #f1f5f9;
  --part-selfcheck-fg: #4d7c0f; --part-selfcheck-bg: #ecfccb;
  --part-sources-fg: #6d28d9; --part-sources-bg: #ede9fe;
  --part-deep-fg: #0369a1; --part-deep-bg: #f0f9ff;
}

.dark {
  --part-problem-fg: #fda4af; --part-problem-bg: #3f1d2b;
  --part-theory-fg: #7dd3fc; --part-theory-bg: #0c2a43;
  --part-viz-fg: #5eead4; --part-viz-bg: #0d3330;
  --part-mistakes-fg: #fdba74; --part-mistakes-bg: #3b200d;
  --part-tradeoff-fg: #fcd34d; --part-tradeoff-bg: #3a2a0a;
  --part-code-fg: #cbd5e1; --part-code-bg: #1e293b;
  --part-selfcheck-fg: #bef264; --part-selfcheck-bg: #1a2e0d;
  --part-sources-fg: #c4b5fd; --part-sources-bg: #2a1f3d;
  --part-deep-fg: #7dd3fc; --part-deep-bg: #0f1b2e;
}
```

- [ ] **Step 5: Запустить тест**

Run: `npm test -- components/parts.test.ts`
Expected: PASS (10 тестов).

- [ ] **Step 6: Проверить, как VitePress строит оглавление и якоря**

Run: `grep -rn "ignore-header" node_modules/vitepress/dist/client/theme-default/composables/outline.js; grep -rn "html_inline" node_modules/vitepress/dist/node/*.js | head -5`

Expected:
- первая команда находит `ignore-header` в `serializeHeader` — элементы с этим классом не попадают в текст пункта оглавления;
- вторая показывает, что `html_inline` исключается из текста при генерации slug.

Если хотя бы одно не так — остановиться и сообщить DONE_WITH_CONCERNS с выводом. Тогда метку придётся ставить не в заголовок, а строкой над ним.

- [ ] **Step 7: Создать `Part.vue`, `N.vue`, `Deep.vue`**

`course/components/Part.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { PARTS, type PartKind } from './parts'

const props = defineProps<{ kind: PartKind }>()
const label = computed(() => PARTS[props.kind])
</script>

<template>
  <!-- ignore-header: VitePress не включает метку в текст пункта оглавления. -->
  <span class="part ignore-header" :class="`part-${kind}`">{{ label }}</span>
</template>

<style scoped>
.part {
  display: inline-block;
  margin-right: 0.5em;
  padding: 0.15em 0.6em;
  border-radius: 999px;
  font-size: 0.55em;
  font-weight: 700;
  letter-spacing: 0.02em;
  vertical-align: 0.3em;
  white-space: nowrap;
}
.part-problem { color: var(--part-problem-fg); background: var(--part-problem-bg); }
.part-theory { color: var(--part-theory-fg); background: var(--part-theory-bg); }
.part-viz { color: var(--part-viz-fg); background: var(--part-viz-bg); }
.part-mistakes { color: var(--part-mistakes-fg); background: var(--part-mistakes-bg); }
.part-tradeoff { color: var(--part-tradeoff-fg); background: var(--part-tradeoff-bg); }
.part-code { color: var(--part-code-fg); background: var(--part-code-bg); }
.part-selfcheck { color: var(--part-selfcheck-fg); background: var(--part-selfcheck-bg); }
.part-sources { color: var(--part-sources-fg); background: var(--part-sources-bg); }
.part-deep { color: var(--part-deep-fg); background: var(--part-deep-bg); }
</style>
```

`course/components/N.vue`:

```vue
<script setup lang="ts">
withDefaults(defineProps<{ tone?: 'num' | 'brand' | 'accent' }>(), { tone: 'num' })
</script>

<template>
  <span class="n" :class="`n-${tone}`"><slot /></span>
</template>

<style scoped>
.n {
  padding: 0 0.3em;
  border-radius: 5px;
  font-family: var(--vp-font-family-mono);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.n-num { color: var(--course-num-fg); background: var(--course-num-bg); }
.n-brand { color: var(--vp-c-brand-1); background: var(--vp-c-brand-soft); }
.n-accent { color: var(--course-accent); background: var(--course-accent-soft); }
</style>
```

`course/components/Deep.vue`:

```vue
<script setup lang="ts">
defineProps<{ title: string }>()
</script>

<template>
  <!-- Нативный details: доступен с клавиатуры, а поиск по странице раскрывает его сам. -->
  <details class="deep">
    <summary><span aria-hidden="true">🔬</span> Под капотом: {{ title }}</summary>
    <div class="deep-body"><slot /></div>
  </details>
</template>

<style scoped>
.deep {
  margin: 20px 0;
  border-left: 4px solid var(--vp-c-brand-1);
  border-radius: 8px;
  background: var(--course-deep-bg);
}
.deep summary {
  padding: 10px 14px;
  color: var(--part-deep-fg);
  font-weight: 600;
  cursor: pointer;
}
.deep summary:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
.deep-body { padding: 0 16px 12px; }
.deep-body :deep(p:last-child) { margin-bottom: 0; }
</style>
```

- [ ] **Step 8: Токены темы в `course/.vitepress/theme/custom.css`**

Существующее правило для таблиц сохранить. В начало файла добавить:

```css
:root {
  --vp-font-family-base: 'Inter Variable', Inter, ui-sans-serif, system-ui, sans-serif;
  --vp-font-family-mono: 'JetBrains Mono Variable', ui-monospace, SFMono-Regular, monospace;
  --vp-c-brand-1: #0369a1;
  --vp-c-brand-2: #0284c7;
  --vp-c-brand-3: #0369a1;
  --vp-c-brand-soft: rgba(2, 132, 199, 0.14);
  --course-accent: #4d7c0f;
  --course-accent-soft: rgba(77, 124, 15, 0.12);
  --course-num-fg: #b45309;
  --course-num-bg: #fef3c7;
  --course-deep-bg: #f0f9ff;
}

.dark {
  --vp-c-bg: #0b1220;
  --vp-c-bg-alt: #080e1a;
  --vp-c-bg-soft: #0f172a;
  --vp-c-bg-elv: #111b2e;
  --vp-c-divider: #1e293b;
  --vp-c-gutter: #080e1a;
  --vp-c-brand-1: #38bdf8;
  --vp-c-brand-2: #7dd3fc;
  --vp-c-brand-3: #38bdf8;
  --vp-c-brand-soft: rgba(56, 189, 248, 0.16);
  /* Тёмный текст на голубой кнопке: белый на #38bdf8 не проходит 4,5:1. */
  --vp-button-brand-text: #0b1220;
  --vp-button-brand-hover-text: #0b1220;
  --vp-button-brand-active-text: #0b1220;
  --course-accent: #a3e635;
  --course-accent-soft: rgba(163, 230, 53, 0.14);
  --course-num-fg: #fcd34d;
  --course-num-bg: #3a2a0a;
  --course-deep-bg: #0f1b2e;
}
```

- [ ] **Step 9: Палитра визуализаций под фон `#0b1220`**

В `course/viz/palette.css` в блоке `.dark` заменить три значения: `--viz-node-bg: #111b2e;`, `--viz-edge: #334155;`, `--viz-text: #e2e8f0;`. Остальные тёмные значения сохраняются: они уже проходят контраст на тёмном фоне.

- [ ] **Step 10: Подключить в теме и конфиге**

В `course/.vitepress/theme/index.ts`:

```ts
import '@fontsource-variable/inter'
import '@fontsource-variable/jetbrains-mono'
import Part from '../../components/Part.vue'
import N from '../../components/N.vue'
import Deep from '../../components/Deep.vue'
import '../../components/parts.css'
// внутри enhanceApp:
    app.component('Part', Part)
    app.component('N', N)
    app.component('Deep', Deep)
```

Шрифты импортировать до `custom.css`. В `course/.vitepress/config.ts` добавить на верхний уровень `appearance: 'dark',`.

- [ ] **Step 11: Проверить**

Run: `npm test && npm run typecheck && npm run build`
Expected: PASS, `build complete`.

Затем временно добавить в `modules/00-foundation/index.md`:
- строку `## <Part kind="theory" /> Проверка метки`;
- абзац с `<N>144 мс</N>`;
- блок `<Deep title="тест">Текст.</Deep>`.

Собрать (`npm run build`) и выполнить: `grep -o 'id="[^"]*"' .vitepress/dist/modules/00-foundation/index.html | grep -i провер`
Expected: `id="проверка-метки"` — без «теория».

Через `npm run preview` и Playwright проверить:
- в обеих темах метка, число и блок видны и контрастны;
- пункт оглавления «Проверка метки» без слова «Теория»;
- шрифт Inter загружен с localhost.

Удалить временные строки.

- [ ] **Step 12: Commit**

```bash
git add course/package.json course/package-lock.json course/vitest.config.ts course/components course/.vitepress course/viz/palette.css
git commit -m "feat(course): тема «Ночной терминал», метки частей урока, числа и блок «Под капотом»

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Mermaid — ленивая загрузка и палитра

**Files:**
- Create: `course/viz/core/mermaid-palette.ts`, `course/viz/core/mermaid-palette.test.ts`, `course/.vitepress/mermaid-fence.ts`, `course/.vitepress/mermaid-fence.test.ts`, `course/components/MermaidDiagram.vue`
- Modify: `course/package.json`, `course/.vitepress/config.ts`, `course/.vitepress/theme/index.ts`, `course/viz/README.md`

**Interfaces:**
- Consumes: тема из задачи 1.
- Produces:
  - `MERMAID_CLASSES`, `MermaidClass`, `ThemeName`;
  - `mermaidThemeVariables(theme: ThemeName): Record<string, string>`;
  - `withPaletteClasses(code: string, theme: ThemeName): string`;
  - `mermaidFencePlugin(md: MarkdownRenderer): void`;
  - глобальный компонент `<MermaidDiagram code="<encodeURIComponent>">`.

  Задача 4 оборачивает его в `VizFrame`.

- [ ] **Step 1: Написать падающий тест `course/viz/core/mermaid-palette.test.ts`**

```ts
import { describe, expect, it } from 'vitest'
import { MERMAID_CLASSES, mermaidThemeVariables, withPaletteClasses } from './mermaid-palette'

describe('withPaletteClasses', () => {
  it('добавляет classDef для всех классов сразу после строки flowchart', () => {
    const out = withPaletteClasses('flowchart LR\n  A --> B', 'dark').split('\n')
    expect(out[0]).toBe('flowchart LR')
    for (const name of MERMAID_CLASSES) expect(out.some((l) => l.trim().startsWith(`classDef ${name} `))).toBe(true)
    expect(out.at(-1)).toBe('  A --> B')
  })

  it('понимает graph и пустые строки в начале', () => {
    const out = withPaletteClasses('\n\ngraph TB\nA', 'light').split('\n')
    expect(out.findIndex((l) => l.startsWith('graph TB'))).toBeLessThan(out.findIndex((l) => l.includes('classDef client')))
  })

  it('не трогает другие типы диаграмм', () => {
    const seq = 'sequenceDiagram\n  A->>B: hi'
    expect(withPaletteClasses(seq, 'dark')).toBe(seq)
  })

  it('в тёмной и светлой теме цвета разные', () => {
    expect(withPaletteClasses('flowchart LR', 'dark')).not.toBe(withPaletteClasses('flowchart LR', 'light'))
  })
})

describe('mermaidThemeVariables', () => {
  it.each(['dark', 'light'] as const)('%s: задаёт фон, текст, линии и шрифт', (theme) => {
    const v = mermaidThemeVariables(theme)
    for (const key of ['background', 'primaryColor', 'primaryTextColor', 'lineColor', 'fontFamily']) expect(v[key]).toBeTruthy()
  })
})
```

- [ ] **Step 2: Запустить и убедиться, что тест падает**

Run: `npm test -- viz/core/mermaid-palette.test.ts`
Expected: FAIL — нет модуля.

- [ ] **Step 3: Реализовать `course/viz/core/mermaid-palette.ts`**

```ts
// Mermaid не читает CSS-переменные, поэтому палитра продублирована здесь.
// Смысл цветов совпадает с viz/palette.css: клиент синий, сервис фиолетовый, БД зелёная…
export type ThemeName = 'dark' | 'light'

export const MERMAID_CLASSES = ['client', 'service', 'db', 'cache', 'broker', 'network', 'deep'] as const
export type MermaidClass = (typeof MERMAID_CLASSES)[number]

interface ClassStyle {
  fill: string
  stroke: string
  color: string
  dashed?: boolean
}

const CLASS_STYLES: Record<ThemeName, Record<MermaidClass, ClassStyle>> = {
  dark: {
    client: { fill: '#0c1a33', stroke: '#60a5fa', color: '#dbeafe' },
    service: { fill: '#1c1433', stroke: '#a78bfa', color: '#ede9fe' },
    db: { fill: '#062a20', stroke: '#34d399', color: '#d1fae5' },
    cache: { fill: '#2a1d05', stroke: '#fbbf24', color: '#fef3c7' },
    broker: { fill: '#062a30', stroke: '#22d3ee', color: '#cffafe' },
    network: { fill: '#111b2e', stroke: '#94a3b8', color: '#e2e8f0' },
    deep: { fill: '#0f1b2e', stroke: '#38bdf8', color: '#e0f2fe', dashed: true },
  },
  light: {
    client: { fill: '#eff6ff', stroke: '#2563eb', color: '#1e3a8a' },
    service: { fill: '#f5f3ff', stroke: '#7c3aed', color: '#4c1d95' },
    db: { fill: '#ecfdf5', stroke: '#059669', color: '#064e3b' },
    cache: { fill: '#fffbeb', stroke: '#d97706', color: '#78350f' },
    broker: { fill: '#ecfeff', stroke: '#0891b2', color: '#164e63' },
    network: { fill: '#f8fafc', stroke: '#64748b', color: '#1e293b' },
    deep: { fill: '#f0f9ff', stroke: '#0284c7', color: '#0c4a6e', dashed: true },
  },
}

const THEME_VARIABLES: Record<ThemeName, Record<string, string>> = {
  dark: {
    background: '#0b1220',
    primaryColor: '#111b2e',
    primaryTextColor: '#e2e8f0',
    primaryBorderColor: '#38bdf8',
    secondaryColor: '#0f172a',
    tertiaryColor: '#0f172a',
    lineColor: '#64748b',
    textColor: '#e2e8f0',
    clusterBkg: '#0f172a',
    clusterBorder: '#1e293b',
    edgeLabelBackground: '#0b1220',
    fontFamily: "'Inter Variable', Inter, sans-serif",
  },
  light: {
    background: '#ffffff',
    primaryColor: '#f8fafc',
    primaryTextColor: '#0f172a',
    primaryBorderColor: '#0284c7',
    secondaryColor: '#f1f5f9',
    tertiaryColor: '#ffffff',
    lineColor: '#94a3b8',
    textColor: '#0f172a',
    clusterBkg: '#f8fafc',
    clusterBorder: '#e2e8f0',
    edgeLabelBackground: '#ffffff',
    fontFamily: "'Inter Variable', Inter, sans-serif",
  },
}

export function mermaidThemeVariables(theme: ThemeName): Record<string, string> {
  return { ...THEME_VARIABLES[theme] }
}

function classDef(name: MermaidClass, s: ClassStyle): string {
  const dash = s.dashed ? ',stroke-dasharray:5 3' : ''
  return `  classDef ${name} fill:${s.fill},stroke:${s.stroke},color:${s.color},stroke-width:2px${dash}`
}

const FLOWCHART_HEADER = /^\s*(flowchart|graph)\b/

export function withPaletteClasses(code: string, theme: ThemeName): string {
  const lines = code.split('\n')
  const header = lines.findIndex((line) => line.trim() !== '')
  if (header === -1 || !FLOWCHART_HEADER.test(lines[header] ?? '')) return code
  const defs = MERMAID_CLASSES.map((name) => classDef(name, CLASS_STYLES[theme][name]))
  return [...lines.slice(0, header + 1), ...defs, ...lines.slice(header + 1)].join('\n')
}
```

- [ ] **Step 4: Запустить тест**

Run: `npm test -- viz/core/mermaid-palette.test.ts`
Expected: PASS (6 тестов).

- [ ] **Step 5: Написать падающий тест `course/.vitepress/mermaid-fence.test.ts`**

```ts
import { describe, expect, it } from 'vitest'
import { createMarkdownRenderer } from 'vitepress'
import { mermaidFencePlugin } from './mermaid-fence'

async function render(src: string): Promise<string> {
  const md = await createMarkdownRenderer(process.cwd())
  mermaidFencePlugin(md)
  return md.render(src)
}

describe('mermaidFencePlugin', () => {
  it('превращает блок mermaid в компонент с закодированным кодом', async () => {
    const html = await render('```mermaid\nflowchart LR\n  A --> B\n```\n')
    expect(html).toContain(`<MermaidDiagram code="${encodeURIComponent('flowchart LR\n  A --> B\n')}" />`)
  })

  it('не трогает другие блоки кода', async () => {
    const html = await render('```go\nfunc main() {}\n```\n')
    expect(html).not.toContain('MermaidDiagram')
    expect(html).toContain('main')
  })
})
```

- [ ] **Step 6: Запустить и убедиться, что тест падает**

Run: `npm test -- .vitepress/mermaid-fence.test.ts`
Expected: FAIL — нет модуля.

- [ ] **Step 7: Реализовать `course/.vitepress/mermaid-fence.ts`**

```ts
import type { MarkdownRenderer } from 'vitepress'

// Блок ```mermaid становится компонентом, который сам грузит mermaid в браузере.
// Код кодируется: в атрибуте не должно быть кавычек и переводов строк.
export function mermaidFencePlugin(md: MarkdownRenderer): void {
  const fallback = md.renderer.rules.fence
  if (fallback === undefined) throw new Error('mermaid-fence: у markdown-it нет правила fence')
  md.renderer.rules.fence = (tokens, idx, options, env, self) => {
    const token = tokens[idx]
    if (token !== undefined && token.info.trim() === 'mermaid') {
      return `<MermaidDiagram code="${encodeURIComponent(token.content)}" />`
    }
    return fallback(tokens, idx, options, env, self)
  }
}
```

- [ ] **Step 8: Запустить тест**

Run: `npm test -- .vitepress/mermaid-fence.test.ts`
Expected: PASS.

- [ ] **Step 9: Создать `course/components/MermaidDiagram.vue`**

```vue
<script setup lang="ts">
import { onMounted, ref, useId, watch } from 'vue'
import { useData } from 'vitepress'
import { mermaidThemeVariables, withPaletteClasses } from '../viz/core/mermaid-palette'

const props = defineProps<{ code: string }>()
const { isDark } = useData()
const host = ref<HTMLElement>()
const error = ref<string | null>(null)
const baseId = `mmd-${useId().replace(/[^a-zA-Z0-9-]/g, '')}`
// Каждый рендер получает свой id: при смене темы на странице не появятся два SVG с одинаковым id.
let renderSeq = 0

async function render(): Promise<void> {
  const seq = ++renderSeq
  const theme = isDark.value ? 'dark' : 'light'
  const { default: mermaid } = await import('mermaid')
  mermaid.initialize({ startOnLoad: false, theme: 'base', themeVariables: mermaidThemeVariables(theme), securityLevel: 'strict' })
  try {
    const { svg } = await mermaid.render(`${baseId}-${seq}`, withPaletteClasses(decodeURIComponent(props.code), theme))
    if (seq !== renderSeq || host.value === undefined) return
    host.value.innerHTML = svg
    error.value = null
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

onMounted(render)
watch(isDark, render)
</script>

<template>
  <div class="mermaid-diagram">
    <div ref="host" class="mermaid-host" role="img" aria-label="Схема" />
    <pre v-if="error" class="mermaid-error">Ошибка схемы: {{ error }}</pre>
  </div>
</template>

<style scoped>
.mermaid-diagram { margin: 20px 0; }
/* Место под схему резервируется заранее, чтобы страница не прыгала после загрузки mermaid. */
.mermaid-host { min-height: 120px; display: flex; justify-content: center; overflow-x: auto; }
.mermaid-host :deep(svg) { max-width: 100%; height: auto; }
.mermaid-error { color: var(--vp-c-danger-1); white-space: pre-wrap; }
</style>
```

- [ ] **Step 10: Убрать плагин и подключить компонент**

- `course/package.json`: удалить `vitepress-plugin-mermaid`, выполнить `npm install`.
- `course/.vitepress/config.ts`:
  - убрать импорт и обёртку `withMermaid(...)`, вернуть `export default defineConfig({ ... })`;
  - добавить `import { mermaidFencePlugin } from './mermaid-fence'`;
  - заменить `markdown.config` на `(md) => { termsCheckPlugin(md, KNOWN_TERMS); mermaidFencePlugin(md) }`;
  - в `vite` оставить `optimizeDeps.include: ['mermaid']` и добавить комментарий и порог:

    ```ts
    // Mermaid грузится лениво отдельным чанком (~2 МБ), поэтому общий порог предупреждения поднят.
    // Размер основного чанка app.*.js проверяется в задаче 9.
    build: { chunkSizeWarningLimit: 2500 },
    ```
- `course/.vitepress/theme/index.ts`: зарегистрировать `MermaidDiagram` глобально.

- [ ] **Step 11: Дополнить `course/viz/README.md`**

Добавить два раздела:
- **«Классы Mermaid»** — таблица `client`, `service`, `db`, `cache`, `broker`, `network`, `deep` → смысл. Пример: `A[Телефон]:::client --> B[Go-сервис]:::service`. Пометка, что `classDef` добавляются автоматически, а цвета живут в `viz/core/mermaid-palette.ts`.
- **«Шкала задержек»** — `cpu` → `--viz-service`, `memory` → `--viz-cache`, `disk` → `--viz-db`, `network` → `--viz-network`. Пояснить, что здесь цвет означает уровень иерархии памяти, а не тип узла.

- [ ] **Step 12: Проверить**

Run: `npm test && npm run typecheck && npm run build && ls -la .vitepress/dist/assets/app.*.js && grep -l "mermaid" .vitepress/dist/assets/app.*.js || echo "в app-чанке mermaid нет"`

Expected:
- PASS и `build complete`;
- `app.*.js` заметно меньше прежних ~700 КБ;
- в выводе `в app-чанке mermaid нет`.

Через `npm run preview` и Playwright проверить:
- схемы уроков 0.1, 0.2 и 0.3 рисуются;
- переключение темы перерисовывает схему в новых цветах;
- `document.querySelectorAll('svg[id^="mmd-"]')` не содержит повторяющихся `id`.

- [ ] **Step 13: Commit**

```bash
git add course/package.json course/package-lock.json course/viz course/.vitepress course/components/MermaidDiagram.vue
git commit -m "feat(course): ленивый Mermaid в палитре курса вместо плагина

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Подписи с терминами, лёгкие подсказки, указатель глоссария, мелочи бэклога

**Files:**
- Create: `course/viz/core/caption.ts`, `course/viz/core/caption.test.ts`, `course/glossary/terms-short.data.ts`
- Modify:
  - `course/viz/core/types.ts`, `course/viz/components/Player.vue`;
  - `course/viz/simulations/request-path.ts`, `course/viz/simulations/request-path.test.ts`;
  - `course/glossary/schema.ts`, `course/glossary/schema.test.ts`;
  - `course/components/T.vue`, `course/components/GlossaryList.vue`, `course/glossary.md`, `course/.vitepress/theme/index.ts`;
  - `course/viz/core/format.ts`, `course/viz/core/format.test.ts`;
  - `course/viz/core/player.ts`, `course/viz/core/player.test.ts`;
  - `course/.vitepress/sidebar.ts`, `course/.vitepress/sidebar.test.ts`.

**Interfaces:**
- Produces:
  - `CaptionSegment { text: string; termId?: string }`, `Caption = string | readonly CaptionSegment[]`;
  - `captionSegments(c): readonly CaptionSegment[]`, `captionText(c): string`, `captionTermIds(c): string[]`;
  - `Step<S>.caption: Caption`;
  - `ShortTerm { term; en?; short }`, `toShortTerms(g: Glossary): Record<string, ShortTerm>`;
  - `groupByLetter<T extends Term>(terms: readonly T[]): { letter: string; terms: T[] }[]`;
  - data loader `glossary/terms-short.data.ts`.

- [ ] **Step 1: Написать падающий тест `course/viz/core/caption.test.ts`**

```ts
import { describe, expect, it } from 'vitest'
import { captionSegments, captionTermIds, captionText } from './caption'

describe('caption', () => {
  it('строка — один сегмент без термина', () => {
    expect(captionSegments('Привет')).toEqual([{ text: 'Привет' }])
    expect(captionText('Привет')).toBe('Привет')
    expect(captionTermIds('Привет')).toEqual([])
  })

  it('сегменты склеиваются в текст, термины собираются по порядку', () => {
    const c = [{ text: 'Прошёл один ' }, { text: 'RTT', termId: 'rtt' }, { text: ', а потом ' }, { text: 'TLS', termId: 'tls' }]
    expect(captionText(c)).toBe('Прошёл один RTT, а потом TLS')
    expect(captionTermIds(c)).toEqual(['rtt', 'tls'])
  })
})
```

- [ ] **Step 2: Запустить и убедиться, что тест падает**

Run: `npm test -- viz/core/caption.test.ts`
Expected: FAIL.

- [ ] **Step 3: Реализовать `course/viz/core/caption.ts` и обновить `types.ts`**

```ts
// Подпись шага: обычная строка или сегменты, часть из которых — термины глоссария.
export interface CaptionSegment {
  text: string
  termId?: string
}

export type Caption = string | readonly CaptionSegment[]

export function captionSegments(caption: Caption): readonly CaptionSegment[] {
  return typeof caption === 'string' ? [{ text: caption }] : caption
}

export function captionText(caption: Caption): string {
  return captionSegments(caption).map((s) => s.text).join('')
}

export function captionTermIds(caption: Caption): string[] {
  return captionSegments(caption).flatMap((s) => (s.termId === undefined ? [] : [s.termId]))
}
```

В `course/viz/core/types.ts` добавить `import type { Caption } from './caption'` и заменить `caption: string` на `caption: Caption`.

- [ ] **Step 4: Запустить тест**

Run: `npm test -- viz/core/caption.test.ts`
Expected: PASS.

- [ ] **Step 5: Player показывает термины в подписях**

В `course/viz/components/Player.vue` добавить импорты:

```ts
import T from '../../components/T.vue'
import { captionSegments } from '../core/caption'
```

В `<figcaption>` заменить `{{ step.caption }}` на:

```html
<template v-for="(seg, i) in captionSegments(step.caption)" :key="i"><T v-if="seg.termId" :id="seg.termId">{{ seg.text }}</T><template v-else>{{ seg.text }}</template></template>
```

- [ ] **Step 6: Сценарий пути запроса на сегментах (сначала тест)**

В `course/viz/simulations/request-path.test.ts`:
- заменить `step.caption.trim()` на `captionText(step.caption).trim()`;
- заменить `steps.at(-1)!.caption` на `captionText(steps.at(-1)!.caption)`;
- добавить тест:

  ```ts
  import { fileURLToPath } from 'node:url'
  import { loadGlossaryFile } from '../../glossary/load'
  import { captionTermIds, captionText } from '../core/caption'

  it('все термины в подписях есть в глоссарии, и они есть хотя бы в пяти шагах', () => {
    const known = new Set(Object.keys(loadGlossaryFile(fileURLToPath(new URL('../../glossary/terms.yaml', import.meta.url)))))
    const ids = steps.flatMap((s) => captionTermIds(s.caption))
    expect(ids.length).toBeGreaterThanOrEqual(5)
    for (const id of ids) expect(known.has(id)).toBe(true)
  })
  ```

Run: `npm test -- viz/simulations/request-path.test.ts`
Expected: FAIL на новом тесте (терминов 0).

Затем в `request-path.ts`:
- `caption` в `Segment` становится типа `Caption`;
- добавить хелпер `const term = (text: string, termId: string): CaptionSegment => ({ text, termId })`.

Перевести на сегменты подписи шагов:
- шаг 2 («DNS-запрос») — `dns`;
- шаг 4 («TCP-рукопожатие») — `tcp`;
- шаг 5 («RTT») — `rtt`;
- шаг 6 («TLS 1.3») — `tls`;
- шаг 9 («TLS-терминация») — `tls-termination`;
- шаг 12 («fsync») — `fsync`.

Текст подписей не менять, только разбить на сегменты. Финальная подпись остаётся строкой.

Run: `npm test -- viz/simulations/request-path.test.ts`
Expected: PASS.

- [ ] **Step 7: Лёгкие данные для подсказок и указатель (сначала тесты)**

В `course/glossary/schema.test.ts` добавить:

```ts
import { groupByLetter, toShortTerms } from './schema'

describe('toShortTerms', () => {
  it('оставляет только term, en и short', () => {
    const g = parseGlossary({ wal: { term: 'WAL', en: 'Write-Ahead Log', short: 'Журнал.', long: 'Подробно.', links: [{ title: 'x' }] } })
    expect(toShortTerms(g)).toEqual({ wal: { term: 'WAL', en: 'Write-Ahead Log', short: 'Журнал.' } })
  })
})

describe('groupByLetter', () => {
  it('группирует по первой букве: сначала кириллица, потом латиница', () => {
    const g = parseGlossary({
      dns: { term: 'DNS', short: '.' },
      lat: { term: 'Задержка', short: '.' },
      aud: { term: 'Аудит', short: '.' },
      bal: { term: 'балансировщик', short: '.' },
      fs: { term: 'fsync', short: '.' },
    })
    expect(groupByLetter(Object.values(g)).map((x) => [x.letter, x.terms.map((t) => t.id)])).toEqual([
      ['А', ['aud']],
      ['Б', ['bal']],
      ['З', ['lat']],
      ['D', ['dns']],
      ['F', ['fs']],
    ])
  })
})
```

Run: `npm test -- glossary/schema.test.ts`
Expected: FAIL.

Затем в `course/glossary/schema.ts`:

```ts
export interface ShortTerm {
  term: string
  en?: string
  short: string
}

export function toShortTerms(glossary: Glossary): Record<string, ShortTerm> {
  const result: Record<string, ShortTerm> = {}
  for (const [id, t] of Object.entries(glossary)) {
    result[id] = t.en === undefined ? { term: t.term, short: t.short } : { term: t.term, en: t.en, short: t.short }
  }
  return result
}

export function groupByLetter<T extends Term>(terms: readonly T[]): { letter: string; terms: T[] }[] {
  const groups: { letter: string; terms: T[] }[] = []
  for (const t of sortTerms(terms)) {
    const letter = t.term.charAt(0).toLocaleUpperCase('ru')
    const last = groups.at(-1)
    if (last !== undefined && last.letter === letter) last.terms.push(t)
    else groups.push({ letter, terms: [t] })
  }
  return groups
}
```

Run: `npm test -- glossary/schema.test.ts`
Expected: PASS.

- [ ] **Step 8: Loader `course/glossary/terms-short.data.ts`**

```ts
import { defineLoader } from 'vitepress'
import { loadGlossaryFile } from './load'
import { toShortTerms, type ShortTerm } from './schema'

// Подсказкам терминов на каждой странице нужны только короткие определения.
// Полный глоссарий с HTML грузится лишь на странице /glossary.
declare const data: Record<string, ShortTerm>
export { data }

export default defineLoader({
  watch: ['./terms.yaml'],
  load(watchedFiles: string[]): Record<string, ShortTerm> {
    const file = watchedFiles[0]
    if (file === undefined) throw new Error('glossary/terms.yaml не найден')
    return toShortTerms(loadGlossaryFile(file))
  },
})
```

- [ ] **Step 9: T.vue — лёгкие данные, неинтерактивная подсказка, правильный Esc**

В `course/components/T.vue`:
- импорт данных: `import { data as glossary } from '../glossary/terms-short.data'`;
- удалить `<a class="term-more" ...>` и его стили;
- вместо ссылки добавить `<span class="term-hint">Нажмите ещё раз — откроется глоссарий</span>` со стилем `display: none` и `@media (hover: none) { .term-hint { display: block; margin-top: 6px; font-size: 13px; color: var(--vp-c-text-2); } }`;
- `@mouseleave` → `onMouseLeave`, `@focusout` → `onFocusOut`:

  ```ts
  // Esc закрывает подсказку до тех пор, пока пользователь не уйдёт с термина:
  // уход мыши не считается уходом, если фокус всё ещё внутри.
  function onMouseLeave(): void {
    if (!root.value?.contains(document.activeElement)) dismissed.value = false
  }

  function onFocusOut(event: FocusEvent): void {
    if (!root.value?.contains(event.relatedTarget as Node | null)) dismissed.value = false
  }
  ```

Удалить `resetDismissed`.

- [ ] **Step 10: Глоссарий — локальный импорт и указатель**

В `course/.vitepress/theme/index.ts` убрать импорт и глобальную регистрацию `GlossaryList`: так полный глоссарий не попадает в общий чанк.

В `course/glossary.md` после frontmatter добавить:

```md
<script setup>
import GlossaryList from './components/GlossaryList.vue'
</script>
```

Текст вступления заменить на: «Все термины курса в одном месте: сначала русские, затем латинские названия. В уроках термины подчёркнуты пунктиром: наведите курсор или перейдите к термину клавишей Tab, чтобы увидеть короткое определение, а клик приведёт сюда. На телефоне первое нажатие показывает определение, второе открывает глоссарий. Esc закрывает подсказку.»

В `course/components/GlossaryList.vue`:
- `const groups = computed(() => groupByLetter(Object.values(glossary)))`;
- над списком — навигация `<nav class="letters" aria-label="Алфавитный указатель"><a v-for="g in groups" :key="g.letter" :href="`#letter-${g.letter}`">{{ g.letter }}</a></nav>`;
- затем для каждой группы `<h2 :id="`letter-${g.letter}`" class="letter">{{ g.letter }}</h2>` и прежний `<dl>` с терминами группы.

Стиль `.letters`: `display: flex; flex-wrap: wrap; gap: 6px;`. Ссылки — пилюли с `--vp-c-brand-soft`.

- [ ] **Step 11: Мелочи бэклога (сначала тесты)**

1. **`player.test.ts`.** Добавить в таблицу `playerReducer` строки:
   - `['seek NaN → начало', at(2), { type: 'seek', index: Number.NaN }, { index: 0 }]`;
   - `['seek Infinity → начало', at(2), { type: 'seek', index: Number.POSITIVE_INFINITY }, { index: 0 }]`.

   В `player.ts` в `clampIndex` первой строкой: `if (!Number.isFinite(index)) return 0`.
2. **`format.ts` и `format.test.ts`.** Литералы неразрывного пробела заменить на `' '`. В регулярном выражении — на `/[  ]/g`. Поведение не меняется, существующие тесты остаются зелёными.
3. **`sidebar.test.ts`.** Тест: урок с битым frontmatter (`---\ntitle: [\n---\n`) → `buildSidebar` бросает ошибку, текст которой содержит имя файла урока.

   В `sidebar.ts` чтение заголовка урока и модуля обернуть так:

   ```ts
   function titleOf(path: string, fallback: string): string {
     try {
       return readTitle(readFileSync(path, 'utf8'), fallback)
     } catch (e) {
       throw new Error(`${path}: не удалось разобрать frontmatter`, { cause: e })
     }
   }
   ```

Run: `npm test`
Expected: PASS.

- [ ] **Step 12: Проверить**

Run: `npm run typecheck && npm run build`
Expected: без ошибок.

Через `npm run preview` и Playwright проверить:
- в 0.3 на шаге 5 слово «RTT» в подписи подчёркнуто и показывает подсказку;
- `/glossary` показывает указатель, а клик по «F» прокручивает к `#letter-F`;
- в подсказке термина нет ссылки;
- `grep -c "longHtml" .vitepress/dist/assets/app.*.js` → `0`.

- [ ] **Step 13: Commit**

```bash
git add course
git commit -m "feat(course): термины в подписях визуализации, лёгкие подсказки, алфавитный указатель глоссария

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Полноэкранный режим — VizFrame

**Files:**
- Create: `course/viz/core/fullscreen.ts`, `course/viz/core/fullscreen.test.ts`, `course/viz/components/VizFrame.vue`
- Modify: `course/viz/scenes/RequestPath.vue`, `course/viz/scenes/LatencyScale.vue`, `course/viz/scenes/EstimationCalculator.vue`, `course/components/MermaidDiagram.vue`, `course/viz/README.md`

**Interfaces:**
- Consumes: `MermaidDiagram` (задача 2), `Player` (задача 3).
- Produces:
  - `FullscreenMode = 'off' | 'native' | 'overlay'`;
  - `FullscreenApi { enabled: boolean; request(el: HTMLElement): Promise<void>; exit(): Promise<void> }`;
  - `enterFullscreen(el, api): Promise<FullscreenMode>`;
  - `exitFullscreen(mode, api): Promise<'off'>`;
  - `<VizFrame label="...">` со слотом.

- [ ] **Step 1: Написать падающий тест `course/viz/core/fullscreen.test.ts`**

```ts
import { describe, expect, it, vi } from 'vitest'
import { enterFullscreen, exitFullscreen, type FullscreenApi } from './fullscreen'

const el = {} as HTMLElement
const api = (over: Partial<FullscreenApi> = {}): FullscreenApi => ({
  enabled: true,
  request: vi.fn().mockResolvedValue(undefined),
  exit: vi.fn().mockResolvedValue(undefined),
  ...over,
})

describe('enterFullscreen', () => {
  it('настоящий полноэкранный режим, если браузер разрешил', async () => {
    expect(await enterFullscreen(el, api())).toBe('native')
  })
  it('оверлей, если API недоступен (iPhone Safari)', async () => {
    const a = api({ enabled: false })
    expect(await enterFullscreen(el, a)).toBe('overlay')
    expect(a.request).not.toHaveBeenCalled()
  })
  it('оверлей, если браузер отказал', async () => {
    expect(await enterFullscreen(el, api({ request: vi.fn().mockRejectedValue(new Error('denied')) }))).toBe('overlay')
  })
})

describe('exitFullscreen', () => {
  it('из native вызывает exit', async () => {
    const a = api()
    expect(await exitFullscreen('native', a)).toBe('off')
    expect(a.exit).toHaveBeenCalledOnce()
  })
  it('из overlay не трогает API', async () => {
    const a = api()
    expect(await exitFullscreen('overlay', a)).toBe('off')
    expect(a.exit).not.toHaveBeenCalled()
  })
  it('если браузер уже вышел сам (Esc), ошибка exit не мешает', async () => {
    expect(await exitFullscreen('native', api({ exit: vi.fn().mockRejectedValue(new TypeError('not in fullscreen')) }))).toBe('off')
  })
})
```

- [ ] **Step 2: Запустить и убедиться, что тест падает**

Run: `npm test -- viz/core/fullscreen.test.ts`
Expected: FAIL.

- [ ] **Step 3: Реализовать `course/viz/core/fullscreen.ts`**

```ts
export type FullscreenMode = 'off' | 'native' | 'overlay'

export interface FullscreenApi {
  enabled: boolean
  request(el: HTMLElement): Promise<void>
  exit(): Promise<void>
}

// Если настоящий полноэкранный режим недоступен или браузер отказал,
// показываем визуализацию оверлеем поверх страницы — визуально то же самое.
export async function enterFullscreen(el: HTMLElement, api: FullscreenApi): Promise<FullscreenMode> {
  if (!api.enabled) return 'overlay'
  try {
    await api.request(el)
    return 'native'
  } catch {
    return 'overlay'
  }
}

export async function exitFullscreen(mode: FullscreenMode, api: FullscreenApi): Promise<'off'> {
  if (mode !== 'native') return 'off'
  try {
    await api.exit()
  } catch {
    // Браузер мог уже выйти сам (Esc) — тогда exit отклоняется, а цель и так достигнута.
  }
  return 'off'
}
```

- [ ] **Step 4: Запустить тест**

Run: `npm test -- viz/core/fullscreen.test.ts`
Expected: PASS (6 тестов).

- [ ] **Step 5: Создать `course/viz/components/VizFrame.vue`**

```vue
<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { enterFullscreen, exitFullscreen, type FullscreenApi, type FullscreenMode } from '../core/fullscreen'

const props = defineProps<{ label: string }>()
const frame = ref<HTMLElement>()
const mode = ref<FullscreenMode>('off')
const rotateHint = ref(false)

function browserApi(): FullscreenApi {
  return {
    enabled: document.fullscreenEnabled === true,
    request: (el) => el.requestFullscreen(),
    exit: () => document.exitFullscreen(),
  }
}

async function toggle(): Promise<void> {
  const el = frame.value
  if (el === undefined) return
  if (mode.value !== 'off') {
    mode.value = await exitFullscreen(mode.value, browserApi())
    return
  }
  mode.value = await enterFullscreen(el, browserApi())
  rotateHint.value = window.matchMedia('(max-width: 640px) and (orientation: portrait)').matches
  await nextTick()
  // Клавиши визуализации должны работать сразу после входа.
  el.querySelector<HTMLElement>('figure[tabindex="0"], [tabindex="0"], button, input, select')?.focus()
}

function onFullscreenChange(): void {
  if (mode.value === 'native' && document.fullscreenElement !== frame.value) mode.value = 'off'
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && mode.value === 'overlay') mode.value = 'off'
}

watch(mode, (m) => document.documentElement.classList.toggle('viz-scroll-lock', m === 'overlay'))

onMounted(() => {
  document.addEventListener('fullscreenchange', onFullscreenChange)
  document.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  document.removeEventListener('keydown', onKeydown)
  document.documentElement.classList.remove('viz-scroll-lock')
})
</script>

<template>
  <div
    ref="frame"
    class="viz-frame"
    :class="{ 'is-fullscreen': mode !== 'off', 'is-overlay': mode === 'overlay' }"
    :role="mode === 'overlay' ? 'dialog' : undefined"
    :aria-modal="mode === 'overlay' ? 'true' : undefined"
    :aria-label="props.label"
  >
    <div class="viz-frame-bar">
      <button type="button" class="viz-fs-btn" @click="toggle">
        <span aria-hidden="true">{{ mode === 'off' ? '⛶' : '✕' }}</span>
        {{ mode === 'off' ? 'На весь экран' : 'Выйти (Esc)' }}
      </button>
    </div>
    <p v-if="mode !== 'off' && rotateHint" class="viz-rotate">
      Поверните телефон горизонтально — так визуализация видна лучше.
      <button type="button" aria-label="Скрыть подсказку" @click="rotateHint = false">✕</button>
    </p>
    <slot />
  </div>
</template>

<style scoped>
.viz-frame { position: relative; }
.viz-frame-bar { display: flex; justify-content: flex-end; margin-bottom: -8px; }
.viz-fs-btn { padding: 4px 10px; border: 1px solid var(--vp-c-divider); border-radius: 8px; background: var(--vp-c-bg-soft); color: var(--vp-c-text-2); font-size: 13px; }
.viz-fs-btn:hover { color: var(--vp-c-brand-1); border-color: var(--vp-c-brand-1); }
.viz-fs-btn:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
.is-fullscreen { display: flex; flex-direction: column; gap: 8px; padding: 16px; background: var(--vp-c-bg); overflow: auto; }
.is-overlay { position: fixed; inset: 0; z-index: 100; }
.is-fullscreen .viz-frame-bar { margin-bottom: 0; }
.is-fullscreen :deep(.viz-player), .is-fullscreen :deep(.latency), .is-fullscreen :deep(.calc), .is-fullscreen :deep(.mermaid-diagram) { margin: 0; }
.is-fullscreen :deep(svg) { max-height: calc(100vh - 240px); }
.is-fullscreen :deep(.viz-caption) { font-size: 1.15rem; }
.viz-rotate { display: flex; gap: 8px; align-items: center; margin: 0; padding: 8px 12px; border-radius: 8px; background: var(--vp-c-brand-soft); font-size: 14px; }
:global(html.viz-scroll-lock) { overflow: hidden; }
</style>
```

- [ ] **Step 6: Обернуть визуализации и схемы**

- `RequestPath.vue`: корень `<VizFrame label="Путь запроса «Отправить сообщение»">`, внутри — прежний `<Player>`.
- `LatencyScale.vue`: корень `<VizFrame label="Шкала задержек">`, внутри — прежний `<section>`.
- `EstimationCalculator.vue`: корень `<VizFrame label="Калькулятор оценки нагрузки">`, внутри — прежний `<section>`.
- `MermaidDiagram.vue`: корень `<VizFrame label="Схема">`, внутри — прежний `<div class="mermaid-diagram">`.

Импорт: `import VizFrame from '../components/VizFrame.vue'` (для Mermaid — `'../viz/components/VizFrame.vue'`).

- [ ] **Step 7: Дополнить `course/viz/README.md`**

Раздел «Полноэкранный режим»:
- каждая сцена и схема оборачивается в `VizFrame`;
- в полноэкранном режиме контейнер плеера шире 900 px, и код встаёт справа сам — через container query;
- оверлей — запасной вариант для iPhone.

- [ ] **Step 8: Проверить**

Run: `npm test && npm run typecheck && npm run build`
Expected: PASS.

Через `npm run preview` и Playwright, на 0.3, 0.4, 0.5 и карте курса в 0.1:
- клик ⛶ → `document.fullscreenElement` равен обёртке;
- в 0.3 при ширине 1280 код справа от схемы;
- ← → меняют шаг;
- Esc (`browser_press_key Escape`) → режим выключен, кнопка снова «На весь экран».

Затем эмуляция отказа: `browser_evaluate` с `Element.prototype.requestFullscreen = () => Promise.reject(new Error('x'))` → клик ⛶ → обёртка с классом `is-overlay`, на `<html>` есть `viz-scroll-lock`. После Esc класса нет.

Затем снова войти в оверлей, перейти на другой урок и проверить: на `<html>` нет `viz-scroll-lock`.

- [ ] **Step 9: Commit**

```bash
git add course/viz course/components/MermaidDiagram.vue
git commit -m "feat(viz): полноэкранный режим для визуализаций и схем

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Проверка Go-фрагментов и обработчик уровня прода

**Files:**
- Create: `course/scripts/go-snippets.ts`, `course/scripts/go-snippets.test.ts`, `course/scripts/check-go-snippets.ts`, `course/scripts/go-harness/send-message.go.tmpl`, `course/scripts/go-harness/sql-pool.go.tmpl`
- Modify:
  - `course/package.json`, `course/tsconfig.json`, `course/vitest.config.ts`;
  - `course/components/Callout.vue`;
  - `course/viz/simulations/request-path.ts`, `course/viz/simulations/request-path.test.ts`;
  - `course/modules/00-foundation/02-what-is-system-design.md`, `03-request-path.md`, `04-latency.md`, `05-estimation.md`.

**Interfaces:**
- Produces:
  - `GoSnippet { file: string; line: number; harness: string | null; code: string }`;
  - `extractGoSnippets(markdown: string, file: string): GoSnippet[]`;
  - `findUnmarkedGoBlocks(markdown: string): number[]` — номера строк 1-based;
  - `buildGoFile(snippet: GoSnippet, harnessTemplate: string | null): string`;
  - скрипт `npm run check:go`;
  - тип `simplified` в `Callout`;
  - новый `CODE_LINE = { entry: 6, auth: 7, execute: 29, respond: 44 }`.

- [ ] **Step 1: Настройки**

- `course/vitest.config.ts`: добавить `'scripts/**/*.test.ts'` в `include`.
- `course/tsconfig.json`: добавить `"scripts/**/*.ts"` в `include` и `"allowImportingTsExtensions": true` в `compilerOptions` (`noEmit` уже включён).
- `course/package.json` → `scripts`: `"check:go": "node scripts/check-go-snippets.ts"`. Node 26 запускает TypeScript без сборки, удаляя типы.

- [ ] **Step 2: Написать падающий тест `course/scripts/go-snippets.test.ts`**

````ts
import { describe, expect, it } from 'vitest'
import { buildGoFile, extractGoSnippets, findUnmarkedGoBlocks } from './go-snippets'

const md = [
  'Текст',
  '<!-- go-check -->',
  '',
  '```go',
  'package main',
  '',
  'func main() {}',
  '```',
  '',
  '<!-- go-check: harness=sql-pool -->',
  '```go',
  'db.SetMaxOpenConns(80)',
  '```',
  '',
  '```go',
  'x := 1',
  '```',
].join('\n')

describe('extractGoSnippets', () => {
  it('находит помеченные блоки, обвязку и строку начала', () => {
    expect(extractGoSnippets(md, 'a.md')).toEqual([
      { file: 'a.md', line: 4, harness: null, code: 'package main\n\nfunc main() {}' },
      { file: 'a.md', line: 11, harness: 'sql-pool', code: 'db.SetMaxOpenConns(80)' },
    ])
  })
})

describe('findUnmarkedGoBlocks', () => {
  it('возвращает строки блоков go без маркера', () => {
    expect(findUnmarkedGoBlocks(md)).toEqual([15])
  })
})

describe('buildGoFile', () => {
  const snip = (code: string, harness: string | null = null) => ({ file: 'a.md', line: 1, harness, code })

  it('полная программа остаётся как есть', () => {
    expect(buildGoFile(snip('package main\n\nfunc main() {}'), null)).toBe('package main\n\nfunc main() {}\n')
  })

  it('без package и без обвязки получает package snippet', () => {
    expect(buildGoFile(snip('func f() {}'), null)).toBe('package snippet\n\nfunc f() {}\n')
  })

  it('вставляет фрагмент на место // SNIPPET с отступом этой строки', () => {
    const tmpl = 'package snippet\n\nfunc f() {\n\t// SNIPPET\n}\n'
    expect(buildGoFile(snip('a := 1\n\nb := a', 'x'), tmpl)).toBe('package snippet\n\nfunc f() {\n\ta := 1\n\n\tb := a\n}\n')
  })

  it('обвязка без // SNIPPET — понятная ошибка с файлом и строкой', () => {
    expect(() => buildGoFile(snip('x', 'x'), 'package snippet\n')).toThrow(/a\.md:1.*SNIPPET/)
  })
})
````

- [ ] **Step 3: Запустить и убедиться, что тест падает**

Run: `npm test -- scripts/go-snippets.test.ts`
Expected: FAIL.

- [ ] **Step 4: Реализовать `course/scripts/go-snippets.ts`**

````ts
export interface GoSnippet {
  file: string
  line: number
  harness: string | null
  code: string
}

const MARKER = /^<!--\s*go-check(?::\s*harness=([a-z0-9-]+))?\s*-->\s*$/
const GO_FENCE = /^```go\s*$/
const FENCE_END = /^```\s*$/

interface Block {
  start: number
  code: string
  marker: { harness: string | null } | null
}

function goBlocks(markdown: string): Block[] {
  const lines = markdown.split('\n')
  const blocks: Block[] = []
  for (let i = 0; i < lines.length; i++) {
    if (!GO_FENCE.test(lines[i] ?? '')) continue
    let end = i + 1
    while (end < lines.length && !FENCE_END.test(lines[end] ?? '')) end++
    // Маркер — ближайшая непустая строка над блоком.
    let prev = i - 1
    while (prev >= 0 && (lines[prev] ?? '').trim() === '') prev--
    const m = prev >= 0 ? MARKER.exec((lines[prev] ?? '').trim()) : null
    blocks.push({ start: i + 1, code: lines.slice(i + 1, end).join('\n'), marker: m ? { harness: m[1] ?? null } : null })
    i = end
  }
  return blocks
}

export function extractGoSnippets(markdown: string, file: string): GoSnippet[] {
  return goBlocks(markdown).flatMap((b) => (b.marker === null ? [] : [{ file, line: b.start, harness: b.marker.harness, code: b.code }]))
}

export function findUnmarkedGoBlocks(markdown: string): number[] {
  return goBlocks(markdown).filter((b) => b.marker === null).map((b) => b.start)
}

const PLACEHOLDER = /^([ \t]*)\/\/ SNIPPET[ \t]*$/m

export function buildGoFile(snippet: GoSnippet, harnessTemplate: string | null): string {
  if (harnessTemplate === null) {
    const body = snippet.code.startsWith('package ') ? snippet.code : `package snippet\n\n${snippet.code}`
    return `${body}\n`
  }
  const m = PLACEHOLDER.exec(harnessTemplate)
  if (m === null) throw new Error(`${snippet.file}:${snippet.line}: в обвязке «${snippet.harness}» нет строки // SNIPPET`)
  const indent = m[1] ?? ''
  const indented = snippet.code.split('\n').map((l) => (l.trim() === '' ? '' : indent + l)).join('\n')
  // Функция-замена: в Go-коде может встретиться `$`, который строковая замена приняла бы за шаблон.
  return harnessTemplate.replace(PLACEHOLDER, () => indented)
}
````

- [ ] **Step 5: Запустить тест**

Run: `npm test -- scripts/go-snippets.test.ts`
Expected: PASS.

- [ ] **Step 6: Скрипт `course/scripts/check-go-snippets.ts`**

```ts
import { execFileSync } from 'node:child_process'
import { existsSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { buildGoFile, extractGoSnippets, findUnmarkedGoBlocks } from './go-snippets.ts'

const ROOT = fileURLToPath(new URL('..', import.meta.url))
const MODULES = join(ROOT, 'modules')
const HARNESS = join(ROOT, 'scripts', 'go-harness')

const files = readdirSync(MODULES, { recursive: true, encoding: 'utf8' })
  .filter((f) => f.endsWith('.md'))
  .map((f) => join(MODULES, f))

const failures: string[] = []
let checked = 0

for (const path of files) {
  const md = readFileSync(path, 'utf8')
  const rel = relative(ROOT, path)
  for (const line of findUnmarkedGoBlocks(md)) failures.push(`${rel}:${line}: блок go без маркера <!-- go-check -->`)
  for (const snippet of extractGoSnippets(md, rel)) {
    checked++
    let template: string | null = null
    if (snippet.harness !== null) {
      const tmplPath = join(HARNESS, `${snippet.harness}.go.tmpl`)
      if (!existsSync(tmplPath)) {
        failures.push(`${rel}:${snippet.line}: нет обвязки ${relative(ROOT, tmplPath)}`)
        continue
      }
      template = readFileSync(tmplPath, 'utf8')
    }
    const dir = mkdtempSync(join(tmpdir(), 'go-snippet-'))
    try {
      writeFileSync(join(dir, 'go.mod'), 'module snippet\n\ngo 1.26\n')
      writeFileSync(join(dir, 'main.go'), buildGoFile(snippet, template))
      const unformatted = execFileSync('gofmt', ['-l', 'main.go'], { cwd: dir, encoding: 'utf8' }).trim()
      if (unformatted !== '') failures.push(`${rel}:${snippet.line}: gofmt требует форматирования`)
      execFileSync('go', ['vet', './...'], { cwd: dir, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
    } catch (e) {
      const err = e as { stderr?: string; message: string }
      failures.push(`${rel}:${snippet.line}: ${err.stderr?.trim() || err.message}`)
    } finally {
      rmSync(dir, { recursive: true, force: true })
    }
  }
}

if (failures.length > 0) {
  console.error(failures.join('\n'))
  process.exit(1)
}
console.log(`Проверено Go-фрагментов: ${checked}, ошибок нет.`)
```

- [ ] **Step 7: Обвязки**

`course/scripts/go-harness/sql-pool.go.tmpl`:

```go
package snippet

import "database/sql"

func configurePool(db *sql.DB) {
	// SNIPPET
}
```

`course/scripts/go-harness/send-message.go.tmpl` — минимальные типы, без которых не компилируется обработчик урока 0.3:

```go
package snippet

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
	"unicode/utf8"
)

// Обвязка для проверки фрагмента урока 0.3: минимальные типы, без которых он не компилируется.

type chatErrors struct{ ErrNotMember error }

// chat имитирует доменный пакет chat с его ошибками.
var chat = chatErrors{ErrNotMember: errors.New("not a chat member")}

type Message struct{ ID int64 }

type MessageResponse struct {
	ID int64 `json:"id"`
}

func toResponse(m Message) MessageResponse { return MessageResponse{ID: m.ID} }

const maxTextRunes = 4096

type SendMessageRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func (r SendMessageRequest) Validate() error {
	if n := utf8.RuneCountInString(r.Text); n == 0 || n > maxTextRunes {
		return errors.New("text must be 1..4096 characters")
	}
	return nil
}

type authenticator interface {
	UserID(r *http.Request) (int64, error)
}

type messageSender interface {
	Execute(ctx context.Context, userID, chatID int64, text string) (Message, error)
}

type Handler struct {
	auth authenticator
	send messageSender
	log  *slog.Logger
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		slog.Warn("encode error response", "err", err)
	}
}

var _ = time.Second

// SNIPPET
```

- [ ] **Step 8: Новый обработчик в уроке 0.3**

В `03-request-path.md` перед блоком ` ```go ` внутри `<RequestPath>` поставить строку `<!-- go-check: harness=send-message -->` и пустую строку. Блок заменить на:

```go
const (
	maxBodyBytes   = 64 << 10 // 64 КБ: текст сообщения и метаданные с большим запасом
	executeTimeout = 2 * time.Second
)

func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.UserID(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var req SendMessageRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), executeTimeout)
	defer cancel()

	msg, err := h.send.Execute(ctx, userID, req.ChatID, req.Text)
	switch {
	case errors.Is(err, chat.ErrNotMember):
		writeError(w, http.StatusForbidden, "not a chat member")
		return
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, "timeout")
		return
	case err != nil:
		h.log.ErrorContext(ctx, "send message", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(toResponse(msg)); err != nil {
		h.log.WarnContext(ctx, "encode response", "err", err)
	}
}
```

В `request-path.ts` поставить `CODE_LINE = { entry: 6, auth: 7, execute: 29, respond: 44 } as const`. Тест на расхождение (`request-path.test.ts`, блок `CODE_LINE и Go-код урока 0.3`) должен остаться зелёным. Если он ищет первый блок ` ```go `, маркер ему не мешает.

- [ ] **Step 9: Плашка «Упрощено»**

В `course/components/Callout.vue`:
- добавить `simplified: { icon: '⚠️', title: 'Упрощено' }` в `KINDS` и `'simplified'` в тип `Kind`;
- добавить стиль `.callout-simplified { border-color: var(--vp-c-warning-1); background: var(--vp-c-warning-soft); }`.

В `03-request-path.md` сразу после `</RequestPath>` добавить `<Callout type="simplified">` с перечнем, что в настоящем сервисе устроено иначе:
- **аутентификация** — middleware, кладёт пользователя в `context`; обработчик берёт его оттуда (модуль 1);
- **логирование запросов, метрики и трассировка** — middleware (модуль 12);
- **`Validate` и DTO** — в HTTP-адаптере, отдельно от доменной модели (модуль 1);
- **сопоставление доменных ошибок со статусами** — в одной функции адаптера, а не в каждом обработчике;
- **таймаут** — из конфигурации, а не константа.

Текст урока, описывающий обработчик (раздел про Go-сервис в «Разборе по шагам»), обновить под новый код: лимит тела, `DisallowUnknownFields`, валидация, таймаут через `context`, `errors.Is`, проверка ошибки `Encode`. Что делает каждая строка — объяснить коротко.

- [ ] **Step 10: Маркеры у остальных фрагментов**

Перед блоками ` ```go ` поставить:
- в `02-what-is-system-design.md` — `<!-- go-check -->`;
- в `05-estimation.md` — `<!-- go-check -->`;
- в `04-latency.md` — `<!-- go-check: harness=sql-pool -->`.

- [ ] **Step 11: Проверить**

Run: `npm test && npm run typecheck && npm run check:go && npm run build`
Expected: тесты PASS; `Проверено Go-фрагментов: 4, ошибок нет.`; `build complete`.

Если `go vet` или `gofmt` находят проблему во фрагменте урока — исправить фрагмент, а не обвязку. Обвязку менять только если в ней не хватает типа.

Проверка на дурака: временно удалить маркер в 0.5 → `npm run check:go` падает с `modules/00-foundation/05-estimation.md:<строка>: блок go без маркера`. Вернуть маркер.

- [ ] **Step 12: Commit**

```bash
git add course
git commit -m "feat(course): проверка Go-фрагментов, обработчик уровня прода в уроке 0.3

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Уроки 0.1 и 0.2 — метки, числа, «Под капотом», источники

**Files:**
- Modify: `course/modules/00-foundation/01-how-to-study.md`, `course/modules/00-foundation/02-what-is-system-design.md`, при необходимости `course/glossary/terms.yaml`

**Interfaces:**
- Consumes: `<Part>`, `<N>`, `<Deep>` (задача 1); классы Mermaid (задача 2).

**Общие правила для задач 6–8** (повторены в каждой):
1. **Метки.** Каждый заголовок `##` начинается с `<Part kind="…" />`. Соответствие:
   - «Проблема…» → `problem`;
   - «Визуализация…», «Калькулятор», «Шкала» → `viz`;
   - «Типичные ошибки» → `mistakes`;
   - «Компромисс…» → `tradeoff`;
   - разделы, где главное — код → `code`;
   - «Самопроверка», «Мини-интервью» → `selfcheck`;
   - «Источники» → `sources`;
   - остальное → `theory`.

   Текст заголовков не менять: от него зависят якоря.
2. **Числа.** 3–8 ключевых чисел урока (которые урок просит запомнить или сравнить) — в `<N>`. Внутри `<T>`, формул и таблиц `<N>` не ставить.
3. **«Под капотом».** Блоки `<Deep title="…">` из таблицы ниже, сразу после места, которое углубляют.
   - Объём каждого — 150–500 слов.
   - Голос автора, термины через `<T>` или с объяснением в тексте, числа с источником, оценки помечены.
   - Заголовков внутри `<Deep>` нет.
4. **Самодостаточность.** Перечитать урок и найти места, где ученику пришлось бы идти в книгу (термин не объяснён, механизм назван, но не показан, число без происхождения). Закрыть в основном тексте или в `<Deep>`. Список закрытых мест — в отчёт.
5. **Источники.** Каждая строка в формате «Автор, «Название», глава — что там найти дополнительно».
6. **Схемы Mermaid.** Используют классы `client`, `service`, `db`, `cache`, `broker`, `network`, `deep` вместо собственных `classDef` и стилей. Собственные `classDef` удалить.
7. **Проверка.** `npm test && npm run check:go && npm run build`, затем Playwright: страница в тёмной и светлой теме, ширина 375 и 1280 px. Проверить, что метки видны, `<Deep>` раскрывается с клавиатуры (Tab → Enter) и горизонтальной прокрутки нет.

- [ ] **Step 1: Урок 0.1 — метки, числа, схема**

Применить правила 1, 2, 5 и 6. Карта курса: узлы ядра — `:::service`, узлы углубления — `:::deep`. Собственный `classDef` углубления удалить, пометку «(У)» в подписях оставить.

- [ ] **Step 2: Урок 0.1 — «Под капотом: почему вспоминание работает лучше перечитывания»**

Поставить после раздела про активное воспроизведение. Обязательное содержание:
- **Кривая забывания** (Эббингауз, 1885). Без повторения большая часть материала забывается уже в первые сутки. Форма кривой — быстрое падение и затем медленное. Точные проценты зависят от материала, поэтому подать как форму, а не как числа.
- **Эффект тестирования.** Roediger & Karpicke, 2006, «Test-Enhanced Learning». Студенты, которые после первого чтения трижды вспоминали текст, через неделю помнили около 61%. Те, кто трижды перечитывал, — около 40%. Сразу после занятия перечитывание выглядело выигрышнее — в этом и есть иллюзия.
- **Эффект интервалов.** Повторение с растущими паузами лучше «зубрёжки подряд».
- **Желательные трудности** (Бьорк). Если вспоминать трудно, это не признак неудачи: усилие и закрепляет память.
- **Практический вывод для этого курса:** вопросы самопроверки, мини-интервью, возврат к уроку через неделю.

Числа Roediger & Karpicke подать как результат конкретного эксперимента, а не как универсальный закон. Источник добавить в «Источники» по правилу 5.

- [ ] **Step 3: Урок 0.2 — метки, числа, схемы**

Применить правила 1, 2, 5 и 6. Числа для `<N>`:
- таблица «девяток»;
- 0,997 у последовательной цепочки;
- 0,9999 у резерва;
- бюджет 43 минуты;
- p50 / p90 в примере с перцентилями.

Схемы цепочки и резерва — узлы `:::service`.

- [ ] **Step 4: Урок 0.2 — «Под капотом: коррелированные отказы»**

Поставить сразу после формулы 1 − (1 − a)². Обязательное содержание:
- **Почему формула — оценка сверху.** Она верна, только если отказы независимы.
- **Общие причины отказа** (common-mode failure): один коммутатор или стойка, одно питание, одна зона доступности, одна ошибка в коде на всех копиях, одна неудачная выкатка конфигурации.
- **Пример на числах.** Два сервера по 99% за общим коммутатором с доступностью 99,9%: (1 − 0,01²) × 0,999 = 0,9999 × 0,999 ≈ 0,9989. Общий коммутатор съел почти весь выигрыш от резерва, и доступность ниже, чем у одного коммутатора.
- **Правило оценки.** Доступность системы не выше доступности самой слабой общей зависимости. Чтобы резерв работал, копии разносят по независимым доменам отказа (стойки, зоны, регионы) и выкатывают изменения не на все копии сразу. Ссылки вперёд: модуль 8 (репликация, failover), модуль 13b (canary).

- [ ] **Step 5: Самодостаточность 0.1 и 0.2 (правило 4)**

- [ ] **Step 6: Проверить (правило 7) и закоммитить**

```bash
git add course/modules/00-foundation/01-how-to-study.md course/modules/00-foundation/02-what-is-system-design.md course/glossary/terms.yaml
git commit -m "feat(course): уроки 0.1–0.2 — метки, ключевые числа, «Под капотом», источники

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Уроки 0.3 и 0.4 — метки, числа, «Под капотом», источники

**Files:**
- Modify: `course/modules/00-foundation/03-request-path.md`, `course/modules/00-foundation/04-latency.md`, при необходимости `course/glossary/terms.yaml`

**Interfaces:**
- Consumes: `<Part>`, `<N>`, `<Deep>`, классы Mermaid; обработчик и плашка «Упрощено» из задачи 5. Код обработчика не менять: от него зависят `CODE_LINE` и `check:go`.

**Общие правила** — те же 7 пунктов, что в задаче 6 (метки → `Part`; 3–8 чисел в `N`; «Под капотом» 150–500 слов после углубляемого места; самодостаточность; формат источников; классы Mermaid; проверка в обеих темах на 375 и 1280 px).

- [ ] **Step 1: Урок 0.3 — метки, числа, схема**

Числа для `<N>`:
- 144 мс;
- 4 мс на сервер и базу;
- 40 мс RTT;
- 2 мс fsync;
- ≈44 мс у второго сообщения по открытому соединению.

Схема участников:
- Телефон — `:::client`;
- DNS и балансировщик — `:::network`;
- Go-сервис — `:::service`;
- PostgreSQL — `:::db`.

- [ ] **Step 2: Урок 0.3 — четыре блока «Под капотом»**

1. **«Как DNS находит адрес»** — после раздела про DNS.
   - **Цепочка:** стаб-резолвер ОС (в Go — встроенный резолвер или cgo, переключается `GODEBUG=netdns=go|cgo`) → рекурсивный резолвер (провайдера или публичный, например 1.1.1.1) → корневые серверы → серверы зоны верхнего уровня (`.com`) → авторитетный сервер домена.
   - **Корневые серверы:** 13 имён (a–m.root-servers.net), за которыми стоят сотни экземпляров по всему миру благодаря anycast.
   - **Кэш:** каждый ответ кэшируется на время своего TTL, кэшируются и отрицательные ответы. Поэтому полный проход по цепочке — редкость, обычно ответ приходит из кэша резолвера.
2. **«Что происходит в TCP-рукопожатии»** — после раздела про TCP.
   - **Порядковые номера.** Каждая сторона выбирает случайный начальный номер (ISN). `SYN` несёт номер клиента, `SYN-ACK` — номер сервера и подтверждение «жду байт ISN+1». Трёх пакетов достаточно, чтобы обе стороны узнали и подтвердили номера друг друга.
   - **Состояния:** `LISTEN` → `SYN_SENT` / `SYN_RECEIVED` → `ESTABLISHED`; при закрытии — `FIN_WAIT`, `TIME_WAIT`.
   - **Потерянный пакет** пересылается по таймеру: начальный RTO — 1 с (RFC 6298), дальше он адаптируется к измеренному RTT. Потеря `SYN` на старте поэтому стоит секунду.
   - **`TIME_WAIT`** держит закрытое соединение некоторое время (в Linux — 60 с), чтобы опоздавшие пакеты не попали в новое соединение. Ссылка на модуль 0b.
3. **«Обмен ключами в TLS 1.3»** — после раздела про TLS.
   - **ClientHello** несёт `key_share` — открытую часть ключа по схеме ECDHE (обычно X25519).
   - **Сервер** в ответ присылает свою `key_share`. После этого обе стороны вычисляют общий секрет, ни разу не передав его по сети.
   - **Остаток рукопожатия** (сертификат, `CertificateVerify`, `Finished`) уже зашифрован.
   - **Проверка сертификата:** цепочка подписей до корневого центра сертификации из хранилища ОС, имя домена, срок действия.
   - **Прямая секретность (forward secrecy):** утечка ключа сервера в будущем не раскрывает прошлый трафик, потому что ключи сессии одноразовые.
   - Источник — RFC 8446.
4. **«Куда на самом деле пишет fsync»** — после раздела про запись в БД.
   - **`write()`** кладёт данные в page cache ОС. **`fsync()`** просит ОС отправить их на устройство и дождаться подтверждения.
   - **Кэш записи у самого диска** — ещё один уровень. Команда сброса (flush / FUA) заставляет диск записать данные надёжно.
   - **Защита от потери питания (PLP)** у серверных SSD позволяет диску подтверждать запись сразу из защищённого кэша, поэтому fsync там — десятые доли миллисекунды. На потребительских SSD без PLP и на HDD — миллисекунды.
   - **В PostgreSQL** — параметр `wal_sync_method` и опасность `fsync = off`: быстрее, но после сбоя питания база может быть повреждена.
   - **Как измерить:** `pg_test_fsync`. Связь с модулем 2b (WAL).

- [ ] **Step 3: Урок 0.4 — метки, числа**

Числа для `<N>`:
- 1 нс у L1;
- 100 нс у RAM;
- 16 мкс у SSD;
- 0,5 мс внутри ЦОД;
- 150 мс через океан;
- 100 одновременных запросов в примере закона Литтла.

- [ ] **Step 4: Урок 0.4 — три блока «Под капотом»**

1. **«Почему последовательное чтение быстрее: локальность и кэш-линии»** — после вывода про последовательное и случайное чтение.
   - **Кэш-линия.** Процессор читает память кэш-линиями по 64 байта (на распространённых x86-64 и ARM).
   - **Пространственная локальность** (рядом лежащее понадобится скоро) и **временная** (недавнее понадобится снова). Аппаратный prefetcher угадывает последовательный доступ и подгружает следующие линии заранее.
   - **В Go:** проход по срезу `[]T` использует каждую загруженную линию полностью. Обход связного списка или `map` прыгает по памяти и чаще ждёт RAM.
   - **Аналогия:** диск и сеть устроены так же — читать подряд дешевле, чем вразнобой.
2. **«Как устроен SSD и почему случайная запись дорогая»** — после строки про SSD.
   - **Устройство памяти.** NAND-память пишется страницами (порядка 4–16 КБ), а стирается только целыми блоками (сотни страниц, порядка мегабайт). Записать поверх старых данных нельзя.
   - **Слой трансляции (FTL).** Контроллер пишет новую версию в свободную страницу, старую помечает недействительной, а потом сборщик мусора переносит живые страницы и стирает блок.
   - **Усиление записи (write amplification):** физически пишется больше, чем просило приложение. Сильнее всего это бьёт по мелким случайным записям.
   - **TRIM** сообщает диску, какие страницы больше не нужны.
   - **Вывод:** журналы и LSM-деревья пишут последовательно не случайно. Ссылка на модуль 2b и Петрова, «Распределённые данные» (Database Internals), часть 1.
   - Числа подать как порядки величин.
3. **«Откуда берётся закон Литтла»** — после формулы L = λW.
   - **Рисунок:** график «сколько запросов сейчас в системе» за долгое время T.
   - **Площадь под графиком** — это сумма времени, которое провёл в системе каждый запрос, то есть N × W (N запросов, каждый в среднем W).
   - **Среднее по времени** L = площадь / T = (N / T) × W = λ × W.
   - **Почему закон не зависит от распределения и порядка:** площадь одинаково считается и по «вертикали», и по «горизонтали».
   - **Условие** — устойчивость: на длинном отрезке ушло столько же, сколько пришло.
   - Источник: Little, 1961.

- [ ] **Step 5: Самодостаточность 0.3 и 0.4 (правило 4)**

- [ ] **Step 6: Проверить и закоммитить**

Run: `npm test && npm run check:go && npm run build` + Playwright (правило 7).

```bash
git add course/modules/00-foundation/03-request-path.md course/modules/00-foundation/04-latency.md course/glossary/terms.yaml
git commit -m "feat(course): уроки 0.3–0.4 — метки, ключевые числа, «Под капотом», источники

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Уроки 0.5 и 0.6 — метки, числа, «Под капотом», источники

**Files:**
- Modify: `course/modules/00-foundation/05-estimation.md`, `course/modules/00-foundation/06-messenger-requirements.md`, при необходимости `course/glossary/terms.yaml`

**Interfaces:**
- Consumes: `<Part>`, `<N>`, `<Deep>`, классы Mermaid.

**Общие правила** — те же 7 пунктов, что в задаче 6 (метки → `Part`; 3–8 чисел в `N`; «Под капотом» 150–500 слов после углубляемого места; самодостаточность; формат источников; классы Mermaid; проверка в обеих темах на 375 и 1280 px).

- [ ] **Step 1: Урок 0.5 — метки, числа**

Числа для `<N>`:
- 86 400 ≈ 10⁵ с;
- 1 млн запросов в сутки ≈ 12 RPS;
- ≈ 231 / 463 RPS;
- 4,4 ТБ.

- [ ] **Step 2: Урок 0.5 — три блока «Под капотом»**

1. **«Нагрузка на чтение»** — после расчёта RPS записи.
   - **Почему калькулятор считает запись, а мессенджер в основном читает:** каждое сообщение доставляется получателям, история прокручивается, чаты открываются заново.
   - **Отношение чтение/запись** оценивается допущением (например, 10:1 — явно помечено как допущение). Пиковый RPS чтения считается так же, как записи.
   - **Почему чтение и запись масштабируют по-разному:** реплики для чтения и кэш против шардирования для записи. Ссылки на модули 4, 8 и 9.
2. **«Биты и байты в каналах»** — после расчёта трафика.
   - **Каналы измеряют в битах в секунду,** данные — в байтах. 1 Гбит/с = 125 МБ/с.
   - **Накладные расходы протоколов** (заголовки TCP/IP, TLS, HTTP) съедают часть канала — оценочно порядка 5–10% на крупных передачах и намного больше на мелких сообщениях.
   - **Входящий и исходящий трафик** считаются отдельно. В облаке за исходящий обычно платят.
3. **«Сколько места на самом деле занимает строка»** — после расчёта хранилища.
   - **В PostgreSQL** у каждой строки есть заголовок 23 байта (с выравниванием обычно 24) и указатель в странице 4 байта. На маленьком сообщении это заметная доля.
   - **Сверху:** индексы (каждый — отдельная структура), WAL, свободное место в страницах и «раздувание» после обновлений (bloat), реплики.
   - **Практическое правило оценки:** умножить «чистый» объём на 1,5–2 — помечено как эвристика. Ссылка на модуль 2b.

- [ ] **Step 3: Урок 0.6 — метки, числа**

Числа для `<N>`:
- 100 млн DAU;
- ≈ 46 тыс. / 139 тыс. RPS;
- 4,4 ПБ;
- 30 млн соединений;
- ≈ 300 серверов шлюза.

Раздел мини-интервью — `selfcheck`.

- [ ] **Step 4: Урок 0.6 — два блока «Под капотом»**

1. **«Сколько стоит одно WebSocket-соединение»** — после оценки числа серверов шлюза.
   - **Ядро ОС:** буферы приёма и отправки сокета. В Linux размеры настраиваются через `net.ipv4.tcp_rmem` / `tcp_wmem` и растут по мере нужды, у простаивающего соединения они небольшие.
   - **Go:** горутина стартует со стеком 2 КБ и растёт по необходимости. На соединение обычно одна-две горутины (чтение и запись) плюс буферы `bufio` по 4 КБ, если используются.
   - **Оценка:** порядка 10–50 КБ на простаивающее соединение (явно помечено как оценка, измерять — в модуле 1.5). 100 тыс. соединений ≈ 1–5 ГБ памяти.
   - **Что ещё ограничивает:**
     - лимит файловых дескрипторов (`ulimit -n`, `fs.file-max`) — каждое соединение это дескриптор;
     - CPU на heartbeat-сообщения;
     - на прокси и балансировщике — диапазон эфемерных портов к одному адресу назначения (по умолчанию в Linux 32768–60999, около 28 тыс.).
   - Ссылки: модуль 1.5 (netpoller и epoll), модуль 3.
2. **«Fan-out на числах»** — после раздела про чтение и группы.
   - **Fan-out при записи:** сообщение в группу из N участников сразу записывается в N «входящих». Читать дёшево, писать дорого.
   - **Fan-out при чтении:** сообщение пишется один раз, а каждый получатель сам забирает новое из чатов. Писать дёшево, читать дорого.
   - **Канал на 1 млн подписчиков:** при записи — миллион операций на один пост, поэтому для больших каналов выбирают чтение. Гибрид — как в примере Twitter из Клеппмана, гл. 1: обычным аккаунтам fan-out при записи, знаменитостям — при чтении.
   - **Числа:** группа из 100 участников при 139 тыс. сообщений/с в пик даёт до 13,9 млн записей во «входящие» в секунду, если бы все сообщения шли в такие группы. Подать как верхнюю оценку и пояснить, почему реальная нагрузка меньше.
   - Ссылка на модуль 7.

- [ ] **Step 5: Самодостаточность 0.5 и 0.6 (правило 4)**

- [ ] **Step 6: Проверить и закоммитить**

Run: `npm test && npm run check:go && npm run build` + Playwright (правило 7).

```bash
git add course/modules/00-foundation/05-estimation.md course/modules/00-foundation/06-messenger-requirements.md course/glossary/terms.yaml
git commit -m "feat(course): уроки 0.5–0.6 — метки, ключевые числа, «Под капотом», источники

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Итоговая проверка, README, бэклог

**Files:**
- Modify: `README.md`, `docs/superpowers/backlog-after-module-0.md`

**Interfaces:**
- Consumes: всё из задач 1–8.

- [ ] **Step 1: Полный прогон**

Run: `cd /home/idel/DevGo/system-desine/course && npm test && npm run typecheck && npm run check:go && npm run build && ls -la .vitepress/dist/assets/app.*.js`

Expected:
- всё зелёное;
- `app.*.js` без Mermaid (`grep -l mermaid` по нему ничего не находит) и заметно меньше прежних ~700 КБ. Размер — в отчёт.

- [ ] **Step 2: Внешний вид и контраст через Playwright**

`npm run preview` в фоне. Для `/`, `/glossary` и шести уроков, на ширине 375 и 1280, в тёмной (по умолчанию) и светлой теме:
1. `scrollWidth <= innerWidth`;
2. скриншот — смотреть его (Read), а не только сохранить. Метки частей урока, `<N>`, `<Deep>` и схемы видны и читаемы;
3. контраст выборочно через `browser_evaluate`: вычислить отношение контраста `getComputedStyle(el).color` к цвету фона для основного текста, `.part`, `.n` и `.deep summary` по формуле WCAG (относительная яркость). Ожидается ≥ 4,5. Значения — в отчёт.

Проблемы исправить в соответствующих файлах и повторить.

- [ ] **Step 3: Полноэкранный режим и якоря**

1. Для каждой визуализации (0.3, 0.4, 0.5) и карты курса (0.1) проверить:
   - вход ⛶;
   - клавиши;
   - Esc;
   - оверлей при отказе API (эмуляция из задачи 4);
   - отсутствие `viz-scroll-lock` после ухода со страницы в оверлее.
2. Переключить тему при открытой схеме в полноэкранном режиме — схема перерисована, `id` SVG уникальны.
3. Якоря: `grep -o 'id="[^"]*"' .vitepress/dist/modules/00-foundation/03-request-path.html` — ни один `id` не содержит слов меток («теория», «проблема», «визуализация»).

- [ ] **Step 4: README**

В «Как добавить урок» дописать:
- метки `<Part kind>` у каждого `##` (список kind) и что метка не меняет якорь;
- `<N>` для 3–8 ключевых чисел;
- `<Deep title>` для подробностей, 150–500 слов;
- `simplified` в `Callout` для упрощённого кода;
- Go-фрагменты помечаются `<!-- go-check -->` или `<!-- go-check: harness=имя -->`, обвязки лежат в `course/scripts/go-harness/`, проверка — `npm run check:go`;
- визуализации и схемы оборачиваются в `VizFrame` (Mermaid — автоматически);
- в «Проверках» добавить `npm run check:go`.

- [ ] **Step 5: Бэклог**

В `docs/superpowers/backlog-after-module-0.md`:
- удалить выполненные пункты: запросы на полировку, подписи с терминами, легенда, NBSP, `seek(NaN)`, YAML-ошибка, `role=tooltip`, Esc при уходе мыши, Mermaid-чанк, мелкая карта курса, тяжёлый T;
- оставить невыполненные;
- добавить строку «Выполнено в подпроекте «Полировка модуля 0» (2026-10-12)» со списком закрытого.

- [ ] **Step 6: Commit**

```bash
cd /home/idel/DevGo/system-desine
git add README.md docs/superpowers/backlog-after-module-0.md course
git commit -m "docs: README и бэклог после полировки модуля 0; финальные правки

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
