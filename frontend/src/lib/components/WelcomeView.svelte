<script lang="ts">
  import { describeError } from "../i18n/errors";
  import { Folder, FolderPlus, GitFork, AlertCircle } from "@lucide/svelte";
  import appIcon from "../../assets/app-icon-rounded.png";
  import type { service } from "../../../wailsjs/go/models";
  import {
    SetRootPath,
    CreateRoot,
    CloneRepository,
    InitRepository,
  } from "../../../wailsjs/go/main/App";
  import { appState } from "../stores/appState.svelte";
  import { t } from "../i18n";
  import Button from "./Button.svelte";
  import PathField from "./PathField.svelte";

  interface Props {
    onsuccess?: () => void;
  }

  let { onsuccess }: Props = $props();

  let selectedOption = $state<"existing" | "create" | "clone">("existing");
  let isLoading = $state(false);
  let errorMessage = $state<string | null>(null);

  // Option 1: Existing
  let existingPath = $state("");
  let existingPathInfo = $state<service.PathInfo | null>(null);

  // Option 2: Create
  let createParentDir = $state("");
  let createParentInfo = $state<service.PathInfo | null>(null);
  let createFolderName = $state("agent-configs");

  // Option 3: Clone
  let cloneUrl = $state("");
  let cloneParentDir = $state("");
  let cloneParentInfo = $state<service.PathInfo | null>(null);
  let cloneFolderName = $state("agent-configs");

  const RESERVED_WINDOWS_NAMES = [
    "CON",
    "PRN",
    "AUX",
    "NUL",
    "COM1",
    "COM2",
    "COM3",
    "COM4",
    "COM5",
    "COM6",
    "COM7",
    "COM8",
    "COM9",
    "LPT1",
    "LPT2",
    "LPT3",
    "LPT4",
    "LPT5",
    "LPT6",
    "LPT7",
    "LPT8",
    "LPT9",
  ];
  const INVALID_CHARS_REGEX = /[<>:"/\\|?*\x00-\x1F]/;

  function isValidName(name: string): boolean {
    const trimmed = name.trim();
    if (!trimmed) return false;
    if (trimmed.startsWith(".") || trimmed.endsWith(".") || name.endsWith(" ")) return false;
    if (INVALID_CHARS_REGEX.test(trimmed)) return false;
    if (RESERVED_WINDOWS_NAMES.includes(trimmed.toUpperCase())) return false;
    return true;
  }

  function handleCloneUrlInput(e: Event) {
    const val = (e.target as HTMLInputElement).value;
    cloneUrl = val;
    // Auto-derive folder name from git URL
    const match = val.trim().match(/\/([^/]+?)(\.git)?$/);
    if (match && match[1] && isValidName(match[1])) {
      cloneFolderName = match[1];
    }
  }

  async function handleUseExisting() {
    if (!existingPathInfo?.exists || !existingPathInfo?.isDir || isLoading) return;
    isLoading = true;
    errorMessage = null;
    try {
      const cfg = await SetRootPath(existingPath.trim());
      appState.config = cfg;
      await appState.refreshAll();
      onsuccess?.();
    } catch (err: any) {
      errorMessage = describeError(err);
      appState.showError(err);
    } finally {
      isLoading = false;
    }
  }

  async function handleCreateNew() {
    if (
      !createParentInfo?.exists ||
      !createParentInfo?.isDir ||
      !isValidName(createFolderName) ||
      isLoading
    ) {
      return;
    }
    isLoading = true;
    errorMessage = null;
    try {
      const cfg = await CreateRoot(createParentDir.trim(), createFolderName.trim());
      appState.config = cfg;
      try {
        await InitRepository();
      } catch {
        // init is optional if git is not installed or repo init deferred
      }
      await appState.refreshAll();
      onsuccess?.();
    } catch (err: any) {
      errorMessage = describeError(err);
      appState.showError(err);
    } finally {
      isLoading = false;
    }
  }

  async function handleCloneRepo() {
    if (
      !cloneUrl.trim() ||
      !cloneParentInfo?.exists ||
      !cloneParentInfo?.isDir ||
      !isValidName(cloneFolderName) ||
      isLoading
    ) {
      return;
    }
    isLoading = true;
    errorMessage = null;
    try {
      const cfg = await CloneRepository(
        cloneUrl.trim(),
        cloneParentDir.trim(),
        cloneFolderName.trim()
      );
      appState.config = cfg;
      await appState.refreshAll();
      onsuccess?.();
    } catch (err: any) {
      errorMessage = describeError(err);
      appState.showError(err);
    } finally {
      isLoading = false;
    }
  }
</script>

<div class="onboarding-wrapper">
  <div class="onboarding-container">
    <div class="onboarding-header">
      <img class="header-icon" src={appIcon} alt="" />
      <h1 class="header-title">{t("onboarding.title")}</h1>
      <p class="header-subtitle">{t("onboarding.subtitle")}</p>
    </div>

    {#if errorMessage}
      <div class="onboarding-error">
        <AlertCircle size={18} />
        <span>{errorMessage}</span>
      </div>
    {/if}

    <div class="options-grid">
      <!-- Option 1: Existing Folder -->
      <div
        class="option-card"
        class:selected={selectedOption === "existing"}
        onclick={() => (selectedOption = "existing")}
        role="button"
        tabindex="0"
        onkeydown={(e) => {
          if (e.key === "Enter" || e.key === " ") selectedOption = "existing";
        }}
      >
        <div class="card-header">
          <div class="card-icon">
            <Folder size={24} />
          </div>
          <div class="card-title-group">
            <h3 class="card-title">{t("onboarding.opt_existing_title")}</h3>
            <p class="card-desc">{t("onboarding.opt_existing_desc")}</p>
          </div>
        </div>

        {#if selectedOption === "existing"}
          <div class="card-form" onclick={(e) => e.stopPropagation()} role="presentation">
            <PathField
              value={existingPath}
              mode="root"
              allowCreate={false}
              onchange={(val, info) => {
                existingPath = val;
                existingPathInfo = info;
              }}
            />
            <div class="form-actions">
              <Button
                variant="primary"
                disabled={!existingPathInfo?.exists || !existingPathInfo?.isDir || isLoading}
                loading={isLoading}
                onclick={handleUseExisting}
              >
                {t("onboarding.btn_use_folder")}
              </Button>
            </div>
          </div>
        {/if}
      </div>

      <!-- Option 2: Create New Folder -->
      <div
        class="option-card"
        class:selected={selectedOption === "create"}
        onclick={() => (selectedOption = "create")}
        role="button"
        tabindex="0"
        onkeydown={(e) => {
          if (e.key === "Enter" || e.key === " ") selectedOption = "create";
        }}
      >
        <div class="card-header">
          <div class="card-icon">
            <FolderPlus size={24} />
          </div>
          <div class="card-title-group">
            <h3 class="card-title">{t("onboarding.opt_create_title")}</h3>
            <p class="card-desc">{t("onboarding.opt_create_desc")}</p>
          </div>
        </div>

        {#if selectedOption === "create"}
          <div class="card-form" onclick={(e) => e.stopPropagation()} role="presentation">
            <PathField
              label={t("onboarding.parent_folder_label")}
              value={createParentDir}
              mode="backup"
              allowCreate={true}
              onchange={(val, info) => {
                createParentDir = val;
                createParentInfo = info;
              }}
            />
            <div class="input-field-group">
              <label for="create-folder-name" class="field-label">
                {t("onboarding.folder_name_label")}
              </label>
              <input
                id="create-folder-name"
                class="form-input"
                type="text"
                value={createFolderName}
                aria-invalid={createFolderName.length > 0 && !isValidName(createFolderName)}
                aria-describedby={createFolderName.length > 0 && !isValidName(createFolderName)
                  ? "create-name-error"
                  : undefined}
                oninput={(e) => (createFolderName = (e.target as HTMLInputElement).value)}
              />
              {#if createFolderName.length > 0 && !isValidName(createFolderName)}
                <p class="field-error" id="create-name-error" role="alert">
                  {t("onboarding.invalid_folder_name")}
                </p>
              {/if}
            </div>
            <div class="form-actions">
              <Button
                variant="primary"
                disabled={!createParentInfo?.exists ||
                  !createParentInfo?.isDir ||
                  !isValidName(createFolderName) ||
                  isLoading}
                loading={isLoading}
                onclick={handleCreateNew}
              >
                {t("onboarding.btn_create_folder")}
              </Button>
            </div>
          </div>
        {/if}
      </div>

      <!-- Option 3: Clone Repository -->
      <div
        class="option-card"
        class:selected={selectedOption === "clone"}
        onclick={() => (selectedOption = "clone")}
        role="button"
        tabindex="0"
        onkeydown={(e) => {
          if (e.key === "Enter" || e.key === " ") selectedOption = "clone";
        }}
      >
        <div class="card-header">
          <div class="card-icon">
            <GitFork size={24} />
          </div>
          <div class="card-title-group">
            <h3 class="card-title">{t("onboarding.opt_clone_title")}</h3>
            <p class="card-desc">{t("onboarding.opt_clone_desc")}</p>
          </div>
        </div>

        {#if selectedOption === "clone"}
          <div class="card-form" onclick={(e) => e.stopPropagation()} role="presentation">
            <div class="input-field-group">
              <label for="clone-url-input" class="field-label">
                {t("onboarding.repo_url_label")}
              </label>
              <input
                id="clone-url-input"
                class="form-input"
                type="text"
                value={cloneUrl}
                placeholder={t("onboarding.repo_url_placeholder")}
                oninput={handleCloneUrlInput}
              />
            </div>
            <PathField
              label={t("onboarding.parent_folder_label")}
              value={cloneParentDir}
              mode="backup"
              allowCreate={true}
              onchange={(val, info) => {
                cloneParentDir = val;
                cloneParentInfo = info;
              }}
            />
            <div class="input-field-group">
              <label for="clone-folder-name" class="field-label">
                {t("onboarding.folder_name_label")}
              </label>
              <input
                id="clone-folder-name"
                class="form-input"
                type="text"
                value={cloneFolderName}
                aria-invalid={cloneFolderName.length > 0 && !isValidName(cloneFolderName)}
                aria-describedby={cloneFolderName.length > 0 && !isValidName(cloneFolderName)
                  ? "clone-name-error"
                  : undefined}
                oninput={(e) => (cloneFolderName = (e.target as HTMLInputElement).value)}
              />
              {#if cloneFolderName.length > 0 && !isValidName(cloneFolderName)}
                <p class="field-error" id="clone-name-error" role="alert">
                  {t("onboarding.invalid_folder_name")}
                </p>
              {/if}
            </div>
            <div class="form-actions">
              <Button
                variant="primary"
                disabled={!cloneUrl.trim() ||
                  !cloneParentInfo?.exists ||
                  !cloneParentInfo?.isDir ||
                  !isValidName(cloneFolderName) ||
                  isLoading}
                loading={isLoading}
                onclick={handleCloneRepo}
              >
                {#if isLoading}
                  <span>{t("onboarding.cloning")}</span>
                {:else}
                  <span>{t("onboarding.btn_clone")}</span>
                {/if}
              </Button>
            </div>
          </div>
        {/if}
      </div>
    </div>
  </div>
</div>

<style>
  .onboarding-wrapper {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 100%;
    padding: var(--space-6) var(--space-4);
  }

  .onboarding-container {
    max-width: 680px;
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .onboarding-header {
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .header-icon {
    width: 56px;
    height: 56px;
    color: var(--accent);
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: var(--space-3);
  }

  .header-title {
    font-size: var(--text-2xl);
    font-weight: var(--weight-bold);
    color: var(--ink);
    margin: 0 0 var(--space-2) 0;
  }

  .header-subtitle {
    font-size: var(--text-sm);
    color: var(--text-muted);
    max-width: 480px;
    margin: 0;
    line-height: 1.5;
  }

  .onboarding-error {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-md);
    background-color: var(--danger-bg);
    color: var(--danger);
    font-size: var(--text-sm);
  }

  .options-grid {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .option-card {
    background-color: var(--surface);
    border-radius: var(--radius-xl);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--elev-1);
    padding: var(--space-5);
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-standard);
    outline: none;
  }

  .option-card:hover {
    border-color: var(--border);
    box-shadow: var(--elev-2);
  }

  .option-card:focus-visible,
  .option-card.selected:focus-visible {
    box-shadow: var(--focus-ring);
  }

  .option-card.selected {
    border-color: var(--accent);
    box-shadow:
      var(--elev-2),
      0 0 0 1px var(--accent);
    cursor: default;
  }

  .card-header {
    display: flex;
    align-items: flex-start;
    gap: var(--space-4);
  }

  .card-icon {
    width: 44px;
    height: 44px;
    border-radius: var(--radius-lg);
    background-color: var(--surface-sunken);
    color: var(--ink);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .option-card.selected .card-icon {
    color: var(--accent);
    background-color: var(--surface-sunken);
  }

  .card-title-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    flex: 1;
  }

  .card-title {
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
    margin: 0;
  }

  .card-desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin: 0;
    line-height: 1.4;
  }

  .card-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    margin-top: var(--space-4);
    padding-top: var(--space-4);
    border-top: 1px solid var(--border-subtle);
  }

  .input-field-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    width: 100%;
  }

  .field-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .form-input {
    height: 36px;
    padding: 0 var(--space-3);
    border-radius: var(--radius-md);
    border: none;
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--ink);
    font-family: var(--font-sans);
    font-size: var(--text-sm);
    outline: none;
  }

  .form-input:focus-visible {
    box-shadow: var(--focus-ring);
  }

  .field-error {
    margin: 0;
    color: var(--danger);
    font-size: var(--text-xs);
  }

  .form-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: var(--space-2);
  }
</style>
