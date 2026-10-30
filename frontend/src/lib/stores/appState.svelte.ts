import { api, parseError } from "../api";
import type { config, service, orchestrator, profiles } from "../../../wailsjs/go/models";
import { initTheme } from "../theme/theme";
import { setLocale, t } from "../i18n";
import type { ToastItem } from "../components/Toast.svelte";

export type AppView = "rules" | "connections" | "subscriptions" | "locations";

class AppState {
  activeView = $state<AppView>("rules");
  config = $state<config.Config | null>(null);
  gitInfo = $state<service.GitInfoPayload | null>(null);
  profiles = $state<profiles.Profile[]>([]);
  statusPlan = $state<orchestrator.SyncPlan | null>(null);
  isChecking = $state(false);
  isSyncing = $state(false);
  toasts = $state<ToastItem[]>([]);

  // Modals & Drawers state
  settingsDrawerOpen = $state(false);
  backupsDrawerOpen = $state(false);
  activeBackupConnectionId = $state<string | null>(null);
  activeBackupProfileName = $state<string>("");
  previewModalOpen = $state(false);
  connectionWizardOpen = $state(false);
  connectionWizardProfile = $state<string>("");
  connectionWizardReplacesId = $state<string>("");
  addRemoteModalOpen = $state(false);
  gitIdentityModalOpen = $state(false);

  addToast(type: "success" | "error" | "info", message: string) {
    const id = Math.random().toString(36).slice(2, 9);
    this.toasts = [...this.toasts, { id, type, message }];
    setTimeout(() => {
      this.dismissToast(id);
    }, 4000);
  }

  dismissToast(id: string) {
    this.toasts = this.toasts.filter((t) => t.id !== id);
  }

  showError(err: unknown): string {
    const parsed = parseError(err);
    const translationKey = `errors.${parsed.code}`;
    let msg = t(translationKey, parsed.params);
    if (msg === translationKey) {
      msg = parsed.message || t("errors.unknown");
    }
    this.addToast("error", msg);
    if (parsed.code === "git_identity_missing") {
      this.gitIdentityModalOpen = true;
    }
    return parsed.code;
  }

  async loadInitialData() {
    try {
      this.isChecking = true;
      const cfg = await api.getConfig();
      this.config = cfg;

      if (cfg) {
        initTheme((cfg.theme as any) || "system");
        setLocale((cfg.language as any) || "system");
      }

      if (cfg?.rootPath) {
        await this.refreshAll();
      }
    } catch (err) {
      this.showError(err);
    } finally {
      this.isChecking = false;
    }
  }

  async refreshAll() {
    try {
      const [cfg, profilesList, git, plan] = await Promise.all([
        api.getConfig(),
        api.listProfiles(),
        api.getGitInfo(),
        api.getStatus(),
      ]);
      this.config = cfg;
      this.profiles = profilesList;
      this.gitInfo = git;
      this.statusPlan = plan;
    } catch (err) {
      this.showError(err);
    }
  }

  async checkRemote() {
    try {
      this.isChecking = true;
      const [plan, git] = await Promise.all([
        api.checkRemote(),
        api.getGitInfo(),
      ]);
      this.statusPlan = plan;
      this.gitInfo = git;
    } catch (err) {
      this.showError(err);
    } finally {
      this.isChecking = false;
    }
  }

  // H5: Replace the entire statusPlan store when status:changed event arrives
  handleStatusChanged(plan: orchestrator.SyncPlan) {
    if (!plan) return;
    this.statusPlan = plan;
    if (this.gitInfo) {
      this.gitInfo.ahead = plan.ahead;
      this.gitInfo.behind = plan.behind;
    }
  }
}

export const appState = new AppState();
