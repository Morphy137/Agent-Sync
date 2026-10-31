import { rules, type rulesync, type service } from "../../../../wailsjs/go/models";
import type { RuleScope } from "../../api";

/** Mirrors rules.Origin* in the Go backend. */
export const ORIGINS = ["repo", "local", "subscription"] as const;
export type SourceOrigin = (typeof ORIGINS)[number];

/** Mirrors rules.Mode* in the Go backend. */
export const MODES = ["two_way", "one_way", "create_only"] as const;
export type DestinationMode = (typeof MODES)[number];
export const DEFAULT_MODE: DestinationMode = "two_way";
/** Transformed destinations always behave as one-way copies. */
export const TRANSFORM_FORCED_MODE: DestinationMode = "one_way";

/** Mirrors rules.Kind* in the Go backend. */
export const KINDS = ["auto", "file", "folder", "skill", "mcp", "instructions"] as const;
export type ContentKind = (typeof KINDS)[number];
export const AUTO_KIND: ContentKind = "auto";
const FOLDER_KINDS: readonly string[] = ["folder", "skill"];

export const TRANSFORM_NONE = "none";
export const TRANSFORM_TYPES = ["json_merge", "toml_merge"] as const;
export type TransformChoice = typeof TRANSFORM_NONE | (typeof TRANSFORM_TYPES)[number];
/** Usual MCP keys, prefilled when the user picks a transform without a key. */
export const DEFAULT_TRANSFORM_KEYS: Record<string, string> = {
  json_merge: "mcpServers",
  toml_merge: "mcp_servers",
};

/** Mirrors rulesync.Dest* states. */
export const DEST_STATES = [
  "ready",
  "location_unmapped",
  "blocked_collision",
  "disabled",
  "source_missing",
] as const;

/** Mirrors rulesync.Status* file statuses. */
export const FILE_STATUSES = [
  "in_sync",
  "new_in_source",
  "source_changed",
  "target_changed",
  "target_missing",
  "conflict",
  "untracked_diff",
  "unreadable",
  "source_deleted",
  "drift",
  "ignored",
] as const;

/** Mirrors rules.Collision* types. */
export const COLLISION_TYPES = [
  "same_target",
  "folder_overlap",
  "cycle",
  "inside_repo",
  "case_conflict",
] as const;

/** Mirrors the rulesync resolution choices (UseSource, UseTarget, UseCustom, KeepLocal). */
export const RESOLUTION = {
  useSource: "use_source",
  useTarget: "use_target",
  custom: "custom",
  keepLocal: "keep_local",
} as const;

export const BUILTIN_HOME_ALIAS = "home";
export const IN_SYNC = "in_sync";

/** Statuses that always require the user to look at a diff before AgentSync touches the file. */
const ALWAYS_REVIEW_STATUSES: readonly string[] = ["conflict", "drift", "untracked_diff"];
const TARGET_CHANGED = "target_changed";

export type DotTone = "ok" | "down" | "up" | "conflict" | "idle";

export function fileDotTone(status: string): DotTone {
  switch (status) {
    case "in_sync":
      return "ok";
    case "new_in_source":
    case "source_changed":
    case "target_missing":
    case "source_deleted":
      return "down";
    case "target_changed":
    case "untracked_diff":
    case "drift":
      return "up";
    case "conflict":
    case "unreadable":
      return "conflict";
    default:
      return "idle";
  }
}

export function destStateTone(state: string): DotTone {
  switch (state) {
    case "ready":
      return "ok";
    case "location_unmapped":
      return "up";
    case "blocked_collision":
    case "source_missing":
      return "conflict";
    default:
      return "idle";
  }
}

/** True when changes made in the destination can be written back to the source. */
export function canWriteBack(rule: rules.Rule, dest: rules.Destination | undefined): boolean {
  return rule.source?.origin !== "subscription" && !dest?.transform;
}

