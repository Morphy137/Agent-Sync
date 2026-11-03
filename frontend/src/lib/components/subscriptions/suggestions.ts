import type { service } from "../../../../wailsjs/go/models";
import { t } from "../../i18n";

export const SUGGESTION_APPLIED = "applied";
export const SUGGESTION_SKIPPED = "skipped";

const CODE_LOCATION_UNMAPPED = "location_unmapped";
const CODE_INVALID_SUGGESTIONS = "invalid_suggestions";
const FOLDER_ERROR_CODES = ["target_not_found", "path_not_absolute", "location_invalid"];
const MODE_CREATE_ONLY = "create_only";
const UNKNOWN_REASON = "unknown";

export type Folders = Record<string, string>;

/** A suggestion can be ticked when it is valid and not applied yet. */
export function isSelectable(suggestion: service.SuggestionInfo): boolean {
  return suggestion.valid && !suggestion.applied;
}

/** Location aliases that a suggestion still needs a folder for on this PC. */
export function unmappedAliases(suggestion: service.SuggestionInfo): string[] {
  const aliases = (suggestion.destinations ?? [])
    .filter((dest) => !dest.mapped && !dest.applied)
    .map((dest) => dest.location);
  return [...new Set(aliases)];
}

/** Folders the app recognized on this PC; the user still confirms them by applying. */
export function detectedFolders(suggestions: service.SuggestionInfo[]): Folders {
  const folders: Folders = {};
  for (const suggestion of suggestions) {
    for (const dest of suggestion.destinations ?? []) {
      if (!dest.mapped && dest.detectedPath && !folders[dest.location]) {
        folders[dest.location] = dest.detectedPath;
      }
    }
  }
  return folders;
}

export function initialSelection(suggestions: service.SuggestionInfo[]): string[] {
  return suggestions.filter(isSelectable).map((suggestion) => suggestion.id);
}

/** Aliases of the selected suggestions that still have no folder. */
export function missingFolderAliases(
  suggestions: service.SuggestionInfo[],
  selected: ReadonlySet<string>,
  folders: Folders
): string[] {
  const missing = new Set<string>();
  for (const suggestion of suggestions) {
    if (!selected.has(suggestion.id)) continue;
    for (const alias of unmappedAliases(suggestion)) {
      if (!folders[alias]) missing.add(alias);
    }
  }
  return [...missing];
}

/** Only the folders the selected suggestions need are sent to the backend. */
export function mappingsFor(
  suggestions: service.SuggestionInfo[],
  selected: ReadonlySet<string>,
  folders: Folders
): Folders {
  const mappings: Folders = {};
  for (const suggestion of suggestions) {
    if (!selected.has(suggestion.id)) continue;
    for (const alias of unmappedAliases(suggestion)) {
      if (folders[alias]) mappings[alias] = folders[alias];
    }
  }
  return mappings;
}

export function invalidReasonText(reason: string): string {
  const key = `subscriptions.suggestions.invalid.${reason}`;
  const text = t(key);
  return text === key ? t(`subscriptions.suggestions.invalid.${UNKNOWN_REASON}`) : text;
}

/** Plain-language message for a suggestion the backend refused to apply. */
export function failureText(result: service.SuggestionApplyResult): string {
  const alias = result.errorAlias;
  if (result.errorCode === CODE_LOCATION_UNMAPPED) {
    return t("subscriptions.suggestions.errors.folder_missing", { alias });
  }
  if (FOLDER_ERROR_CODES.includes(result.errorCode)) {
    return t("subscriptions.suggestions.errors.folder_invalid", { alias });
  }
  if (result.errorCode === CODE_INVALID_SUGGESTIONS) {
    const known = `subscriptions.suggestions.errors.${result.errorReason}`;
    return t(known) === known ? invalidReasonText(result.errorReason) : t(known);
  }
  if (result.errorReason) return invalidReasonText(result.errorReason);
  const coreKey = `errors.${result.errorCode}`;
  const core = t(coreKey);
  return core === coreKey ? t("errors.unknown") : core;
}

/** True when a destination with this mode keeps an existing file instead of replacing it. */
export function keepsExistingFile(dest: service.SuggestionDestinationInfo): boolean {
  return dest.mode === MODE_CREATE_ONLY;
}
