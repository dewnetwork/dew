import { defineConfig, type DefaultTheme } from 'vitepress'
import { readFileSync, existsSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { parse as parseYaml } from 'yaml'

const __dirname = dirname(fileURLToPath(import.meta.url))
const docsRoot = resolve(__dirname, '..')

type SidebarYamlItem = string
type SidebarYamlCategory = {
  category: string
  path: string
  items: SidebarYamlItem[]
}

function frontmatterTitle(filePath: string): string | null {
  if (!existsSync(filePath)) return null
  const text = readFileSync(filePath, 'utf8')
  if (!text.startsWith('---')) return null
  const end = text.indexOf('\n---', 3)
  if (end === -1) return null
  const fm = text.slice(3, end).trim()
  const match = /^title:\s*(.+)$/m.exec(fm)
  if (!match) return null
  return match[1].trim().replace(/^["']|["']$/g, '')
}

function loadSidebar(): DefaultTheme.SidebarItem[] {
  const raw = readFileSync(resolve(docsRoot, 'sidebar.yaml'), 'utf8')
  const categories = parseYaml(raw) as SidebarYamlCategory[]

  return categories.map((cat) => ({
    text: cat.category,
    collapsed: false,
    items: cat.items.map((item) => {
      const slug = item.replace(/\.md$/, '')
      const abs = resolve(docsRoot, cat.path, item)
      const title = frontmatterTitle(abs) ?? titleFromSlug(slug)
      return {
        text: title,
        link: `/${cat.path}/${slug}`,
      }
    }),
  }))
}

/** Fallback when a page has no frontmatter title. */
function titleFromSlug(slug: string): string {
  return slug
    .split('-')
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ')
}

const sidebar = loadSidebar()

export default defineConfig({
  title: 'Dewchain',
  description:
    'High-performance, EVM-compatible Layer 1 — protocol docs for the Go + Node monorepo.',
  lang: 'en-US',
  cleanUrls: true,
  lastUpdated: true,
  ignoreDeadLinks: true,

  head: [
    ['link', { rel: 'icon', href: '/favicon.svg', type: 'image/svg+xml' }],
    [
      'link',
      {
        rel: 'stylesheet',
        href: 'https://fonts.googleapis.com/css2?family=Figtree:ital,wght@0,400;0,500;0,600;0,700;1,400&family=IBM+Plex+Mono:wght@400;500&family=Syne:wght@600;700;800&display=swap',
      },
    ],
    ['meta', { name: 'theme-color', content: '#0a1628' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:title', content: 'Dewchain Docs' }],
    [
      'meta',
      {
        property: 'og:description',
        content:
          'Protocol, architecture, and development documentation for Dewchain L1.',
      },
    ],
  ],

  themeConfig: {
    logo: { src: '/logo.svg', alt: 'Dewchain' },
    siteTitle: 'Dewchain',

    nav: [
      { text: 'Vision', link: '/overview/vision' },
      { text: 'Architecture', link: '/architecture/overview' },
      { text: 'Roadmap', link: '/development/roadmap' },
      { text: 'JSON-RPC', link: '/api/json-rpc' },
    ],

    sidebar: {
      '/': sidebar,
    },

    socialLinks: [],

    search: {
      provider: 'local',
    },

    outline: {
      level: [2, 3],
      label: 'On this page',
    },

    docFooter: {
      prev: 'Previous',
      next: 'Next',
    },

    lastUpdated: {
      text: 'Updated',
      formatOptions: {
        dateStyle: 'medium',
      },
    },

    footer: {
      message: 'Dewchain monorepo — Go L1 core · Node docs site',
      copyright: 'Documentation status: draft until public testnet freeze',
    },

    returnToTopLabel: 'Back to top',
    sidebarMenuLabel: 'Menu',
    darkModeSwitchLabel: 'Theme',
  },

  markdown: {
    theme: {
      light: 'github-light',
      dark: 'github-dark',
    },
    lineNumbers: false,
  },

  vite: {
    server: {
      port: 5173,
      strictPort: false,
    },
  },
})
