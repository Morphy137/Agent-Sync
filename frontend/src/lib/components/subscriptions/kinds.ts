import { File, FileText, Folder, Plug, Sparkles } from "@lucide/svelte";
import { t } from "../../i18n";
import type { EntryKind } from "./tree";

export function kindLabel(kind: EntryKind): string {
  switch (kind) {
    case "skill":
      return t("subscriptions.kinds.skill");
    case "mcp":
      return t("subscriptions.kinds.mcp");
    case "instructions":
      return t("subscriptions.kinds.instructions");
    case "folder":
      return t("subscriptions.kinds.folder");
    default:
      return t("subscriptions.kinds.file");
  }
}

export function syncActionLabel(kind: EntryKind): string {
  switch (kind) {
    case "skill":
      return t("subscriptions.tree.sync_skill");
    case "mcp":
      return t("subscriptions.tree.sync_mcp");
    case "instructions":
      return t("subscriptions.tree.sync_instructions");
    case "folder":
      return t("subscriptions.tree.sync_folder");
    default:
      return t("subscriptions.tree.sync_file");
  }
}

export function kindIcon(kind: EntryKind): typeof File {
  switch (kind) {
    case "skill":
      return Sparkles;
    case "mcp":
      return Plug;
    case "instructions":
      return FileText;
    case "folder":
      return Folder;
    default:
      return File;
  }
}
