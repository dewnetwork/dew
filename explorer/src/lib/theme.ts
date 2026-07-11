export type ThemePreference = "light" | "dark" | "system";
export type ResolvedTheme = "light" | "dark";

export function getSystemTheme(): ResolvedTheme {
  if (typeof window === "undefined") return "light";
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function resolveTheme(pref: ThemePreference): ResolvedTheme {
  return pref === "system" ? getSystemTheme() : pref;
}

export function applyTheme(resolved: ResolvedTheme): void {
  const root = document.documentElement;
  root.dataset.theme = resolved;
  root.style.colorScheme = resolved;
}

/** Cycle light → dark → system → light */
export function nextTheme(pref: ThemePreference): ThemePreference {
  if (pref === "light") return "dark";
  if (pref === "dark") return "system";
  return "light";
}

export function themeLabel(pref: ThemePreference, resolved: ResolvedTheme): string {
  if (pref === "system") return `System (${resolved})`;
  return pref === "dark" ? "Dark" : "Light";
}
