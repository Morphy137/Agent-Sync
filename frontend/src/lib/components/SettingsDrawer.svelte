<script lang="ts">
  import type { service } from "../../../wailsjs/go/models";
  import { appState, SETTINGS_DEFAULTS } from "../stores/appState.svelte";
  import { t, setLocale } from "../i18n";
  import { applyTheme } from "../theme/theme";
  import Drawer from "./Drawer.svelte";
  import Card from "./Card.svelte";
  import Button from "./Button.svelte";
  import Select from "./Select.svelte";
  import Input from "./Input.svelte";
  import Switch from "./Switch.svelte";
  import SegmentedControl from "./SegmentedControl.svelte";
  import PathField from "./PathField.svelte";
  import Modal from "./Modal.svelte";
  import GitignoreModal from "./GitignoreModal.svelte";
  import ThemePicker from "./ThemePicker.svelte";
  import ChangelogModal from "./ChangelogModal.svelte";
  import { Star, Sparkles, GraduationCap } from "@lucide/svelte";
  import { api } from "../api";
  import { updateState } from "../stores/updates.svelte";

  interface Props {
    open: boolean;
    onclose: () => void;
  }

  let { open, onclose }: Props = $props();

  /** Wait after the last keystroke before free-text and numeric settings are saved. */
  const AUTOSAVE_DEBOUNCE_MS = 600;
  const MIN_BACKUP_RETENTION = 1;

  let language = $state<string>("system");
  let theme = $state<string>("system");
  let exclusionsOpen = $state(false);
  let changelogOpen = $state(false);
  let restoreModalOpen = $state(false);

  /** Project page; starring it helps others find AgentSync. */
  const PROJECT_URL = "https://github.com/Morphy137/Agent-Sync";

  const appVersion = $derived(updateState.info?.version ?? "");
  const installMode = $derived(
    updateState.info?.portable
      ? "portable"
      : updateState.info?.installed
        ? "installed"
        : "standalone"
  );
  let checkUpdates = $state(true);

  function toast(type: "success" | "error" | "info", message: string) {
    appState.addToast(type, message);
  }
  let backupPath = $state<string>("");
  let backupPathUsable = $state(true);
  let backupRetention = $state<number>(SETTINGS_DEFAULTS.backupRetention);
  let checkInterval = $state<number>(SETTINGS_DEFAULTS.checkIntervalMinutes);
  let confirmBeforeSync = $state<boolean>(SETTINGS_DEFAULTS.confirmBeforeSync);
  let gitUserName = $state<string>("");
  let gitUserEmail = $state<string>("");

  let rootPath = $state<string>("");
  let confirmRootModalOpen = $state(false);

  const retentionInvalid = $derived(
    !Number.isInteger(backupRetention) || backupRetention < MIN_BACKUP_RETENTION
  );

  let wasOpen = false;
  let saveTimer: ReturnType<typeof setTimeout> | null = null;
  let saveChain: Promise<void> = Promise.resolve();

  $effect(() => {
    if (open && !wasOpen) {
      wasOpen = true;
      loadFromState();
    } else if (!open && wasOpen) {
      wasOpen = false;
    }
  });

  function loadFromState() {
    const cfg = appState.config;
    if (cfg) {
      language = cfg.language || SETTINGS_DEFAULTS.language;
      theme = cfg.theme || SETTINGS_DEFAULTS.theme;
      backupPath = cfg.backupPath || "";
      backupPathUsable = true;
      backupRetention = cfg.backupRetention || SETTINGS_DEFAULTS.backupRetention;
      checkInterval = cfg.checkIntervalMinutes ?? SETTINGS_DEFAULTS.checkIntervalMinutes;
      confirmBeforeSync = cfg.confirmBeforeSync ?? SETTINGS_DEFAULTS.confirmBeforeSync;
      checkUpdates = !cfg.disableUpdateCheck;
      rootPath = cfg.rootPath || "";
    }
    if (appState.gitInfo) {
      gitUserName = appState.gitInfo.userName || "";
      gitUserEmail = appState.gitInfo.userEmail || "";
    }
  }

  /** Settings to send: invalid fields keep their last saved value so they are never overwritten. */
  function buildPayload(): service.SettingsPayload {
    const saved = appState.config;
    return {
      language,
      theme,
      backupPath: backupPathUsable && backupPath.trim() ? backupPath : (saved?.backupPath ?? ""),
      backupRetention: retentionInvalid
        ? (saved?.backupRetention ?? SETTINGS_DEFAULTS.backupRetention)
        : backupRetention,
      checkIntervalMinutes: checkInterval,
      confirmBeforeSync,
      disableUpdateCheck: !checkUpdates,
    };
  }

  function isSameAsSaved(payload: service.SettingsPayload): boolean {
    const saved = appState.config;
    if (!saved) return false;
    return (
      payload.language === (saved.language || SETTINGS_DEFAULTS.language) &&
      payload.theme === (saved.theme || SETTINGS_DEFAULTS.theme) &&
      payload.backupPath === saved.backupPath &&
      payload.backupRetention === saved.backupRetention &&
      payload.checkIntervalMinutes === saved.checkIntervalMinutes &&
      payload.confirmBeforeSync === saved.confirmBeforeSync &&
      payload.disableUpdateCheck === Boolean(saved.disableUpdateCheck)
    );
  }

  /** Puts the last saved settings back in the fields and the live theme and language. */
  function restoreSavedSettings() {
    loadFromState();
    applyTheme(theme);
    setLocale(language as any);
  }

  async function persistSettings() {
    const payload = buildPayload();
    if (!isSameAsSaved(payload)) {
      try {
        appState.config = await api.updateSettings(payload);
      } catch (err) {
        appState.showError(err);
        restoreSavedSettings();
        return;
      }
    }
    await persistGitIdentity();
  }

  async function persistGitIdentity() {
    const name = gitUserName.trim();
    const email = gitUserEmail.trim();
    const info = appState.gitInfo;
    if (!info?.isRepo || !email) return;
    if (name === (info.userName || "") && email === (info.userEmail || "")) return;
    try {
      await api.setGitIdentity(name, email);
      appState.gitInfo = await api.getGitInfo();
    } catch (err) {
      appState.showError(err);
    }
  }

  /** Saves one at a time, always from the latest field values, so an older save cannot win. */
  function saveNow(): Promise<void> {
    if (saveTimer) {
      clearTimeout(saveTimer);
      saveTimer = null;
    }
    saveChain = saveChain.then(persistSettings);
    return saveChain;
  }

  function saveDebounced() {
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(() => void saveNow(), AUTOSAVE_DEBOUNCE_MS);
  }

  function handleLanguageChange(val: string) {
    language = val;
    setLocale(val as any);
    void saveNow();
  }

  function handleThemeChange(val: string) {
    theme = val;
    applyTheme(val);
    void saveNow();
  }

  function handleIntervalChange(val: string | number) {
    checkInterval = Number(val);
    void saveNow();
  }

  function handleConfirmBeforeSyncChange(checked: boolean) {
    confirmBeforeSync = checked;
    void saveNow();
  }

  function handleCheckUpdatesChange(checked: boolean) {
    checkUpdates = checked;
    void saveNow();
  }

  function isUsableFolder(info: service.PathInfo | null): boolean {
    return Boolean(info?.exists && info.isDir && info.writable && !info.isInsideRoot);
  }

  function handleBackupPathChange(value: string, info: service.PathInfo | null) {
    backupPath = value;
    backupPathUsable = isUsableFolder(info);
    if (backupPathUsable && value.trim() !== (appState.config?.backupPath ?? "")) void saveNow();
  }

  function handleRetentionInput(value: string) {
    backupRetention = Number(value);
    if (!retentionInvalid) saveDebounced();
  }

  function handleRootPathChange(value: string, info: service.PathInfo | null) {
    rootPath = value;
    const current = appState.config?.rootPath ?? "";
    if (!value.trim() || value.trim() === current || !isUsableFolder(info)) return;
    confirmRootModalOpen = true;
  }

  function cancelRootChange() {
    confirmRootModalOpen = false;
    rootPath = appState.config?.rootPath ?? "";
  }

  async function confirmRootChange() {
    confirmRootModalOpen = false;
    const nextRoot = rootPath.trim();
    try {
      await saveChain;
      appState.config = await api.setRootPath(nextRoot);
      await appState.refreshAll();
    } catch (err) {
      appState.showError(err);
      rootPath = appState.config?.rootPath ?? "";
    }
  }

  /** Backups default to a "backups" folder inside the app data folder. */
  function defaultBackupPath(): string {
    const dataDir = updateState.info?.dataDir;
    if (!dataDir) return appState.config?.backupPath ?? "";
    const separator = dataDir.includes("\\") ? "\\" : "/";
    return `${dataDir.replace(/[\\/]+$/, "")}${separator}${SETTINGS_DEFAULTS.backupFolderName}`;
  }

  async function confirmRestoreDefaults() {
    restoreModalOpen = false;
    language = SETTINGS_DEFAULTS.language;
    theme = SETTINGS_DEFAULTS.theme;
    backupPath = defaultBackupPath();
    backupPathUsable = true;
    backupRetention = SETTINGS_DEFAULTS.backupRetention;
    checkInterval = SETTINGS_DEFAULTS.checkIntervalMinutes;
    confirmBeforeSync = SETTINGS_DEFAULTS.confirmBeforeSync;
    checkUpdates = !SETTINGS_DEFAULTS.disableUpdateCheck;
    setLocale(language as any);
    applyTheme(theme);
    await saveNow();
  }

  function handleReplayTutorial() {
    handleClose();
    appState.tutorialOpen = true;
  }

  function handleClose() {
    if (saveTimer) void saveNow();
    onclose();
  }