/** True when a file status needs an explicit decision from the user. */
export function needsReview(
  status: string,
  rule: rules.Rule,
  dest: rules.Destination | undefined
): boolean {
  if (ALWAYS_REVIEW_STATUSES.includes(status)) return true;
  if (status !== TARGET_CHANGED) return false;
  return dest?.mode !== "two_way" || !canWriteBack(rule, dest);
}

export function isFolderKind(kind: string | undefined): boolean {
  return FOLDER_KINDS.includes(kind ?? "");
}

export function joinPath(...parts: string[]): string {
  return parts
    .filter((p) => p !== "")
    .map((p, index) => (index === 0 ? p.replace(/[\\/]+$/, "") : p.replace(/^[\\/]+|[\\/]+$/g, "")))
    .join("/");
}

/** Best effort absolute path of a rule source, used by "Open source". */
export function sourceAbsPath(status: rulesync.RuleStatus, rootPath: string): string {
  const src = status.rule.source;
  if (src.origin === "local") return src.path;
  if (src.origin === "repo" && rootPath) return joinPath(rootPath, src.path);
  const file = status.destinations.flatMap((d) => d.files ?? [])[0];
  if (!file?.sourceAbs) return "";
  if (!isFolderKind(status.detectedKind)) return file.sourceAbs;
  const normalized = file.sourceAbs.replace(/\\/g, "/");
  const suffix = `/${file.relPath}`;
  return normalized.endsWith(suffix) ? normalized.slice(0, -suffix.length) : normalized;
}

/** Splits comma or newline separated patterns. */
export function parsePatternList(text: string): string[] {
  return text
    .split(/[,\n]/)
    .map((p) => p.trim())
    .filter((p) => p !== "");
}

export function baseName(path: string): string {
  const parts = path.split(/[\\/]/).filter(Boolean);
  return parts[parts.length - 1] ?? "";
}

export function destinationLabel(dest: Pick<rules.Destination, "location" | "path">): string {
  return dest.path ? `${dest.location}/${dest.path}` : dest.location;
}

/** Editable draft of one destination in the rule editor. */
export interface DestinationDraft {
  /** Local key for keyed each blocks; never sent to the backend. */
  key: string;
  id: string;
  location: string;
  path: string;
  mode: DestinationMode;
  transform: TransformChoice;
  transformKey: string;
  enabled: boolean;
  ignore: string[];
  /** Placement suggestion that prefilled this destination, if any. */
  suggestion: { label: string; detected: boolean } | null;
  /** True once the user edited the path/mode/transform by hand (suggestions stop overriding it). */
  touched: boolean;
}

let draftCounter = 0;
export function newDraftKey(): string {
  draftCounter += 1;
  return `draft-${draftCounter}`;
}

export function emptyDestinationDraft(): DestinationDraft {
  return {
    key: newDraftKey(),
    id: "",
    location: "",
    path: "",
    mode: DEFAULT_MODE,
    transform: TRANSFORM_NONE,
    transformKey: "",
    enabled: true,
    ignore: [],
    suggestion: null,
    touched: false,
  };
}

function asMode(mode: string | undefined): DestinationMode {
  return (MODES as readonly string[]).includes(mode ?? "")
    ? (mode as DestinationMode)
    : DEFAULT_MODE;
}

function asTransform(type: string | undefined): TransformChoice {
  return (TRANSFORM_TYPES as readonly string[]).includes(type ?? "")
    ? (type as TransformChoice)
    : TRANSFORM_NONE;
}

export function draftFromDestination(dest: rules.Destination): DestinationDraft {
  return {
    key: newDraftKey(),
    id: dest.id ?? "",
    location: dest.location ?? "",
    path: dest.path ?? "",
    mode: asMode(dest.mode),
    transform: asTransform(dest.transform?.type),
    transformKey: dest.transform?.key ?? "",
    enabled: dest.enabled ?? true,
    ignore: [...(dest.ignore ?? [])],
    suggestion: null,
    touched: true,
  };
}

