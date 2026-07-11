import { create } from "zustand";
import { persist } from "zustand/middleware";
import {
  applyTheme,
  nextTheme,
  resolveTheme,
  type ThemePreference,
} from "@/lib/theme";

type Toast = { id: number; message: string };

type UiState = {
  theme: ThemePreference;
  recentSearches: string[];
  toasts: Toast[];
  setTheme: (theme: ThemePreference) => void;
  cycleTheme: () => void;
  pushRecent: (q: string) => void;
  toast: (message: string) => void;
  dismissToast: (id: number) => void;
};

let toastSeq = 0;

function syncDomTheme(pref: ThemePreference) {
  applyTheme(resolveTheme(pref));
}

export const useUiStore = create<UiState>()(
  persist(
    (set, get) => ({
      theme: "system",
      recentSearches: [],
      toasts: [],
      setTheme: (theme) => {
        syncDomTheme(theme);
        set({ theme });
      },
      cycleTheme: () => {
        const theme = nextTheme(get().theme);
        syncDomTheme(theme);
        set({ theme });
      },
      pushRecent: (q) => {
        const t = q.trim();
        if (!t) return;
        const prev = get().recentSearches.filter((x) => x !== t);
        set({ recentSearches: [t, ...prev].slice(0, 8) });
      },
      toast: (message) => {
        const id = ++toastSeq;
        set((s) => ({ toasts: [...s.toasts, { id, message }] }));
        window.setTimeout(() => get().dismissToast(id), 2200);
      },
      dismissToast: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
    }),
    {
      name: "dew-explorer-ui",
      partialize: (s) => ({ recentSearches: s.recentSearches, theme: s.theme }),
      onRehydrateStorage: () => (state) => {
        if (state?.theme) syncDomTheme(state.theme);
      },
    },
  ),
);

/** Call once after store is available; also listen for OS theme changes when preference is system. */
export function initThemeListeners(): () => void {
  const apply = () => {
    const pref = useUiStore.getState().theme;
    if (pref === "system") syncDomTheme("system");
  };
  const mq = window.matchMedia("(prefers-color-scheme: dark)");
  mq.addEventListener("change", apply);
  // Apply current (may still be default before rehydrate)
  apply();
  return () => mq.removeEventListener("change", apply);
}
