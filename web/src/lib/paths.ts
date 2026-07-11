/**
 * Prefix a site-absolute path with Astro `base` (e.g. `/` or `/dew/`).
 * Hash links and absolute URLs are returned unchanged.
 *
 * Astro may expose BASE_URL as `/dew` or `/dew/` depending on config;
 * always normalize so we never produce `/dewfavicon.svg`.
 */
export function withBase(path: string): string {
  if (
    path.startsWith('#') ||
    path.startsWith('http://') ||
    path.startsWith('https://') ||
    path.startsWith('//')
  ) {
    return path
  }

  let base = import.meta.env.BASE_URL || '/'
  if (!base.endsWith('/')) base = `${base}/`

  const cleaned = path.replace(/^\//, '')
  return `${base}${cleaned}`
}
