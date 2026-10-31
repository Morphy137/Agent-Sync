import { parseError } from "../api";
import { t } from "./index";

const ERROR_NAMESPACES = ["errors", "remote.errors"] as const;

/** Turns a backend error ("[code] {params}") into a translated, user-facing sentence. */
export function describeError(err: unknown): string {
  const parsed = parseError(err);
  if (parsed.code === "unknown") return parsed.message || t("errors.unknown");
  for (const namespace of ERROR_NAMESPACES) {
    const key = `${namespace}.${parsed.code}`;
    const translated = t(key, parsed.params);
    if (translated !== key) return translated;
  }
  return t("errors.unknown");
}
