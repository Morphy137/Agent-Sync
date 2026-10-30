import { describe, it, expect } from "vitest";
import { t, setLocale, getDictionary } from "../src/lib/i18n";

const en = getDictionary("en");
const es = getDictionary("es");

function getAllKeys(obj: Record<string, any>, prefix = ""): string[] {
  let keys: string[] = [];
  for (const [k, v] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${k}` : k;
    if (v && typeof v === "object" && !Array.isArray(v)) {
      keys = keys.concat(getAllKeys(v, fullKey));
    } else {
      keys.push(fullKey);
    }
  }
  return keys.sort();
}

describe("i18n Parity & Coverage", () => {
  it("en.json and es.json should have identical keys", () => {
    const enKeys = getAllKeys(en);
    const esKeys = getAllKeys(es);

    const missingInEs = enKeys.filter((k) => !esKeys.includes(k));
    const missingInEn = esKeys.filter((k) => !enKeys.includes(k));

    expect(missingInEs, `Missing in es.json: ${missingInEs.join(", ")}`).toEqual([]);
    expect(missingInEn, `Missing in en.json: ${missingInEn.join(", ")}`).toEqual([]);
  });

  it("should have translations for all apperr codes", () => {
    const apperrCodes = [
      "root_not_found",
      "root_not_repo",
      "git_not_installed",
      "git_no_remote",
      "git_auth_failed",
      "git_network",
      "git_not_fast_forward",
      "git_rebase_conflict",
      "git_dirty_outside_profiles",
      "git_dirty_worktree",
      "git_identity_missing",
      "git_push_rejected",
      "target_not_found",
      "target_inside_root",
      "target_already_connected",
      "profile_not_found",
      "profile_already_connected",
      "plan_changed",
      "unresolved_conflicts",
      "backup_failed",
      "write_failed",
      "operation_in_progress",
      "invalid_folder_name",
      "folder_exists",
      "path_not_absolute",
      "clone_failed",
      "draft_not_found",
      "unknown",
    ];

    for (const code of apperrCodes) {
      const enKey = `errors.${code}`;
      setLocale("en");
      const enTranslation = t(enKey);
      expect(enTranslation, `Missing English error translation for ${enKey}`).not.toBe(enKey);

      setLocale("es");
      const esTranslation = t(enKey);
      expect(esTranslation, `Missing Spanish error translation for ${enKey}`).not.toBe(enKey);
    }
  });

  it("should interpolate parameters correctly", () => {
    setLocale("en");
    expect(t("statusbar.ahead_behind", { ahead: 2, behind: 5 })).toBe("↑2 ↓5");
    setLocale("es");
    expect(t("statusbar.ahead_behind", { ahead: 2, behind: 5 })).toBe("↑2 ↓5");
  });
});
