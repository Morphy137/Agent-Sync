import type { service } from "../../../../wailsjs/go/models";
import { t } from "../../i18n";

/** The search box only appears once the list is long enough to need it. */
export const SEARCH_THRESHOLD = 8;

const BYTES_PER_UNIT = 1024;
const SIZE_UNITS = ["B", "KB", "MB", "GB"];
const FRACTION_DIGITS = 1;

export const BACKUP_KIND = {
  rule: "rule",
  source: "source",
  repo: "repo",
  legacy: "legacy",
} as const;

/** Reasons the backend writes; anything else is shown as stored. */
const KNOWN_REASONS = new Set(["sync", "discard_repo_edit", "restore"]);

export interface BackupGroup {
  key: string;
  owner: string;
  destination: string;
  entries: service.BackupEntry[];
}

export function formatSize(bytes: number): string {
  let value = bytes;
  let unit = 0;
  while (value >= BYTES_PER_UNIT && unit < SIZE_UNITS.length - 1) {
    value /= BYTES_PER_UNIT;
    unit++;
  }
  const text = unit === 0 ? String(value) : value.toFixed(FRACTION_DIGITS);
  return `${text} ${SIZE_UNITS[unit]}`;
}

/** Picks the singular key (`<base>_one`) for exactly one item. */
export function countKey(base: string, count: number): string {
  return count === 1 ? `${base}_one` : base;
}

export function reasonLabel(reason: string): string {
  return KNOWN_REASONS.has(reason) ? t(`backups.reason.${reason}`) : reason;
}

/** The name shown for who owns a backup: the rule, the repository or an older connection. */
export function ownerLabel(entry: service.BackupEntry): string {
  if (entry.kind === BACKUP_KIND.repo) return t("backups.owner_repo");
  return entry.owner || t("backups.owner_unknown");
}

/** Where a backup's files belong: the rule destination, the rule source or nothing known. */
export function destinationLabel(entry: service.BackupEntry): string {
  switch (entry.kind) {
    case BACKUP_KIND.source:
      return t("backups.destination_source");
    case BACKUP_KIND.legacy:
      return t("backups.destination_legacy");
    case BACKUP_KIND.repo:
      return "";
    default:
      return entry.destination;
  }
}

function groupKey(entry: service.BackupEntry): string {
  return [entry.kind, entry.ruleId || entry.owner, entry.destId].join("|");
}

/** Groups entries by rule and destination, keeping the newest-first order of the list. */
export function groupBackups(entries: service.BackupEntry[]): BackupGroup[] {
  const groups = new Map<string, BackupGroup>();
  for (const entry of entries) {
    const key = groupKey(entry);
    let group = groups.get(key);
    if (!group) {
      group = {
        key,
        owner: ownerLabel(entry),
        destination: destinationLabel(entry),
        entries: [],
      };
      groups.set(key, group);
    }
    group.entries.push(entry);
  }
  return [...groups.values()];
}

/** Matches the query against the owner, destination, folder and file names. */
export function filterBackups(
  entries: service.BackupEntry[],
  query: string
): service.BackupEntry[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return entries;
  return entries.filter((entry) =>
    [ownerLabel(entry), destinationLabel(entry), entry.targetPath, ...(entry.files ?? [])].some(
      (text) => text.toLowerCase().includes(needle)
    )
  );
}
