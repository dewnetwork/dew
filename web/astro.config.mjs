// @ts-check
import { defineConfig } from 'astro/config';

import react from '@astrojs/react';
import tailwindcss from '@tailwindcss/vite';

/**
 * SITE_BASE — project root path for assets/links.
 *   local / custom domain: `/` (default)
 *   GitHub project pages:  `/dew`  (no trailing slash; Astro convention)
 *
 * SITE_URL — origin for canonical / og URLs (optional).
 *   e.g. https://dewnetwork.github.io
 *
 * @param {string | undefined} raw
 */
function normalizeAstroBase(raw) {
  const v = (raw ?? '/').trim() || '/';
  if (v === '/') return '/';
  const withSlash = v.startsWith('/') ? v : `/${v}`;
  return withSlash.replace(/\/+$/, '');
}

const siteBase = normalizeAstroBase(process.env.SITE_BASE);
const siteUrl = process.env.SITE_URL?.trim() || undefined;

// https://astro.build/config
export default defineConfig({
  site: siteUrl,
  base: siteBase,
  integrations: [react()],
  vite: {
    plugins: [tailwindcss()],
  },
  server: {
    port: 4321,
  },
});
