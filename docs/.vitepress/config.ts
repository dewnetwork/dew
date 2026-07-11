import { withMermaid } from 'vitepress-plugin-mermaid'
import type { DefaultTheme } from 'vitepress'
import { readFileSync, existsSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { parse as parseYaml } from 'yaml'
// @ts-expect-error no bundled types
import taskLists from 'markdown-it-task-lists'

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

// When merging with the landing site, set DOCS_BASE=/docs/ (or /dew/docs/ on
// GitHub project pages) so assets/links resolve correctly. Standalone
// `docs:dev` / `docs:build` keep base at /.
const docsBaseRaw = process.env.DOCS_BASE?.trim() || '/'
const docsBase = docsBaseRaw.endsWith('/') ? docsBaseRaw : `${docsBaseRaw}/`

export default withMermaid({
  title: 'Dew',
  description:
    'High-performance, EVM-compatible Layer 1 — protocol docs for the Go + Node monorepo.',
  lang: 'en-US',
  base: docsBase,
  cleanUrls: true,
  lastUpdated: true,
  ignoreDeadLinks: true,

  head: [
    // Head hrefs are not auto-prefixed by VitePress — include base explicitly.
    [
      'link',
      { rel: 'icon', href: `${docsBase}favicon.svg`, type: 'image/svg+xml' },
    ],
    [
      'link',
      {
        rel: 'stylesheet',
        href: 'https://fonts.googleapis.com/css2?family=Figtree:ital,wght@0,400;0,500;0,600;0,700;1,400&family=IBM+Plex+Mono:wght@400;500&family=Syne:wght@600;700;800&display=swap',
      },
    ],
    ['meta', { name: 'theme-color', content: '#0a1628' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:title', content: 'Dew Docs' }],
    [
      'meta',
      {
        property: 'og:description',
        content:
          'Protocol, architecture, and development documentation for Dew L1.',
      },
    ],
  ],

  themeConfig: {
    // Logo path is relative to public/; VitePress applies `base` automatically.
    logo: { src: '/logo.svg', alt: 'Dew' },
    siteTitle: 'Dew',

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
      message: 'Dew monorepo — Go L1 core · Node docs site',
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
    math: true,
    // Render `- [ ]` / `- [x]` as disabled checkboxes (docs are source of truth).
    config(md) {
      md.use(taskLists, {
        enabled: false,
        label: true,
      })
    },
  },

  mermaid: {
    theme: 'base',
    themeVariables: {
      primaryColor: '#3db8a8',
      primaryTextColor: '#0f1c2e',
      primaryBorderColor: '#1a8f82',
      lineColor: '#3d4f63',
      secondaryColor: '#e8f0ed',
      tertiaryColor: '#f7faf9',
      fontFamily: 'Figtree, system-ui, sans-serif',
    },
    flowchart: {
      curve: 'basis',
      htmlLabels: true,
      padding: 12,
    },
    sequence: {
      actorMargin: 40,
      messageMargin: 30,
    },
  },

  vite: {
    server: {
      port: 5173,
      strictPort: false,
    },
    // Mermaid imports dayjs as ESM default; CJS dayjs.min.js breaks in the browser.
    // Use exact /^dayjs$/ only — a string alias prefixes subpaths like dayjs/plugin/* incorrectly.
    resolve: {
      alias: [
        { find: /^dayjs$/, replacement: 'dayjs/esm/index.js' },
        {
          find: 'dayjs/plugin/duration.js',
          replacement: 'dayjs/esm/plugin/duration',
        },
        {
          find: 'dayjs/plugin/advancedFormat.js',
          replacement: 'dayjs/esm/plugin/advancedFormat',
        },
        {
          find: 'dayjs/plugin/customParseFormat.js',
          replacement: 'dayjs/esm/plugin/customParseFormat',
        },
        {
          find: 'dayjs/plugin/isoWeek.js',
          replacement: 'dayjs/esm/plugin/isoWeek',
        },
      ],
    },
    optimizeDeps: {
      include: [
        'dayjs',
        'dayjs/esm/index.js',
        'dayjs/esm/plugin/advancedFormat',
        'dayjs/esm/plugin/customParseFormat',
        'dayjs/esm/plugin/isoWeek',
        'dayjs/esm/plugin/duration',
        'mermaid',
      ],
    },
    ssr: {
      noExternal: ['mermaid', 'dayjs', 'vitepress-plugin-mermaid'],
    },
  },
})


