import { api, parseError } from "../api";
import type { config, main, orchestrator, profiles } from "../../../wailsjs/go/models";
import { initTheme } from "../theme/theme";
import { setLocale, t } from "../i18n";
import type { ToastItem } from "../components/Toast.svelte";

class AppState {
  config = $state<config.Config | null>(null);
  gitInfo = $state<main.GitInfoPayload | null>(null);
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
  initialLinkModalOpen = $state(false);
  initialLinkData = $state<main.InitialLinkResult | null>(null);
  addRemoteModalOpen = $state(false);

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

  showError(err: unknown) {
    const parsed = parseError(err);
    const translationKey = `errors.${parsed.code}`;
    let msg = t(translationKey, parsed.params);
    if (msg === translationKey) {
      msg = parsed.message || t("errors.unknown");
    }
    this.addToast("error", msg);
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
      this.isChecking = true;
      const [profilesList, git, plan] = await Promise.all([
        api.listProfiles(),
        api.getGitInfo(),
        api.getStatus(),
      ]);
      this.profiles = profilesList;
      this.gitInfo = git;
      this.statusPlan = plan;
    } catch (err) {
      this.showError(err);
    } finally {
      this.isChecking = false;
    }
  }

  handleStatusChanged(payload: any) {
    if (this.statusPlan) {
      this.statusPlan.direction = payload.direction;
      this.statusPlan.ahead = payload.ahead;
      this.statusPlan.behind = payload.behind;
      this.statusPlan.connections = payload.connections;
    }
    if (this.gitInfo) {
      this.gitInfo.ahead = payload.ahead;
      this.gitInfo.behind = payload.behind;
    }
  }
}

export const appState = new AppState();
