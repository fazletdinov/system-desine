import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  // Плагин Vue — чтобы тесты рендерили настоящие компоненты .vue (Shot), а не только чистые модули.
  plugins: [vue()],
  test: {
    include: ['.vitepress/**/*.test.ts', 'glossary/**/*.test.ts', 'interview/**/*.test.ts', 'viz/**/*.test.ts', 'components/**/*.test.ts', 'scripts/**/*.test.ts'],
    environment: 'node',
  },
})
