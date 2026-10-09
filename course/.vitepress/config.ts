import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitepress'
import { buildSidebar } from './sidebar'

const MODULES_DIR = fileURLToPath(new URL('../modules', import.meta.url))

export default defineConfig({
  lang: 'ru-RU',
  title: 'System Design на практике',
  description: 'Курс по проектированию высоконагруженных систем на Go: пошаговые визуализации и сквозной проект-мессенджер',
  cleanUrls: true,
  srcExclude: ['**/README.md'],
  themeConfig: {
    nav: [{ text: 'Курс', link: '/modules/00-foundation/' }],
    sidebar: buildSidebar(MODULES_DIR),
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
