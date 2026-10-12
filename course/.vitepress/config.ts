import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitepress'
import { loadGlossaryFile } from '../glossary/load'
import { buildSidebar } from './sidebar'
import { mermaidFencePlugin } from './mermaid-fence'
import { termsCheckPlugin } from './terms-check'

const MODULES_DIR = fileURLToPath(new URL('../modules', import.meta.url))
const GLOSSARY_FILE = fileURLToPath(new URL('../glossary/terms.yaml', import.meta.url))
const KNOWN_TERMS: ReadonlySet<string> = new Set(Object.keys(loadGlossaryFile(GLOSSARY_FILE)))

export default defineConfig({
  lang: 'ru-RU',
  appearance: 'dark',
  title: 'System Design на практике',
  description: 'Курс по проектированию высоконагруженных систем на Go: пошаговые визуализации и сквозной проект-мессенджер',
  cleanUrls: true,
  head: [['link', { rel: 'icon', href: '/favicon.svg', type: 'image/svg+xml' }]],
  srcExclude: ['**/README.md'],
  // mermaid тянет CommonJS-зависимости (fastdom и др.); без пребандлинга dev-сервер падает на импорте.
  vite: {
    optimizeDeps: { include: ['mermaid'] },
    // Mermaid грузится лениво отдельными чанками (самые большие ≈ 700 КБ), поэтому порог поднят с 500 до 1000 КБ.
    // Размер основного чанка app.*.js проверяется в задаче 9.
    build: { chunkSizeWarningLimit: 1000 },
  },
  markdown: {
    config: (md) => {
      termsCheckPlugin(md, KNOWN_TERMS)
      mermaidFencePlugin(md)
    },
  },
  themeConfig: {
    nav: [
      { text: 'Курс', link: '/modules/00-foundation/' },
      { text: 'Глоссарий', link: '/glossary' },
    ],
    sidebar: [
      ...buildSidebar(MODULES_DIR),
      { text: 'Справочник', items: [{ text: 'Глоссарий', link: '/glossary' }] },
    ],
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
