import { describeError } from "../i18n/errors";
import { api, parseError } from "../api";
import type { config, service, orchestrator, rules, rulesync } from "../../../wailsjs/go/models";
import type { RuleScope } from "../api";
import { initTheme } from "../theme/theme";
import { updateState } from "./updates.svelte";
import { setLocale, t } from "../i18n";
import type { ToastItem } from "../components/Toast.svelte";
import type { AppNotification } from "../components/notifications/NotificationCenter.svelte";

/**
 * Default values of the settings shown in the settings drawer.
 * Mirrors the Default* constants and DefaultConfig() in internal/config/config.go.
 * The backup folder default is <data dir>/backups, so it is resolved from the app info.
 */
export const SETTINGS_DEFAULTS = {
  language: "system",
  theme: "system",
  backupRetention: 20,
  checkIntervalMinutes: 5,
  confirmBeforeSync: true,
  disableUpdateCheck: false,
  // The background options are off by default (config.DefaultAutoApplyUpdates, DefaultCloseToTray
  // and DefaultStartWithWindows).
  autoApplyUpdates: false,
  closeToTray: false,
  startWithWindows: false,
  backupFolderName: "backups",
} as const;

export type AppView = "rules" | "files" | "subscriptions" | "locations";

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
  fileRuleId = $state("");
  fileDirty = $state(false);
  ruleEditor = $state<RuleEditorState>({
    open: false,
    rule: null,
    scope: "local",
    presetSource: null,
  });
  /** Incremented whenever rules, locations or subscriptions change; views reload when it changes. */
  rulesVersion = $state(0);
  ruleReport = $state<rulesync.Report | null>(null);
  collisionDetailsOpen = $state(false);
  alertNotifications = $state<AppNotification[]>([]);
  private ruleReportRequest = 0;
  private localStatusRequest = 0;
  private ruleReportRoot = "";

  get ruleNotifications(): AppNotification[] {
    const collisions = this.ruleReport?.collisions ?? [];
    const unmapped = this.ruleReport?.unmapped ?? [];
    const items: AppNotification[] = [];
    if (collisions.length)
      items.push({
        id: `rule_collisions:${this.config?.rootPath ?? ""}`,
        variant: "warning",
        title: t(`rules.collisions.banner_title_${collisions.length === 1 ? "one" : "other"}`, {
          count: collisions.length,
        }),
        message: t("rules.collisions.banner_message"),
        actions: [
          {
            label: t("rules.collisions.review"),
            primary: true,
            onclick: () => {
              if (this.navigate("rules")) this.collisionDetailsOpen = true;
            },
          },
        ],
      });
    if (unmapped.length)
      items.push({
        id: `rule_unmapped:${this.config?.rootPath ?? ""}`,
        variant: "warning",
        title: t(`rules.unmapped.title_${unmapped.length === 1 ? "one" : "other"}`, {
          count: unmapped.length,
        }),
        message: t("rules.unmapped.message", { aliases: unmapped.join(", ") }),
        actions: [
          {
            label: t("rules.unmapped.action"),
            primary: true,
            onclick: () => {
              this.navigate("locations");
            },
          },
        ],
      });
    return [...items, ...this.alertNotifications];
  }

  navigate(view: AppView): boolean {
    if (this.fileDirty && !window.confirm(t("files.unsaved_confirm"))) return false;
    this.fileDirty = false;
    this.activeView = view;
    return true;
  }

  async refreshRuleReport(): Promise<rulesync.Report | null> {
    const request = ++this.ruleReportRequest;
    if (!this.config?.rootPath) {
      this.ruleReport = null;
      this.ruleReportRoot = "";
      return null;
    }
    try {
      const report = await api.getRulesReport();
      if (request === this.ruleReportRequest) this.setRuleReport(report);
    } catch (err) {
      if (request === this.ruleReportRequest) this.showError(err);
    }
    return this.ruleReport;
  }

  setRuleReport(report: rulesync.Report) {
    this.ruleReportRequest++;
    const root = this.config?.rootPath ?? "";
    const previous = this.ruleReportRoot === root ? this.ruleNotifications : [];
    for (const id of ["rule_collisions", "rule_unmapped"]) {
      const old = previous.find((item) => item.id.startsWith(`${id}:`));
      const stillActive =
        id === "rule_collisions" ? report.collisions?.length : report.unmapped?.length;
      if (old && !stillActive) {
        this.alertNotifications = [
          ...this.alertNotifications,
          {
            id: `${id}_resolved_${this.ruleReportRequest}`,
            variant: "info",
            title: t("notifications.resolved_title", { title: old.title }),
            message: old.message,
          },
        ];
      }
    }
    this.ruleReport = report;
    this.ruleReportRoot = root;
  }

  openRuleEditor(options: Partial<Omit<RuleEditorState, "open">> = {}) {
    this.ruleEditor = {
      open: true,
      rule: options.rule ?? null,
      scope: options.scope ?? "local",
      presetSource: options.presetSource ?? null,
    };
  }

  closeRuleEditor() {
    this.ruleEditor = { ...this.ruleEditor, open: false };
  }

  bumpRules() {
    this.rulesVersion++;
    void this.refreshRuleReport();
  }

  /**
   * Recomputes the local sync plan (no network) so pending rule work and the Sync button
   * show right after a rule changes. Does not touch the global checking state.
   */
  async refreshLocalStatus() {
    if (!this.config?.rootPath) return;
    const request = ++this.localStatusRequest;
    try {
      const plan = await api.getStatus();
      if (request === this.localStatusRequest) this.statusPlan = plan;
    } catch (err) {
      this.showError(err);
    }
  }

  /** Reloads rule views and the pending state after a rule is saved, toggled or deleted. */
  afterRuleChange() {
    this.bumpRules();
    void this.refreshLocalStatus();
  }

  config = $state<config.Config | null>(null);
  configLoaded = $state(false);
  gitInfo = $state<service.GitInfoPayload | null>(null);
  statusPlan = $state<orchestrator.SyncPlan | null>(null);
  isChecking = $state(false);
  isSyncing = $state(false);
  toasts = $state<ToastItem[]>([]);

  // Modals & Drawers state
  settingsDrawerOpen = $state(false);
  previewModalOpen = $state(false);
  addRemoteModalOpen = $state(false);
  gitIdentityModalOpen = $state(false);
  tutorialOpen = $state(false);

  addToast(type: "success" | "error" | "info", message: string) {
    const id = Math.random().toString(36).slice(2, 9);
    if (type === "error") {
      if (this.alertNotifications.some((item) => item.message === message)) return;
      this.alertNotifications = [
        ...this.alertNotifications,
        { id: `alert_${id}`, variant: "warning", title: t("notifications.error_title"), message },
      ];
      return;
    }
    this.toasts = [...this.toasts, { id, type, message }];
    setTimeout(() => {
      this.dismissToast(id);
    }, 4000);
  }

  /** Informational bell notification; a new one replaces the previous one with the same prefix. */
  addNotice(idPrefix: string, title: string, message: string) {
    const id = `${idPrefix}_${Math.random().toString(36).slice(2, 9)}`;
    const kept = this.alertNotifications.filter((item) => !item.id.startsWith(`${idPrefix}_`));
    this.alertNotifications = [...kept, { id, variant: "info", title, message }];
  }

  addWarning(message: string) {
    if (this.alertNotifications.some((item) => item.message === message)) return;
    const id = Math.random().toString(36).slice(2, 9);
    this.alertNotifications = [
      ...this.alertNotifications,
      { id: `alert_${id}`, variant: "warning", title: t("notifications.warning_title"), message },
    ];
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
      this.configLoaded = true;

      if (cfg) {
        initTheme(cfg.theme || "system");
        setLocale((cfg.language as any) || "system");
      }

      void updateState.loadInfo();
      updateState.startAutoCheck(!cfg?.disableUpdateCheck);

      if (cfg?.rootPath) {
        await this.refreshAll();
        if (!cfg.tutorialCompleted) this.tutorialOpen = true;
      }
    } catch (err) {
      this.showError(err);
    } finally {
      this.configLoaded = true;
      this.isChecking = false;
    }
  }

  async refreshAll() {
    try {
      const [cfg, git, plan] = await Promise.all([
        api.getConfig(),
        api.getGitInfo(),
        api.getStatus(),
      ]);
      this.config = cfg;
      this.gitInfo = git;
      this.statusPlan = plan;
      void this.refreshRuleReport();
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
    void this.refreshRuleReport();
    if (this.gitInfo) {
      this.gitInfo.ahead = plan.ahead;
      this.gitInfo.behind = plan.behind;
    }
  }
}

export const appState = new AppState();