</script>

<Drawer {open} title={t("settings.title")} onclose={handleClose}>
  <div class="settings-sections">
    <!-- 1. Appearance -->
    <Card padding="md">
      <div class="section-block">
        <h3 class="section-heading">{t("settings.section_appearance")}</h3>

        <ThemePicker value={theme} onchange={handleThemeChange} />

        <SegmentedControl
          label={t("settings.language_label")}
          value={language}
          options={[
            { value: "system", label: t("settings.language_system") },
            { value: "en", label: t("settings.language_en") },
            { value: "es", label: t("settings.language_es") },
          ]}
          onchange={handleLanguageChange}
        />
      </div>
    </Card>

    <!-- 2. Synchronization -->
    <Card padding="md">
      <div class="section-block">
        <h3 class="section-heading">{t("settings.section_sync")}</h3>

        <Select
          label={t("settings.check_interval_label")}
          value={checkInterval}
          options={[
            { value: 0, label: t("settings.interval_disabled") },
            { value: 1, label: t("settings.interval_one_minute") },
            { value: 5, label: t("settings.interval_minutes", { count: 5 }) },
            { value: 15, label: t("settings.interval_minutes", { count: 15 }) },
            { value: 30, label: t("settings.interval_minutes", { count: 30 }) },
          ]}
          onchange={handleIntervalChange}
        />

        <Switch
          label={t("settings.confirm_before_sync_label")}
          checked={confirmBeforeSync}
          onchange={handleConfirmBeforeSyncChange}
        />
      </div>
    </Card>

    <!-- 3. Backups -->
    <Card padding="md">
      <div class="section-block">
        <h3 class="section-heading">{t("settings.section_backups")}</h3>

        <PathField
          label={t("settings.backup_path_label")}
          value={backupPath}
          mode="backup"
          allowCreate={true}
          onchange={handleBackupPathChange}
        />

        <Input
          label={t("settings.backup_retention_label")}
          type="number"
          value={backupRetention}
          oninput={handleRetentionInput}
        />
        {#if retentionInvalid}
          <p class="field-error" role="alert">{t("settings.retention_invalid")}</p>
        {/if}
      </div>
    </Card>

    <!-- 4. Repository -->
    {#if appState.config?.rootPath}
      <Card padding="md">
        <div class="section-block">
          <h3 class="section-heading">{t("settings.section_repository")}</h3>

          <PathField
            label={t("settings.root_path_label")}
            value={rootPath}
            mode="root"
            allowCreate={false}
            onchange={handleRootPathChange}
          />

          {#if appState.gitInfo?.isRepo}
            <Input
              label={t("settings.remote_url_label")}
              value={appState.gitInfo.remoteUrl || "—"}
              disabled={true}
            />

            <Input
              label={t("settings.git_name_label")}
              placeholder={t("settings.git_name_placeholder")}
              value={gitUserName}
              oninput={(v) => {
                gitUserName = v;
                saveDebounced();
              }}
            />

            <Input
              label={t("settings.git_email_label")}
              placeholder={t("settings.git_email_placeholder")}
              value={gitUserEmail}
              oninput={(v) => {
                gitUserEmail = v;
                saveDebounced();
              }}
            />
            <p class="field-hint">{t("settings.git_identity_hint")}</p>

            <Button variant="secondary" size="sm" onclick={() => (exclusionsOpen = true)}>
              {t("exclusions.open_button")}
            </Button>
          {/if}
        </div>
      </Card>
    {/if}

    <!-- 5. Help -->
    <Card padding="md">
      <div class="section-block">
        <div class="help-heading">
          <h3 class="section-heading">{t("settings.section_help")}</h3>
          {#if appVersion}
            <span class="app-version">{t("settings.app_version", { version: appVersion })}</span>
          {/if}
        </div>
        <div class="update-row">
          <div class="update-text">
            <span class="update-title">
              {updateState.available
                ? t("updates.available_title", {
                    version: updateState.status?.release?.version ?? "",
                  })
                : t(`updates.mode.${installMode}`)}
            </span>
            {#if updateState.info?.dataDir}
              <span class="update-path" title={updateState.info.dataDir}>
                {t("updates.data_dir", { path: updateState.info.dataDir })}
              </span>
            {/if}
          </div>
          {#if updateState.available}
            <Button
              variant="primary"
              size="sm"
              loading={updateState.installing}
              disabled={updateState.installing}
              onclick={() => updateState.install(toast)}
            >
              {updateState.canAutoInstall ? t("updates.install") : t("updates.download")}
            </Button>
          {:else}
            <Button
              variant="secondary"
              size="sm"
              loading={updateState.checking}
              disabled={updateState.checking}
              onclick={() => updateState.check(true, toast)}
            >
              {t("updates.check_now")}
            </Button>
          {/if}
        </div>
        <Switch
          label={t("updates.auto_check")}
          checked={checkUpdates}
          onchange={handleCheckUpdatesChange}
        />

        <div class="help-actions">
          <Button variant="secondary" size="sm" onclick={() => (changelogOpen = true)}>
            <Sparkles size={14} />
            <span>{t("settings.whats_new")}</span>
          </Button>
          <Button variant="secondary" size="sm" onclick={handleReplayTutorial}>
            <GraduationCap size={14} />
            <span>{t("settings.replay_tutorial")}</span>
          </Button>
        </div>

        <button
          type="button"
          class="star-card"
          aria-label={t("settings.star_github")}
          onclick={() => api.openURL(PROJECT_URL)}
        >
          <span class="star-icon" aria-hidden="true"><Star size={18} /></span>
          <span class="star-text">
            <span class="star-title">{t("settings.star_github")}</span>
            <span class="star-desc">{t("settings.star_github_desc")}</span>
          </span>
        </button>
      </div>
    </Card>

    <div class="footer-actions">
      <Button variant="secondary" onclick={() => (restoreModalOpen = true)}>
        {t("settings.restore_defaults_button")}
      </Button>
    </div>
  </div>
</Drawer>

<GitignoreModal open={exclusionsOpen} onclose={() => (exclusionsOpen = false)} />
<ChangelogModal open={changelogOpen} onclose={() => (changelogOpen = false)} />

<Modal
  open={confirmRootModalOpen}
  title={t("settings.confirm_root_change_title")}
  onclose={cancelRootChange}
>
  <div class="confirm-root-body">
    <p class="confirm-root-text">{t("settings.confirm_root_change_msg")}</p>
    <div class="confirm-root-actions">
      <Button variant="secondary" onclick={cancelRootChange}>
        {t("common.cancel")}
      </Button>
      <Button variant="primary" onclick={confirmRootChange}>
        {t("common.confirm")}
      </Button>
    </div>
  </div>
</Modal>

<Modal
  open={restoreModalOpen}
  title={t("settings.restore_defaults_title")}
  onclose={() => (restoreModalOpen = false)}
>
  <div class="confirm-root-body">
    <p class="confirm-root-text">{t("settings.restore_defaults_msg")}</p>
    <div class="confirm-root-actions">
      <Button variant="secondary" onclick={() => (restoreModalOpen = false)}>
        {t("common.cancel")}
      </Button>
      <Button variant="primary" onclick={confirmRestoreDefaults}>
        {t("settings.restore_defaults_confirm")}
      </Button>
    </div>
  </div>
</Modal>

<style>
  .settings-sections {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .section-block {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .section-heading {
    margin: 0;
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .field-hint {
    margin: calc(var(--space-2) * -1) 0 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.5;
  }

  .help-heading {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
  }

  .app-version {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .update-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .update-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .update-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    color: var(--ink);
  }

  .update-path {
    font-size: var(--text-xs);
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .help-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-2);
  }

  .star-card {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    padding: var(--space-3);
    border: none;
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--ink);
    font-family: var(--font-sans);
    text-align: left;
    cursor: pointer;
    transition: box-shadow var(--duration-fast) var(--easing-out);
  }

  .star-card:hover {
    box-shadow:
      var(--sunken),
      0 0 0 1px color-mix(in srgb, var(--accent) 45%, transparent);
  }

  .star-card:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .star-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    color: var(--warning);
  }

  .star-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .star-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
  }

  .star-desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.4;
  }

  .field-error {
    margin: calc(var(--space-2) * -1) 0 0;
    font-size: var(--text-xs);
    color: var(--danger);
    line-height: 1.5;
  }

  .footer-actions {
    margin-top: var(--space-2);
    display: flex;
    justify-content: flex-end;
  }

  .confirm-root-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .confirm-root-text {
    font-size: var(--text-sm);
    color: var(--text-muted);
    margin: 0;
  }

  .confirm-root-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }
</style>
