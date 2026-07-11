#!/usr/bin/env node
/**
 * Build landing (web/) + docs (VitePress) and merge into a single static site:
 *
 *   dist/          ← Astro marketing site (web/dist)
 *   dist/docs/     ← VitePress docs (docs/.vitepress/dist) with base <SITE_BASE>docs/
 *
 * Env:
 *   SITE_BASE  site root path — `/` (default) or `/dew/` for GitHub project pages
 *   SITE_URL   origin for canonical URLs — e.g. https://dewnetwork.github.io
 *
 * Usage: node scripts/build-site.mjs
 *        pnpm build
 *        SITE_BASE=/dew/ SITE_URL=https://dewnetwork.github.io pnpm build
 */

import { cpSync, existsSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { execSync } from 'node:child_process'

const __dirname = dirname(fileURLToPath(import.meta.url))
const root = resolve(__dirname, '..')
const outDir = resolve(root, 'dist')
const webDist = resolve(root, 'web/dist')
const docsDist = resolve(root, 'docs/.vitepress/dist')

/** Normalize to leading slash + trailing slash (`/`, `/dew/`). */
function normalizeSiteBase(raw) {
  let v = (raw ?? '/').trim() || '/'
  if (!v.startsWith('/')) v = `/${v}`
  if (!v.endsWith('/')) v = `${v}/`
  return v
}

/** Astro wants no trailing slash except for root. */
function astroBaseFrom(siteBase) {
  if (siteBase === '/') return '/'
  return siteBase.replace(/\/+$/, '')
}

const siteBase = normalizeSiteBase(process.env.SITE_BASE)
const docsBase = `${siteBase}docs/`
const astroBase = astroBaseFrom(siteBase)
const siteUrl = process.env.SITE_URL?.trim() || ''

function log(msg) {
  console.log(msg)
}

function run(cmd, env = {}) {
  log(`\n→ ${cmd}`)
  execSync(cmd, {
    cwd: root,
    stdio: 'inherit',
    env: { ...process.env, ...env },
  })
}

function assertDir(path, label) {
  if (!existsSync(path)) {
    throw new Error(`${label} missing after build: ${path}`)
  }
}

log('Dew site build — landing + docs → dist/')
log(`  SITE_BASE=${siteBase}  (Astro base=${astroBase})`)
log(`  DOCS_BASE=${docsBase}`)
if (siteUrl) log(`  SITE_URL=${siteUrl}`)

if (existsSync(outDir)) {
  rmSync(outDir, { recursive: true, force: true })
}
mkdirSync(outDir, { recursive: true })

// 1) Landing site at SITE_BASE
run('pnpm web:build', {
  SITE_BASE: astroBase,
  ...(siteUrl ? { SITE_URL: siteUrl } : {}),
})
assertDir(webDist, 'web/dist')

// 2) Docs under SITE_BASE + docs/
run('pnpm docs:build', { DOCS_BASE: docsBase })
assertDir(docsDist, 'docs/.vitepress/dist')

// 3) Merge
log('\n→ merge web/dist → dist/')
cpSync(webDist, outDir, { recursive: true })

log('→ merge docs/.vitepress/dist → dist/docs/')
cpSync(docsDist, resolve(outDir, 'docs'), { recursive: true })

// GitHub Pages (and some static hosts) treat `_` paths as private without this.
writeFileSync(resolve(outDir, '.nojekyll'), '')

log(`
✓ Site ready: ${outDir}
  ${siteBase}       landing (Astro)
  ${docsBase}  documentation (VitePress)

Preview:  pnpm site:preview
`)
