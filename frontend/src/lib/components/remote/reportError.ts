import { parseError } from "../../api";
import { t } from "../../i18n";
import { appState } from "../../stores/appState.svelte";

/**
 * Shows an error toast. Codes not yet translated in the core `errors.*` dictionary fall back to
 * `remote.errors.*`; everything else (including the git identity flow) goes through appState.showError.
 */
export function reportError(err: unknown): string {
  const parsed = parseError(err);
  const coreKey = `errors.${parsed.code}`;
  const fallbackKey = `remote.errors.${parsed.code}`;
  if (t(coreKey) === coreKey && t(fallbackKey) !== fallbackKey) {
    appState.addToast("error", t(fallbackKey, parsed.params));
    return parsed.code;
  }
  return appState.showError(err);
}
