import DefaultTheme from 'vitepress/theme-without-fonts'
import type { Theme } from 'vitepress'
import T from '../../components/T.vue'
import Callout from '../../components/Callout.vue'
import SelfCheck from '../../components/SelfCheck.vue'
import '@fontsource-variable/inter'
import '@fontsource-variable/jetbrains-mono'
import Part from '../../components/Part.vue'
import N from '../../components/N.vue'
import Deep from '../../components/Deep.vue'
import MermaidDiagram from '../../components/MermaidDiagram.vue'
import '../../components/parts.css'
import '../../viz/palette.css'
import './custom.css'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('T', T)
    app.component('Callout', Callout)
    app.component('SelfCheck', SelfCheck)
    app.component('Part', Part)
    app.component('N', N)
    app.component('Deep', Deep)
    app.component('MermaidDiagram', MermaidDiagram)
  },
} satisfies Theme
