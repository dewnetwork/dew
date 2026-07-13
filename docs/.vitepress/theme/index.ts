import { h, nextTick, watch } from 'vue'
import type { Theme } from 'vitepress'
import DefaultTheme from 'vitepress/theme'
import { useData } from 'vitepress'
import { createMermaidRenderer } from 'vitepress-mermaid-renderer'
import './custom.css'

/** Brand-aligned Mermaid palette (shared light/dark base colors). */
const mermaidThemeVariables = {
  primaryColor: '#3db8a8',
  primaryTextColor: '#0f1c2e',
  primaryBorderColor: '#1a8f82',
  lineColor: '#3d4f63',
  secondaryColor: '#e8f0ed',
  tertiaryColor: '#f7faf9',
  fontFamily: 'Figtree, system-ui, sans-serif',
}

const theme: Theme = {
  extends: DefaultTheme,
  Layout: () => {
    const { isDark } = useData()

    const initMermaid = () => {
      createMermaidRenderer({
        theme: isDark.value ? 'dark' : 'base',
        themeVariables: mermaidThemeVariables,
        flowchart: {
          curve: 'basis',
          htmlLabels: true,
          padding: 12,
        },
        sequence: {
          actorMargin: 40,
          messageMargin: 30,
        },
      })
    }

    nextTick(() => initMermaid())
    watch(
      () => isDark.value,
      () => initMermaid(),
    )

    return h(DefaultTheme.Layout)
  },
}

export default theme
