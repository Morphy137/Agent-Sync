import en from "./locales/en.json";
import es from "./locales/es.json";
import { localeState, type SupportedLocale } from "./localeState.svelte";

export type { SupportedLocale };
export type LanguageSetting = "system" | "en" | "es";

const dictionaries: Record<SupportedLocale, Record<string, any>> = {
  en,
  es,
};

const listeners = new Set<(loc: SupportedLocale) => void>();

export function getSystemLocale(): SupportedLocale {
  if (typeof navigator !== "undefined" && navigator.language) {
    const lang = navigator.language.toLowerCase();
    if (lang.startsWith("es")) {
      return "es";
    }
  }
  return "en";
}

export function setLocale(lang: LanguageSetting) {
  const resolved: SupportedLocale = lang === "system" ? getSystemLocale() : lang;
  if (typeof document !== "undefined") {
    document.documentElement.setAttribute("lang", resolved);
    const titleVal = getNestedValue(dictionaries[resolved], "app.title");
    if (titleVal) {
      document.title = titleVal;
    }
  }
  try {
    if (typeof localStorage !== "undefined") {
      localStorage.setItem("agentsync_lang", resolved);
    }
  } catch {
    // ignore storage errors
  }
  if (resolved !== localeState.current) {
    localeState.current = resolved;
    listeners.forEach((fn) => fn(resolved));
  }
}

export function getActiveLocale(): SupportedLocale {
  return localeState.current;
}

export function subscribeLocale(fn: (loc: SupportedLocale) => void): () => void {
  listeners.add(fn);
  fn(localeState.current);
  return () => {
    listeners.delete(fn);
  };
}

function getNestedValue(obj: Record<string, any>, path: string): string | undefined {
  const parts = path.split(".");
  let current: any = obj;
  for (const part of parts) {
    if (current && typeof current === "object" && part in current) {
      current = current[part];
    } else {
      return undefined;
    }
  }
  return typeof current === "string" ? current : undefined;
}

export function t(key: string, params?: Record<string, string | number>): string {
  const locale = localeState.current;
  let template = getNestedValue(dictionaries[locale], key);

  if (template === undefined) {
    // Fallback to English
    template = getNestedValue(dictionaries.en, key);
    if (template === undefined) {
      if (import.meta.env.DEV) {
        console.warn(`[i18n] Missing translation key: "${key}" for locale "${locale}"`);
      }
      return key;
    }
  }

  if (params) {
    for (const [k, v] of Object.entries(params)) {
      template = template.replaceAll(`{${k}}`, String(v));
    }
  }

  return template;
}

export function formatDate(isoDateStr: string): string {
  if (!isoDateStr) return "";
  try {
    const date = new Date(isoDateStr);
    return new Intl.DateTimeFormat(localeState.current === "es" ? "es-ES" : "en-US", {
      dateStyle: "medium",
      timeStyle: "short",
    }).format(date);
  } catch {
    return isoDateStr;
  }
}

// Resolve the system language before the first render; persisted config overrides it once loaded.
setLocale("system");
