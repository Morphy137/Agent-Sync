import type { orchestrator } from "../../../../wailsjs/go/models";
import type { RepoDiffMode } from "../../api";
import { t } from "../../i18n";

export type ChangeStatus = "added" | "modified" | "deleted" | "renamed" | "untracked" | "changed";

const UNTRACKED_CODE = "??";

/** Maps a `git status --porcelain` XY code to a simple change status. */
export function classifyGitStatus(code: string): ChangeStatus {
  const trimmed = code.trim();
  if (trimmed === UNTRACKED_CODE) return "untracked";
  if (trimmed.includes("R")) return "renamed";
  if (trimmed.includes("D")) return "deleted";
  if (trimmed.includes("A")) return "added";
  if (trimmed.includes("M")) return "modified";
  return "changed";
}

export function changeStatusLabel(status: ChangeStatus): string {
  switch (status) {
    case "added":
      return t("remote.status.added");
    case "modified":
      return t("remote.status.modified");
    case "deleted":
      return t("remote.status.deleted");
    case "renamed":
      return t("remote.status.renamed");
    case "untracked":
      return t("remote.status.untracked");
    default:
      return t("remote.status.changed");
  }
}

/**
 * Files listed as downloads change because of the remote when the branch is behind
 * (HEAD vs upstream); otherwise the repository copy itself was edited (HEAD vs working tree).
 */
export function downloadDiffMode(plan: orchestrator.SyncPlan | null): RepoDiffMode {
  return (plan?.behind ?? 0) > 0 ? "incoming" : "working";
}

export function diffModeTitle(mode: RepoDiffMode): string {
  return mode === "incoming" ? t("remote.diff.incoming_title") : t("remote.diff.working_title");
}
