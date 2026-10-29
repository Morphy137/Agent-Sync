export type ThemeMode = "system" | "light" | "dark";

let currentMode: ThemeMode = "system";

export function getSystemTheme(): "light" | "dark" {
  if (typeof window !== "undefined" && window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) {
    return "dark";
  }
  return "light";
}

export function applyTheme(mode: ThemeMode) {
  currentMode = mode;
  const resolved = mode === "system" ? getSystemTheme() : mode;
  document.documentElement.setAttribute("data-theme", resolved);
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
