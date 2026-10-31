import { describeError } from "../i18n/errors";
import { api, parseError } from "../api";
import type { config, service, orchestrator, profiles, rules } from "../../../wailsjs/go/models";
import type { RuleScope } from "../api";
import { initTheme } from "../theme/theme";
import { setLocale } from "../i18n";
import type { ToastItem } from "../components/Toast.svelte";

export type AppView = "rules" | "connections" | "subscriptions" | "locations";

export interface RuleEditorState {
  open: boolean;
  /** Existing rule to edit, or null to create a new one. */
  rule: rules.Rule | null;
  scope: RuleScope;
  /** Source to preselect when creating (e.g. picked from a subscription tree). */
  presetSource: rules.Source | null;
}

class AppState {
  activeView = $state<AppView>("rules");
  ruleEditor = $state<RuleEditorState>({
    open: false,
    rule: null,
    scope: "repo",
    presetSource: null,
  });
  /** Incremented whenever rules, locations or subscriptions change; views reload when it changes. */
  rulesVersion = $state(0);

  openRuleEditor(options: Partial<Omit<RuleEditorState, "open">> = {}) {
    this.ruleEditor = {
      open: true,
      rule: options.rule ?? null,
      scope: options.scope ?? (this.config?.rootPath ? "repo" : "local"),
      presetSource: options.presetSource ?? null,
    };
  }

  closeRuleEditor() {
    this.ruleEditor = { ...this.ruleEditor, open: false };
  }

  bumpRules() {
    this.rulesVersion++;
  }
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
  tutorialOpen = $state(false);

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
    this.addToast("error", describeError(err));
    if (parsed.code === "git_identity_missing") {
      this.gitIdentityModalOpen = true;
    }
    return parsed.code;
  }

  /** Closes the tutorial and remembers it on this PC so it does not reopen at startup. */
  async finishTutorial() {
    this.tutorialOpen = false;
    if (this.config?.tutorialCompleted) return;
    try {
      this.config = await api.setTutorialCompleted(true);
    } catch (err) {
      this.showError(err);
    }
  }

  async loadInitialData() {
    try {
      this.isChecking = true;
      const cfg = await api.getConfig();
      this.config = cfg;

      if (cfg) {
        initTheme(cfg.theme || "system");
        setLocale((cfg.language as any) || "system");
      }

      if (cfg?.rootPath) {
        await this.refreshAll();
        if (!cfg.tutorialCompleted) this.tutorialOpen = true;
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
      this.profiles = profilesList ?? [];
      this.gitInfo = git;
      this.statusPlan = plan;
    } catch (err) {
      this.showError(err);
    }
  }

  async checkRemote() {
    try {
      this.isChecking = true;
      const [plan, git] = await Promise.all([api.checkRemote(), api.getGitInfo()]);
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
