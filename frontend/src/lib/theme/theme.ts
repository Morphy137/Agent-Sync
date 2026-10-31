/**
 * Themes are complete color sets (background, surfaces, text and accent). Shadows and status
 * colors come from tokens.css and depend only on the theme's scheme (light or dark).
 */
export type Scheme = "light" | "dark";

export interface ThemeColors {
  canvas: string;
  surface: string;
  surfaceSunken: string;
  ink: string;
  inkHover: string;
  inkContrast: string;
  textMuted: string;
  textSubtle: string;
  accent: string;
  accentHover: string;
  accentContrast: string;
}

export interface ThemeDefinition {
  id: string;
  scheme: Scheme;
  colors: ThemeColors;
}

export const SYSTEM_THEME = "system";
export const DEFAULT_LIGHT_THEME = "light";
export const DEFAULT_DARK_THEME = "dark";

export const THEMES: ThemeDefinition[] = [
  {
    id: "light",
    scheme: "light",
    colors: {
      canvas: "#f7f7f8",
      surface: "#ffffff",
      surfaceSunken: "#f1f1f3",
      ink: "#0a0a0a",
      inkHover: "#1a1a1a",
      inkContrast: "#ffffff",
      textMuted: "#6b6b70",
      textSubtle: "#a0a0a6",
      accent: "#0a0a0a",
      accentHover: "#1a1a1a",
      accentContrast: "#ffffff",
    },
  },
  {
    id: "dark",
    scheme: "dark",
    colors: {
      canvas: "#08080a",
      surface: "#131316",
      surfaceSunken: "#0c0c0e",
      ink: "#f5f5f6",
      inkHover: "#ffffff",
      inkContrast: "#0a0a0a",
      textMuted: "#9a9aa2",
      textSubtle: "#5e5e66",
      accent: "#f5f5f6",
      accentHover: "#ffffff",
      accentContrast: "#0a0a0a",
    },
  },
  {
    id: "nord",
    scheme: "dark",
    colors: {
      canvas: "#242933",
      surface: "#2e3440",
      surfaceSunken: "#272c36",
      ink: "#eceff4",
      inkHover: "#ffffff",
      inkContrast: "#2e3440",
      textMuted: "#c0c8d6",
      textSubtle: "#7b88a1",
      accent: "#88c0d0",
      accentHover: "#a3d3e0",
      accentContrast: "#1c2028",
    },
  },
  {
    id: "rose",
    scheme: "dark",
    colors: {
      canvas: "#1f1a1e",
      surface: "#2b2229",
      surfaceSunken: "#231d22",
      ink: "#f7e9ef",
      inkHover: "#ffffff",
      inkContrast: "#1f1a1e",
      textMuted: "#cdb0bd",
      textSubtle: "#806672",
      accent: "#f472b6",
      accentHover: "#f9a8d4",
      accentContrast: "#2a0f1c",
    },
  },
  {
    id: "forest",
    scheme: "dark",
    colors: {
      canvas: "#161d1a",
      surface: "#1f2924",
      surfaceSunken: "#19221e",
      ink: "#e6f2ea",
      inkHover: "#ffffff",
      inkContrast: "#161d1a",
      textMuted: "#a7c2b1",
      textSubtle: "#62786b",
      accent: "#4ade80",
      accentHover: "#86efac",
      accentContrast: "#072012",
    },
  },
  {
    id: "ocean",
    scheme: "dark",
    colors: {
      canvas: "#141b26",
      surface: "#1c2534",
      surfaceSunken: "#161f2c",
      ink: "#e6eef9",
      inkHover: "#ffffff",
      inkContrast: "#141b26",
      textMuted: "#a3b4cc",
      textSubtle: "#5f7089",
      accent: "#60a5fa",
      accentHover: "#93c5fd",
      accentContrast: "#0a1424",
    },
  },
  {
    id: "violet",
    scheme: "dark",
    colors: {
      canvas: "#1a1724",
      surface: "#242032",
      surfaceSunken: "#1e1a2a",
      ink: "#efeaff",
      inkHover: "#ffffff",
      inkContrast: "#1a1724",
      textMuted: "#b8aed6",
      textSubtle: "#6f6690",
      accent: "#a78bfa",
      accentHover: "#c4b5fd",
      accentContrast: "#160f2b",
    },
  },
  {
    id: "amber",
    scheme: "dark",
    colors: {
      canvas: "#1e1a14",
      surface: "#2a241b",
      surfaceSunken: "#221d16",
      ink: "#f6eedf",
      inkHover: "#ffffff",
      inkContrast: "#1e1a14",
      textMuted: "#c9b79a",
      textSubtle: "#7d6f59",
      accent: "#fbbf24",
      accentHover: "#fcd34d",
      accentContrast: "#261a03",
    },
  },
  {
    id: "midnight",
    scheme: "dark",
    colors: {
      canvas: "#0b1020",
      surface: "#141b33",
      surfaceSunken: "#0f1529",
      ink: "#e8ecff",
      inkHover: "#ffffff",
      inkContrast: "#0b1020",
      textMuted: "#9aa3c7",
      textSubtle: "#56608a",
      accent: "#7c8cff",
      accentHover: "#a1adff",
      accentContrast: "#070b18",
    },
  },
  {
    id: "snow",
    scheme: "light",
    colors: {
      canvas: "#eceff4",
      surface: "#f8f9fb",
      surfaceSunken: "#e5e9f0",
      ink: "#2e3440",
      inkHover: "#3b4252",
      inkContrast: "#ffffff",
      textMuted: "#4c566a",
      textSubtle: "#8f9bb3",
      accent: "#5e81ac",
      accentHover: "#4c6d96",
      accentContrast: "#ffffff",
    },
  },
  {
    id: "sakura",
    scheme: "light",
    colors: {
      canvas: "#fbf1f4",
      surface: "#fff8fa",
      surfaceSunken: "#f5e6eb",
      ink: "#3a1f2a",
      inkHover: "#2a1520",
      inkContrast: "#ffffff",
      textMuted: "#8a5f70",
      textSubtle: "#c4a2b0",
      accent: "#db2777",
      accentHover: "#be185d",
      accentContrast: "#ffffff",
    },
  },
  {
    id: "sage",
    scheme: "light",
    colors: {
      canvas: "#eef4ef",
      surface: "#f8fbf8",
      surfaceSunken: "#e3ece5",
      ink: "#1d2e24",
      inkHover: "#142019",
      inkContrast: "#ffffff",
      textMuted: "#557060",
      textSubtle: "#9ab3a3",
      accent: "#15803d",
      accentHover: "#166534",
      accentContrast: "#ffffff",
    },
  },
  {
    id: "sepia",
    scheme: "light",
    colors: {
      canvas: "#f5efe4",
      surface: "#fbf7ef",
      surfaceSunken: "#ede5d6",
      ink: "#3b2f22",
      inkHover: "#2a2118",
      inkContrast: "#fffaf2",
      textMuted: "#7a6650",
      textSubtle: "#ae9b82",
      accent: "#9a5b2a",
      accentHover: "#7f4920",
      accentContrast: "#fffaf2",
    },
  },
];

