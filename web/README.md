# Dew landing (`web/`)

Marketing site for **Dew** — Astro + React islands, Tailwind CSS v4, Motion.

## Stack

| Piece | Role |
| :--- | :--- |
| **Astro** | Static shell, routing, islands |
| **React** | Interactive pieces (`commit-pulse`, scroll reveals) |
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
  components/          # kebab-case Astro sections + react/ islands
  layouts/base-layout.astro
  lib/
    paths.ts           # withBase() for SITE_BASE
    network.ts         # public-testnet-v1 live URLs (path B)
  pages/index.astro
  styles/global.css    # @theme tokens
public/                # favicon + logo + og
```

Live path B URLs (RPC, faucet, explorer, Guestbook) are centralized in
[`src/lib/network.ts`](./src/lib/network.ts). Keep them in sync with
[`docs/ops/public-testnet.md`](../docs/ops/public-testnet.md) and
[`docs/ops/try-public.md`](../docs/ops/try-public.md).

**Add to MetaMask:** React island `add-network-button` (Network + CTA) always
visible. Uses [`src/lib/wallet.ts`](./src/lib/wallet.ts) —
`wallet_switchEthereumChain` then `wallet_addEthereumChain` (EIP-3085) on code
`4902`; missing wallet / rejection show as error text. MetaMask fox:
`metamask-icon.tsx`.

Docs remain on VitePress under `docs/` (`pnpm docs:dev`). Production merges both via monorepo `pnpm site:build` (see root README — GitHub Pages at `/dew/` + `/dew/docs/`).

When `SITE_BASE` is set (e.g. `/dew`), use `withBase()` from `src/lib/paths.ts` for site-absolute links and static assets so project pages resolve correctly.

## Sections (landing)

| Section | Purpose |
| :--- | :--- |
| Hero | Live badge, try-public CTA, protocol stats |
| Network | Faucet · explorer · Guestbook · RPC cards · Add to MetaMask |
| Why / Protocol / Performance | Vision + pillars + directional metrics |
| Builders | Steps + dual-tab terminal (Public / Local Foundry) |
| Phases | A–D status aligned with `docs/build/phases.md` |
| CTA | Try public + Add to MetaMask + docs + GitHub |

## Polish

| Piece | Detail |
| :--- | :--- |
| Mobile nav | `mobile-nav` sheet (`lg:hidden`) — links, Try public, GitHub |
| OG image | `public/og.svg` (source) · `public/og.png` (1200×630 for crawlers) · wired in `base-layout.astro` |
| Builder terminal | `builder-terminal` — Public vs Local tabs |

Regenerate PNG after editing the SVG:

```bash
rsvg-convert -w 1200 -h 630 public/og.svg -o public/og.png
```

## Block explorer (separate app)

The explorer is **not** part of this package. It lives in monorepo root **`explorer/`** (React + Vite + Tailwind + Radix + nuqs + TanStack Query + TanStack Router + Zustand).

Docs: [`docs/product/block-explorer.md`](../docs/product/block-explorer.md). Live path B: `https://explorer-dew.fadosoft.com`.
