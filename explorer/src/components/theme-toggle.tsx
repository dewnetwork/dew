import { useEffect, useState } from "react";
import { resolveTheme, themeLabel } from "@/lib/theme";
import { useUiStore } from "@/stores/ui";

export function ThemeToggle() {
  const theme = useUiStore((s) => s.theme);
  const cycleTheme = useUiStore((s) => s.cycleTheme);
  const [resolved, setResolved] = useState(() => resolveTheme(theme));

  useEffect(() => {
    setResolved(resolveTheme(theme));
  }, [theme]);

  useEffect(() => {
    if (theme !== "system") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => setResolved(resolveTheme("system"));
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, [theme]);

  const icon = resolved === "dark" ? "☾" : "☀";
  const label = themeLabel(theme, resolved);

  return (
    <button
      type="button"
      className="theme-toggle"
      onClick={cycleTheme}
      title={`Theme: ${label}. Click to cycle Light → Dark → System.`}
      aria-label={`Theme ${label}. Click to change.`}
    >
      <span aria-hidden className="text-sm leading-none">
        {icon}
      </span>
      <span className="hidden sm:inline capitalize">
        {theme === "system" ? "Auto" : theme}
      </span>
    </button>
  );
}
