export type ThemeMode = "system" | "light" | "dark";

const THEME_STORAGE_KEY = "agentsync_theme";
const THEME_MODE_STORAGE_KEY = "agentsync_theme_mode";

let currentMode: ThemeMode = "system";

export function getSystemTheme(): "light" | "dark" {
  if (
    typeof window !== "undefined" &&
    window.matchMedia &&
    window.matchMedia("(prefers-color-scheme: dark)").matches
  ) {
    return "dark";
  }
  return "light";
}

export function applyTheme(mode: ThemeMode) {
  currentMode = mode;
  const resolved = mode === "system" ? getSystemTheme() : mode;
  if (typeof document !== "undefined") {
    document.documentElement.setAttribute("data-theme", resolved);
    const meta = document.querySelector('meta[name="color-scheme"]');
    if (meta) {
      meta.setAttribute("content", resolved);
    }
  }
  try {
    if (typeof localStorage !== "undefined") {
      localStorage.setItem(THEME_STORAGE_KEY, resolved);
      localStorage.setItem(THEME_MODE_STORAGE_KEY, mode);
    }
  } catch {
    // ignore storage errors
  }
}

export function initTheme(initialMode: ThemeMode = "system") {
  applyTheme(initialMode);

  if (typeof window !== "undefined" && window.matchMedia) {
    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    mediaQuery.addEventListener("change", () => {
      if (currentMode === "system") {
        applyTheme("system");
      }
    });
  }
}
