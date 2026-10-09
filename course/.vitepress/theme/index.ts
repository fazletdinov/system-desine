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
