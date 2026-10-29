import en from "./locales/en.json";
import es from "./locales/es.json";

export type SupportedLocale = "en" | "es";
export type LanguageSetting = "system" | "en" | "es";

const dictionaries: Record<SupportedLocale, Record<string, any>> = {
  en,
  es,
};

let currentLocale: SupportedLocale = "en";
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
  if (resolved !== currentLocale) {
    currentLocale = resolved;
    listeners.forEach((fn) => fn(currentLocale));
  }
}

export function getActiveLocale(): SupportedLocale {
  return currentLocale;
}

export function subscribeLocale(fn: (loc: SupportedLocale) => void): () => void {
  listeners.add(fn);
  fn(currentLocale);
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
  let template = getNestedValue(dictionaries[currentLocale], key);

  if (template === undefined) {
    // Fallback to English
    template = getNestedValue(dictionaries.en, key);
    if (template === undefined) {
      if (import.meta.env.DEV) {
        console.warn(`[i18n] Missing translation key: "${key}" for locale "${currentLocale}"`);
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
    return new Intl.DateTimeFormat(currentLocale === "es" ? "es-ES" : "en-US", {
      dateStyle: "medium",
      timeStyle: "short",
    }).format(date);
  } catch {
    return isoDateStr;
  }
}