/** Themes shown directly in Settings; the rest live behind "More". */
export const FEATURED_THEME_IDS = ["light", "dark", "nord", "rose", "forest"];

const CSS_VARIABLES: Record<keyof ThemeColors, string> = {
  canvas: "--canvas",
  surface: "--surface",
  surfaceSunken: "--surface-sunken",
  ink: "--ink",
  inkHover: "--ink-hover",
  inkContrast: "--ink-contrast",
  textMuted: "--text-muted",
  textSubtle: "--text-subtle",
  accent: "--accent",
  accentHover: "--accent-hover",
  accentContrast: "--accent-contrast",
};

const THEME_STORAGE_KEY = "agentsync_theme";

let currentTheme = SYSTEM_THEME;
let systemListenerInstalled = false;

/** Theme saved by the last run, used to paint before the config is loaded. */
export function getStoredTheme(): string {
  try {
    return localStorage?.getItem(THEME_STORAGE_KEY) || SYSTEM_THEME;
  } catch {
    return SYSTEM_THEME;
  }
}

export function findTheme(id: string): ThemeDefinition | undefined {
  return THEMES.find((theme) => theme.id === id);
}

export function getSystemScheme(): Scheme {
  if (
    typeof window !== "undefined" &&
    window.matchMedia?.("(prefers-color-scheme: dark)").matches
  ) {
    return "dark";
  }
  return "light";
}

/** Resolves "system" and unknown ids to a concrete theme. */
export function resolveTheme(id: string): ThemeDefinition {
  const explicit = id === SYSTEM_THEME ? undefined : findTheme(id);
  if (explicit) return explicit;
  const fallback = getSystemScheme() === "dark" ? DEFAULT_DARK_THEME : DEFAULT_LIGHT_THEME;
  return findTheme(fallback)!;
}

export function applyTheme(id: string) {
  currentTheme = id || SYSTEM_THEME;
  const theme = resolveTheme(currentTheme);
  if (typeof document !== "undefined") {
    const root = document.documentElement;
    root.setAttribute("data-theme", theme.id);
    root.setAttribute("data-scheme", theme.scheme);
    for (const [key, cssVar] of Object.entries(CSS_VARIABLES)) {
      root.style.setProperty(cssVar, theme.colors[key as keyof ThemeColors]);
    }
    document.querySelector('meta[name="color-scheme"]')?.setAttribute("content", theme.scheme);
  }
  try {
    localStorage?.setItem(THEME_STORAGE_KEY, currentTheme);
  } catch {
    // ignore storage errors
  }
}

export function initTheme(initial: string = SYSTEM_THEME) {
  applyTheme(initial);
  if (systemListenerInstalled || typeof window === "undefined" || !window.matchMedia) return;
  systemListenerInstalled = true;
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
    if (currentTheme === SYSTEM_THEME) applyTheme(SYSTEM_THEME);
  });
}
