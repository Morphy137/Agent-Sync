export type SupportedLocale = "en" | "es";

// Reactive holder so every template calling t() re-renders when the locale changes.
export const localeState = $state<{ current: SupportedLocale }>({ current: "en" });
