import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    include: ['.vitepress/**/*.test.ts', 'glossary/**/*.test.ts', 'viz/**/*.test.ts', 'components/**/*.test.ts', 'scripts/**/*.test.ts'],
    environment: 'node',
  },
})
