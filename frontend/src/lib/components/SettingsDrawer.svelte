<script lang="ts">
  import type { main } from "../../../wailsjs/go/models";
  import { appState } from "../stores/appState.svelte";
  import { t, setLocale } from "../i18n";
  import { applyTheme } from "../theme/theme";
  import Drawer from "./Drawer.svelte";
  import Button from "./Button.svelte";
  import Select from "./Select.svelte";
  import Input from "./Input.svelte";
  import Switch from "./Switch.svelte";

  interface Props {
    open: boolean;
    onclose: () => void;
    onsave: (payload: main.SettingsPayload) => void;
  }

  let { open, onclose, onsave }: Props = $props();

  let language = $state<string>("system");
  let theme = $state<string>("system");
  let backupPath = $state<string>("");
  let backupRetention = $state<number>(20);
  let checkInterval = $state<number>(5);
  let confirmBeforeSync = $state<boolean>(true);

  $effect(() => {
    if (appState.config) {
      language = appState.config.language || "system";
      theme = appState.config.theme || "system";
      backupPath = appState.config.backupPath || "";
      backupRetention = appState.config.backupRetention || 20;
      checkInterval = appState.config.checkIntervalMinutes ?? 5;
      confirmBeforeSync = appState.config.confirmBeforeSync ?? true;
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
    onsave({
      language,
      theme,
      backupPath,
      backupRetention: Number(backupRetention) || 20,
      checkIntervalMinutes: Number(checkInterval),
      confirmBeforeSync,
    });
  }
</script>

<Drawer {open} title={t("settings.title")} {onclose}>
  <div class="settings-form">
    <Select
      label={t("settings.language_label")}
      value={language}
      options={[
        { value: "system", label: t("settings.language_system") },
        { value: "en", label: t("settings.language_en") },
        { value: "es", label: t("settings.language_es") },
      ]}
      onchange={handleLanguageChange}
    />

    <Select
      label={t("settings.theme_label")}
      value={theme}
      options={[
        { value: "system", label: t("settings.theme_system") },
        { value: "light", label: t("settings.theme_light") },
        { value: "dark", label: t("settings.theme_dark") },
      ]}
      onchange={handleThemeChange}
    />

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

    <div class="switch-row">
      <Switch
        label={t("settings.confirm_before_sync_label")}
        checked={confirmBeforeSync}
        onchange={(checked) => (confirmBeforeSync = checked)}
      />
    </div>

    <div class="save-actions">
      <Button variant="primary" onclick={handleSave}>
        {t("settings.button_save")}
      </Button>
    </div>
  </div>
</Drawer>

<style>
  .settings-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  .switch-row {
    padding: var(--space-2) 0;
  }

  .save-actions {
    margin-top: var(--space-4);
    display: flex;
    justify-content: flex-end;
  }
</style>
