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
    applyTheme(val as any);
  }

  function handleClose() {
    // Revert unsaved theme & language (R5)
    if (language !== initialLanguage) {
      setLocale(initialLanguage as any);
    }
    if (theme !== initialTheme) {
      applyTheme(initialTheme as any);
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

        <SegmentedControl
          label={t("settings.theme_label")}
          value={theme}
          options={[
            { value: "system", label: t("settings.theme_system") },
            { value: "light", label: t("settings.theme_light") },
            { value: "dark", label: t("settings.theme_dark") },
          ]}
          onchange={handleThemeChange}
        />

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
          {/if}
        </div>
      </Card>
    {/if}

    <div class="save-actions">
      <Button variant="primary" onclick={handleSave}>
        {t("settings.button_save")}
      </Button>
    </div>
  </div>
</Drawer>

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