/** Applies a placement suggestion to a draft (keeps id, enabled and ignore list). */
export function applySuggestion(
  draft: DestinationDraft,
  suggestion: {
    label: string;
    detected: boolean;
    location: string;
    path: string;
    mode: string;
    transform?: { type: string; key: string };
  }
): DestinationDraft {
  const transform = asTransform(suggestion.transform?.type);
  return {
    ...draft,
    location: suggestion.location || draft.location,
    path: suggestion.path,
    mode: transform === TRANSFORM_NONE ? asMode(suggestion.mode) : TRANSFORM_FORCED_MODE,
    transform,
    transformKey: suggestion.transform?.key ?? "",
    suggestion: { label: suggestion.label, detected: suggestion.detected },
    touched: false,
  };
}

export interface RuleForm {
  id: string;
  name: string;
  enabled: boolean;
  scope: RuleScope;
  source: rules.Source | null;
  kind: ContentKind;
  sourceIsDir: boolean;
  excludeText: string;
  destinations: DestinationDraft[];
}

export type DestinationErrors = Partial<Record<"location" | "path" | "transformKey", string>>;

export interface RuleFormErrors {
  name?: string;
  source?: string;
  scope?: string;
  destinations?: string;
  byDestination: Record<string, DestinationErrors>;
}

/** Translation keys of each validation error (resolved by the caller with t()). */
export const VALIDATION_KEYS = {
  nameRequired: "rules.editor.validation.name_required",
  sourceRequired: "rules.editor.validation.source_required",
  localNeedsLocalScope: "rules.editor.validation.local_needs_local_scope",
  repoScopeNeedsRoot: "rules.editor.validation.repo_scope_needs_root",
  destinationsRequired: "rules.editor.validation.destinations_required",
  locationRequired: "rules.editor.validation.location_required",
  pathRequired: "rules.editor.validation.path_required",
  pathRelative: "rules.editor.validation.path_relative",
  pathDuplicate: "rules.editor.validation.path_duplicate",
  transformKeyRequired: "rules.editor.validation.transform_key_required",
} as const;

const ABSOLUTE_PATH_PATTERN = /^([a-zA-Z]:|[\\/]|~)/;
const PARENT_SEGMENT = "..";

function isSafeRelativePath(path: string): boolean {
  if (ABSOLUTE_PATH_PATTERN.test(path)) return false;
  return !path.split(/[\\/]/).includes(PARENT_SEGMENT);
}

/** Validates the editor form; returned strings are translation keys. */
export function validateRuleForm(form: RuleForm, hasRoot: boolean): RuleFormErrors {
  const errors: RuleFormErrors = { byDestination: {} };
  if (!form.name.trim()) errors.name = VALIDATION_KEYS.nameRequired;
  if (!form.source?.path) errors.source = VALIDATION_KEYS.sourceRequired;
  if (form.scope === "repo" && !hasRoot) errors.scope = VALIDATION_KEYS.repoScopeNeedsRoot;
  else if (form.source?.origin === "local" && form.scope === "repo") {
    errors.scope = VALIDATION_KEYS.localNeedsLocalScope;
  }
  if (form.destinations.length === 0) errors.destinations = VALIDATION_KEYS.destinationsRequired;

  const seen = new Set<string>();
  for (const draft of form.destinations) {
    const destErrors: DestinationErrors = {};
    const path = draft.path.trim();
    if (!draft.location) destErrors.location = VALIDATION_KEYS.locationRequired;
    if (!path) destErrors.path = VALIDATION_KEYS.pathRequired;
    else if (!isSafeRelativePath(path)) destErrors.path = VALIDATION_KEYS.pathRelative;
    else {
      const identity = `${draft.location}/${path.replace(/\\/g, "/")}`.toLowerCase();
      if (seen.has(identity)) destErrors.path = VALIDATION_KEYS.pathDuplicate;
      seen.add(identity);
    }
    if (!form.sourceIsDir && draft.transform !== TRANSFORM_NONE && !draft.transformKey.trim()) {
      destErrors.transformKey = VALIDATION_KEYS.transformKeyRequired;
    }
    if (Object.keys(destErrors).length > 0) errors.byDestination[draft.key] = destErrors;
  }
  return errors;
}

