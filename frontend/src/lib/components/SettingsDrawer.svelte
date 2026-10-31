<script lang="ts">
  import type { service } from "../../../wailsjs/go/models";
  import { appState } from "../stores/appState.svelte";
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
    onsave: (
      payload: service.SettingsPayload,
      gitName: string,
      gitEmail: string,
      newRootPath?: string
    ) => void;
  }

  let { open, onclose, onsave }: Props = $props();

  let language = $state<string>("system");
  let theme = $state<string>("system");
  let exclusionsOpen = $state(false);
  let changelogOpen = $state(false);

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
  let backupRetention = $state<number>(20);
  let checkInterval = $state<number>(5);
  let confirmBeforeSync = $state<boolean>(true);
  let gitUserName = $state<string>("");
  let gitUserEmail = $state<string>("");

  let rootPath = $state<string>("");
  let initialRootPath = $state<string>("");
  let confirmRootModalOpen = $state(false);

  // Snapshots for revert on cancel (R5)
  let initialLanguage = $state<string>("system");
  let initialTheme = $state<string>("system");
  let wasOpen = false;

  $effect(() => {
    if (open && !wasOpen) {
      wasOpen = true;
      if (appState.config) {
        language = appState.config.language || "system";
        theme = appState.config.theme || "system";
        initialLanguage = language;
        initialTheme = theme;
        backupPath = appState.config.backupPath || "";
        backupRetention = appState.config.backupRetention || 20;
        checkInterval = appState.config.checkIntervalMinutes ?? 5;
        confirmBeforeSync = appState.config.confirmBeforeSync ?? true;
        checkUpdates = !appState.config.disableUpdateCheck;
        rootPath = appState.config.rootPath || "";
        initialRootPath = rootPath;
      }
      if (appState.gitInfo) {
        gitUserName = appState.gitInfo.userName || "";
        gitUserEmail = appState.gitInfo.userEmail || "";
      }
    } else if (!open && wasOpen) {
      wasOpen = false;
    }
  });

  function handleLanguageChange(val: string) {
    language = val;
    setLocale(val as any);
  }

  function handleThemeChange(val: string) {
    theme = val;
    applyTheme(val);
  }

  function handleReplayTutorial() {
    handleClose();
    appState.tutorialOpen = true;
  }

  function handleClose() {
    // Revert unsaved theme & language (R5)
    if (language !== initialLanguage) {
      setLocale(initialLanguage as any);
    }
    if (theme !== initialTheme) {
      applyTheme(initialTheme);
    }
    onclose();
  }

  function handleSave() {
    if (rootPath.trim() && rootPath.trim() !== initialRootPath) {
      confirmRootModalOpen = true;
      return;
    }
    doSave();
  }

  function doSave() {
    initialLanguage = language;
    initialTheme = theme;
    onsave(
      {
        language,
        theme,
        backupPath,
        backupRetention: Number(backupRetention) || 20,
        checkIntervalMinutes: Number(checkInterval),
        confirmBeforeSync,
        disableUpdateCheck: !checkUpdates,
      },
      gitUserName.trim(),
      gitUserEmail.trim(),
      rootPath.trim() !== initialRootPath ? rootPath.trim() : undefined
    );
    confirmRootModalOpen = false;
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
          onchange={(val) => (checkInterval = Number(val))}
        />

        <Switch
          label={t("settings.confirm_before_sync_label")}
          checked={confirmBeforeSync}
          onchange={(checked) => (confirmBeforeSync = checked)}
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
          onchange={(v) => (backupPath = v)}
        />

        <Input
          label={t("settings.backup_retention_label")}
          type="number"
          value={backupRetention}
          oninput={(v) => (backupRetention = Number(v))}
        />
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
            onchange={(v) => (rootPath = v)}
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
              oninput={(v) => (gitUserName = v)}
            />

            <Input
              label={t("settings.git_email_label")}
              placeholder={t("settings.git_email_placeholder")}
              value={gitUserEmail}
              oninput={(v) => (gitUserEmail = v)}
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
          onchange={(checked) => (checkUpdates = checked)}
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

    <div class="save-actions">
      <Button variant="primary" onclick={handleSave}>
        {t("settings.button_save")}
      </Button>
    </div>
  </div>
</Drawer>

<GitignoreModal open={exclusionsOpen} onclose={() => (exclusionsOpen = false)} />
<ChangelogModal open={changelogOpen} onclose={() => (changelogOpen = false)} />

<Modal
  open={confirmRootModalOpen}
  title={t("settings.confirm_root_change_title")}
  onclose={() => (confirmRootModalOpen = false)}
>
  <div class="confirm-root-body">
    <p class="confirm-root-text">{t("settings.confirm_root_change_msg")}</p>
    <div class="confirm-root-actions">
      <Button variant="secondary" onclick={() => (confirmRootModalOpen = false)}>
        {t("common.cancel")}
      </Button>
      <Button variant="primary" onclick={doSave}>
        {t("common.confirm")}
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
    width: 36px;
    height: 36px;
    flex-shrink: 0;
    border-radius: var(--radius-sm);
    background-color: color-mix(in srgb, var(--accent) 14%, transparent);
    color: var(--accent);
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

  .save-actions {
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
