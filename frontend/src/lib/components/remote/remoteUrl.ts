const REMOTE_URL_PATTERN = /^(?:https?:\/\/[^\s/]+\/\S+|ssh:\/\/\S+|git@[^\s:]+:\S+|file:\/\/\S+)$/;
const TRAILING_SLASHES = /[\\/]+$/;
const GIT_SUFFIX = /\.git$/i;
const URL_SEGMENT_SEPARATOR = /[\\/:]/;

/** Client-side pre-check mirroring gitops.ValidateRemoteURL (the backend remains authoritative). */
export function isValidRemoteUrl(url: string): boolean {
  const trimmed = url.trim();
  if (!trimmed || trimmed.startsWith("-") || trimmed.toLowerCase().includes("ext::")) return false;
  const windows = typeof navigator !== "undefined" && /win/i.test(navigator.userAgent);
  const absolutePath =
    /^(?:[a-zA-Z]:[\\/]|\\\\[^\\/]+[\\/][^\\/]+)/.test(trimmed) ||
    (!windows && trimmed.startsWith("/"));
  if (absolutePath) {
    return !/[\x00-\x1f\x7f]/.test(trimmed) && !trimmed.includes("://");
  }
  return REMOTE_URL_PATTERN.test(trimmed);
}

/** Derives a friendly repository name from its URL ("git@host:me/skills.git" -> "skills"). */
export function repoNameFromUrl(url: string): string {
  const cleaned = url.trim().replace(TRAILING_SLASHES, "").replace(GIT_SUFFIX, "");
  return cleaned.split(URL_SEGMENT_SEPARATOR).pop() ?? "";
}