export function hasErrors(errors: RuleFormErrors): boolean {
  return Boolean(
    errors.name ||
    errors.source ||
    errors.scope ||
    errors.destinations ||
    Object.keys(errors.byDestination).length > 0
  );
}

/** Builds the payload for api.saveRule from the editor form. */
export function buildRulePayload(form: RuleForm): rules.Rule {
  const exclude = form.sourceIsDir ? parsePatternList(form.excludeText) : [];
  const source = form.source ?? { origin: "repo", path: "" };
  return rules.Rule.createFrom({
    id: form.id,
    name: form.name.trim(),
    enabled: form.enabled,
    source: {
      origin: source.origin,
      ...(source.origin === "subscription" ? { subscription: source.subscription } : {}),
      path: source.path,
    },
    kind: form.kind,
    ...(exclude.length > 0 ? { exclude } : {}),
    destinations: form.destinations.map((d) => {
      // Transforms only apply to file sources.
      const withTransform = !form.sourceIsDir && d.transform !== TRANSFORM_NONE;
      return {
        id: d.id,
        location: d.location,
        path: d.path.trim().replace(/\\/g, "/"),
        mode: withTransform ? TRANSFORM_FORCED_MODE : d.mode,
        ...(withTransform ? { transform: { type: d.transform, key: d.transformKey.trim() } } : {}),
        enabled: d.enabled,
        ...(d.ignore.length > 0 ? { ignore: d.ignore } : {}),
      };
    }),
  });
}

/** Copy of a rule ready to be saved as a new one (fresh IDs assigned by the backend). */
export function duplicateRule(rule: rules.Rule, name: string): rules.Rule {
  return rules.Rule.createFrom({
    ...rule,
    id: "",
    name,
    destinations: rule.destinations.map((d) => ({ ...d, id: "" })),
  });
}

export interface LocationOption {
  alias: string;
  path: string;
  mapped: boolean;
  builtin: boolean;
}

export function toLocationOptions(locations: service.LocationInfo[]): LocationOption[] {
  return locations.map((l) => ({
    alias: l.alias,
    path: l.path,
    mapped: Boolean(l.path),
    builtin: l.builtin,
  }));
}

export const NEW_LOCATION_VALUE = "__new_location__";
/** Same rule as the backend alias pattern (lower-case letters, digits, "-" and "_"). */
export const ALIAS_PATTERN = /^[a-z0-9][a-z0-9_-]*$/;
export const MAX_ALIAS_LENGTH = 40;

export function normalizeAlias(alias: string): string {
  return alias.trim().toLowerCase();
}

/** Coerces free text into a valid alias shape while typing ("My Vault" -> "my-vault"). */
export function slugifyAlias(raw: string): string {
  return raw
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, "-")
    .replace(/-{2,}/g, "-")
    .replace(/^[-_]+/, "")
    .slice(0, MAX_ALIAS_LENGTH);
}

/** Suggests an alias from a folder path ("C:/Users/me/.claude" -> "claude"). */
export function aliasFromFolder(path: string): string {
  const base =
    path
      .trim()
      .replace(/[\\/]+$/, "")
      .split(/[\\/]/)
      .pop() ?? "";
  return slugifyAlias(base.replace(/^\.+/, "")).replace(/[-_]+$/, "");
}

/** Returns a translation key describing why the alias is invalid, or null. */
export function aliasError(alias: string): string | null {
  const normalized = normalizeAlias(alias);
  if (!normalized) return "locations.form.validation.alias_required";
  if (normalized === BUILTIN_HOME_ALIAS) return "locations.form.validation.alias_reserved";
  if (normalized.length > MAX_ALIAS_LENGTH || !ALIAS_PATTERN.test(normalized)) {
    return "locations.form.validation.alias_invalid";
  }
  return null;
}
