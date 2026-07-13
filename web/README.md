# Dew landing (`web/`)

Marketing site for **Dew** — Astro + React islands, Tailwind CSS v4, Motion.

## Stack

| Piece | Role |
| :--- | :--- |
| **Astro** | Static shell, routing, islands |
| **React** | Interactive pieces (`CommitPulse`, scroll reveals) |
| **Tailwind v4** | Utility styling via `@tailwindcss/vite` |
| **Motion** | Entrance / scroll / commit-cycle animation |

Brand tokens mirror the docs theme: maritime ink + dew cyan, Syne / Figtree / IBM Plex Mono.

## Commands

From monorepo root:

```bash
pnpm web:dev       # http://localhost:4321
pnpm web:build     # → web/dist
pnpm web:preview
```

From this directory:

```bash
pnpm dev
pnpm build
pnpm preview
```

## Structure

```
src/
  components/          # Astro sections + react/ islands
  layouts/BaseLayout.astro
  pages/index.astro
  styles/global.css    # @theme tokens
public/                # favicon + logo
```

Docs remain on VitePress under `docs/` (`pnpm docs:dev`). Production merges both via monorepo `pnpm site:build` (see root README — GitHub Pages at `/dew/` + `/dew/docs/`).

When `SITE_BASE` is set (e.g. `/dew`), use `withBase()` from `src/lib/paths.ts` for site-absolute links and static assets so project pages resolve correctly.

## Block explorer (separate app)

The explorer is **not** part of this package. It lives in monorepo root **`explorer/`** (React + Vite + Tailwind + Radix + nuqs + TanStack Query + TanStack Router + Zustand).

Docs: [`docs/product/block-explorer.md`](../docs/product/block-explorer.md). Live path B: `Explorer: https://explorer-dew.fadosoft.com`.
