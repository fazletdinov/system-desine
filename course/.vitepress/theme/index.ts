import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import T from '../../components/T.vue'
import GlossaryList from '../../components/GlossaryList.vue'
import Callout from '../../components/Callout.vue'
import SelfCheck from '../../components/SelfCheck.vue'
import '../../viz/palette.css'
import './custom.css'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('T', T)
    app.component('GlossaryList', GlossaryList)
    app.component('Callout', Callout)
    app.component('SelfCheck', SelfCheck)
  },
} satisfies Theme
