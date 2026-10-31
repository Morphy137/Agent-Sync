<script lang="ts">
  import type { service } from "../../../wailsjs/go/models";
  import { appState } from "../stores/appState.svelte";
  import { t, setLocale } from "../i18n";
  import { applyTheme, applyPalette, PALETTES, DEFAULT_PALETTE } from "../theme/theme";
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
  let palette = $state<string>(DEFAULT_PALETTE);
  let exclusionsOpen = $state(false);
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
  let initialPalette = $state<string>(DEFAULT_PALETTE);
  let wasOpen = false;

  $effect(() => {
    if (open && !wasOpen) {
      wasOpen = true;
      if (appState.config) {
        language = appState.config.language || "system";
        theme = appState.config.theme || "system";
        initialLanguage = language;
        initialTheme = theme;
        palette = appState.config.palette || DEFAULT_PALETTE;
        initialPalette = palette;
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

  function handlePaletteChange(val: string) {
    palette = val;
    applyPalette(val);
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
      applyTheme(initialTheme as any);
    }
    if (palette !== initialPalette) {
      applyPalette(initialPalette);
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
    initialPalette = palette;
    onsave(
      {
        language,
        theme,
        palette,
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

        <div class="palette-field" role="radiogroup" aria-label={t("settings.palette_label")}>
          <span class="palette-label">{t("settings.palette_label")}</span>
          <div class="palette-grid">
            {#each PALETTES as option (option)}
              <button
                type="button"
                role="radio"
                class="palette-swatch"
                class:selected={palette === option}
                data-palette={option}
                aria-checked={palette === option}
                onclick={() => handlePaletteChange(option)}
              >
                <span class="swatch-dot" aria-hidden="true"></span>
                <span class="swatch-name">{t(`settings.palette.${option}`)}</span>
              </button>
            {/each}
          </div>
        </div>

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
        <h3 class="section-heading">{t("settings.section_help")}</h3>
        <Button variant="secondary" size="sm" onclick={handleReplayTutorial}>
          {t("settings.replay_tutorial")}
        </Button>
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

  .palette-field {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .palette-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .palette-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--space-2);
  }

  .palette-swatch {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    height: 32px;
    padding: 0 var(--space-2);
    border: none;
    border-radius: var(--radius-sm);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--ink);
    font-family: var(--font-sans);
    font-size: var(--text-xs);
    cursor: pointer;
  }

  .palette-swatch.selected {
    box-shadow: var(--focus-ring);
  }

  .palette-swatch:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .swatch-dot {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
    border-radius: var(--radius-full);
    background-color: var(--swatch);
  }

  .palette-swatch[data-palette="graphite"] {
    --swatch: #3a3a40;
  }
  .palette-swatch[data-palette="ocean"] {
    --swatch: #2563eb;
  }
  .palette-swatch[data-palette="forest"] {
    --swatch: #16a34a;
  }
  .palette-swatch[data-palette="violet"] {
    --swatch: #7c3aed;
  }
  .palette-swatch[data-palette="rose"] {
    --swatch: #e11d48;
  }
  .palette-swatch[data-palette="amber"] {
    --swatch: #d97706;
  }
  .palette-swatch[data-palette="nord"] {
    --swatch: #5e81ac;
  }
  .palette-swatch[data-palette="sepia"] {
    --swatch: #9a5b2a;
  }
  .palette-swatch[data-palette="midnight"] {
    --swatch: #3b4ccf;
  }

  .swatch-name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
