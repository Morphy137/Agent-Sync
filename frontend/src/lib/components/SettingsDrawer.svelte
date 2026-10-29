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

  interface Props {
    open: boolean;
    onclose: () => void;
    onsave: (
      payload: service.SettingsPayload,
      gitName: string,
      gitEmail: string
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

  $effect(() => {
    if (open && appState.config) {
      language = appState.config.language || "system";
      theme = appState.config.theme || "system";
      backupPath = appState.config.backupPath || "";
      backupRetention = appState.config.backupRetention || 20;
      checkInterval = appState.config.checkIntervalMinutes ?? 5;
      confirmBeforeSync = appState.config.confirmBeforeSync ?? true;
    }
    if (open && appState.gitInfo) {
      gitUserName = appState.gitInfo.userName || "";
      gitUserEmail = appState.gitInfo.userEmail || "";
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

  function handleSave() {
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
      gitUserEmail.trim()
    );
  }
</script>

<Drawer {open} title={t("settings.title")} {onclose}>
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
            { value: 1, label: t("settings.interval_minutes", { count: 1 }) },
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

        <Input
          label={t("settings.backup_path_label")}
          value={backupPath}
          oninput={(v) => (backupPath = v)}
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

          <Input
            label={t("settings.root_path_label")}
            value={appState.config.rootPath}
            disabled={true}
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
</style>
