#!/usr/bin/env node
/**
 * Build landing (web/) + docs (VitePress) and merge into a single static site:
 *
 *   dist/          ← Astro marketing site (web/dist)
 *   dist/docs/     ← VitePress docs (docs/.vitepress/dist) with base /docs/
 *
 * Usage: node scripts/build-site.mjs
 *        pnpm build
 */

import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { execSync } from 'node:child_process'

const __dirname = dirname(fileURLToPath(import.meta.url))
const root = resolve(__dirname, '..')
const outDir = resolve(root, 'dist')
const webDist = resolve(root, 'web/dist')
const docsDist = resolve(root, 'docs/.vitepress/dist')

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

if (existsSync(outDir)) {
  rmSync(outDir, { recursive: true, force: true })
}
mkdirSync(outDir, { recursive: true })

// 1) Landing site at /
run('pnpm web:build')
assertDir(webDist, 'web/dist')

// 2) Docs under /docs/ (base path baked into asset URLs)
run('pnpm docs:build', { DOCS_BASE: '/docs/' })
assertDir(docsDist, 'docs/.vitepress/dist')

// 3) Merge
log('\n→ merge web/dist → dist/')
cpSync(webDist, outDir, { recursive: true })

log('→ merge docs/.vitepress/dist → dist/docs/')
cpSync(docsDist, resolve(outDir, 'docs'), { recursive: true })

log(`
✓ Site ready: ${outDir}
  /       landing (Astro)
  /docs/  documentation (VitePress)

Preview:  pnpm site:preview
`)
