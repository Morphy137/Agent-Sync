<script lang="ts">
  import { describeError } from "../i18n/errors";
  import { Folder, FolderPlus, ArrowRight, ArrowLeft, Check, AlertCircle } from "@lucide/svelte";
  import type { service, profiles, orchestrator } from "../../../wailsjs/go/models";
  import {
    ListProfiles,
    DetectAgentFolders,
    ListTargetEntries,
    BeginConnection,
    CommitConnection,
    CancelConnection,
  } from "../../../wailsjs/go/main/App";
  import { appState } from "../stores/appState.svelte";
  import { t } from "../i18n";
  import Modal from "./Modal.svelte";
  import Button from "./Button.svelte";
  import PathField from "./PathField.svelte";
  import SegmentedControl from "./SegmentedControl.svelte";
  import Spinner from "./Spinner.svelte";

  interface Props {
    open: boolean;
    initialProfile?: string;
    replacesConnectionId?: string;
    onclose: () => void;
    onsuccess?: () => void;
  }

  let {
    open,
    initialProfile = "",
    replacesConnectionId = "",
    onclose,
    onsuccess,
  }: Props = $props();

  let currentStep = $state(1);
  let availableProfiles = $state<profiles.Profile[]>([]);
  let detectedFolders = $state<service.AgentFolderSuggestion[]>([]);

  // Step 1 state
  let selectedProfile = $state("");
  let isNewProfile = $state(false);
  let newProfileName = $state("");
  let newProfileError = $state<string | null>(null);

  // Step 2 state
  let targetPath = $state("");
  let targetInfo = $state<service.PathInfo | null>(null);
  let targetEntries = $state<service.TargetEntry[]>([]);
  let selectedImportPaths = $state<string[]>([]);
  let startEmpty = $state(false);
  let isLoadingEntries = $state(false);

  // Step 3 state
  let draft = $state<service.ConnectionDraft | null>(null);
  let resolutions = $state<Record<string, "keep_repo" | "keep_local">>({});
  let isPreparingStep3 = $state(false);
  let isCommitting = $state(false);
  let errorMessage = $state<string | null>(null);

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

  // Connected profiles lookup
  const connectedProfilesMap = $derived.by(() => {
    const map: Record<string, string> = {};
    if (appState.config?.connections) {
      for (const c of appState.config.connections) {
        if (!replacesConnectionId || c.id !== replacesConnectionId) {
          map[c.profile.toLowerCase()] = c.targetPath;
        }
      }
    }
    return map;
  });

  $effect(() => {
    if (open) {
      errorMessage = null;
      loadInitialData();
    }
  });

  async function loadInitialData() {
    try {
      const [profs, suggestions] = await Promise.all([ListProfiles(), DetectAgentFolders()]);
      availableProfiles = profs || [];
      detectedFolders = suggestions || [];

      if (replacesConnectionId) {
        const existing = appState.config?.connections.find((c) => c.id === replacesConnectionId);
        if (existing) {
          selectedProfile = existing.profile;
          isNewProfile = false;
          targetPath = existing.targetPath;
          currentStep = 2;
          return;
        }
      }

      if (initialProfile) {
        selectedProfile = initialProfile;
        isNewProfile = false;
        // Check if suggestion matches
        const match = detectedFolders.find(
          (d) => d.suggestedProfile.toLowerCase() === initialProfile.toLowerCase()
        );
        if (match) {
          targetPath = match.path;
        }
        currentStep = 1;
      } else {
        selectedProfile = "";
        isNewProfile = false;
        newProfileName = "";
        targetPath = "";
        currentStep = 1;
      }
    } catch (err: any) {
      errorMessage = describeError(err);
    }
  }

  function validateNewProfileName(name: string): string | null {
    const trimmed = name.trim();
    if (!trimmed) return t("wizard.new_folder_name_label");
    if (trimmed.startsWith(".")) return t("errors.invalid_folder_name");
    if (trimmed.endsWith(".") || trimmed.endsWith(" ")) return t("errors.invalid_folder_name");
    if (INVALID_CHARS_REGEX.test(trimmed)) return t("errors.invalid_folder_name");
    if (RESERVED_WINDOWS_NAMES.includes(trimmed.toUpperCase())) {
      return t("errors.invalid_folder_name");
    }
    if (availableProfiles.some((p) => p.name.toLowerCase() === trimmed.toLowerCase())) {
      return t("errors.folder_exists");
    }
    return null;
  }

  function handleProfileSelect(name: string) {
    selectedProfile = name;
    isNewProfile = false;
    newProfileError = null;

    // Auto-suggest target path if available
    const match = detectedFolders.find(
      (d) => d.suggestedProfile.toLowerCase() === name.toLowerCase()
    );
    if (match && !targetPath) {
      targetPath = match.path;
    }
  }

  function handleSelectNewProfile() {
    isNewProfile = true;
    selectedProfile = newProfileName.trim();
    newProfileError = validateNewProfileName(newProfileName);
  }

  function handleNewProfileInput(e: Event) {
    const val = (e.target as HTMLInputElement).value;
    newProfileName = val;
    newProfileError = validateNewProfileName(val);
    if (!newProfileError) {
      selectedProfile = val.trim();
    }
  }

  // Step 2 target files loading
  const effectiveProfileFileCount = $derived.by(() => {
    if (isNewProfile) return 0;
    const found = availableProfiles.find(
      (p) => p.name.toLowerCase() === selectedProfile.toLowerCase()
    );
    return found ? found.fileCount : 0;
  });

  const shouldOfferTargetImport = $derived(isNewProfile || effectiveProfileFileCount === 0);

  $effect(() => {
    if (
      open &&
      currentStep === 2 &&
      shouldOfferTargetImport &&
      targetInfo?.exists &&
      targetInfo?.isDir
    ) {
      loadTargetEntries(targetPath);
    }
  });

  async function loadTargetEntries(dirPath: string) {
    if (!dirPath) return;
    isLoadingEntries = true;
    try {
      const entries = await ListTargetEntries(dirPath, 2);
      targetEntries = entries || [];

      // Default selection: known root config files (CLAUDE.md, AGENTS.md, GEMINI.md)
      const defaults: string[] = [];
      const knownRootFiles = ["claude.md", "agents.md", "gemini.md"];
      for (const e of targetEntries) {
        if (!e.isDir && !e.tooLarge) {
          const lower = e.relPath.toLowerCase();
          if (knownRootFiles.includes(lower)) {
            defaults.push(e.relPath);
          }
        }
      }
      selectedImportPaths = defaults;
      startEmpty = defaults.length === 0;
    } catch {
      targetEntries = [];
    } finally {
      isLoadingEntries = false;
    }
  }

  function toggleImportEntry(relPath: string) {
    if (selectedImportPaths.includes(relPath)) {
      selectedImportPaths = selectedImportPaths.filter((p) => p !== relPath);
    } else {
      selectedImportPaths = [...selectedImportPaths, relPath];
    }
  }

  const selectedSizeInfo = $derived.by(() => {
    let count = 0;
    let bytes = 0;
    for (const e of targetEntries) {
      if (selectedImportPaths.includes(e.relPath)) {
        count++;
        bytes += e.size;
      }
    }
    const kb = (bytes / 1024).toFixed(1);
    return { count, size: `${kb} KB` };
  });

  // Step navigation validations
  const canAdvanceStep1 = $derived.by(() => {
    if (isNewProfile) {
      return !newProfileError && newProfileName.trim().length > 0;
    }
    return Boolean(selectedProfile && !(selectedProfile.toLowerCase() in connectedProfilesMap));
  });

  const canAdvanceStep2 = $derived.by(() => {
    if (!targetInfo || !targetInfo.exists || !targetInfo.isDir || !targetInfo.writable) {
      return false;
    }
    if (targetInfo.isInsideRoot) return false;
    if (
      targetInfo.alreadyConnected &&
      targetPath !== connectedProfilesMap[selectedProfile.toLowerCase()]
    ) {
      return false;
    }
    if (shouldOfferTargetImport && !startEmpty && selectedImportPaths.length === 0) {
      return false;
    }
    return true;
  });

  async function goToStep2() {
    if (!canAdvanceStep1) return;
    errorMessage = null;

    // If new profile and name matches a suggestion, suggest target
    if (isNewProfile && !targetPath) {
      const match = detectedFolders.find(
        (d) => d.suggestedProfile.toLowerCase() === selectedProfile.toLowerCase()
      );
      if (match) {
        targetPath = match.path;
      }
    }
    currentStep = 2;
  }

  async function goToStep3() {
    if (!canAdvanceStep2) return;
    errorMessage = null;
    isPreparingStep3 = true;

    try {
      const req: service.ConnectionDraftRequest = {
        profile: selectedProfile,
        targetPath: targetPath,
        createProfile: isNewProfile,
        importPaths: shouldOfferTargetImport && !startEmpty ? selectedImportPaths : [],
        replacesConnectionId: replacesConnectionId || undefined,
      };

      const result = await BeginConnection(req);
      draft = result;

      // Default all differences to keep_repo
      const initialResolutions: Record<string, "keep_repo" | "keep_local"> = {};
      if (result.preview?.differences) {
        for (const diff of result.preview.differences) {
          initialResolutions[diff.relPath] = "keep_repo";
        }
      }
      resolutions = initialResolutions;
      currentStep = 3;
    } catch (err: any) {
      errorMessage = describeError(err);
    } finally {
      isPreparingStep3 = false;
    }
  }

  function applyAllDifferences(choice: "keep_repo" | "keep_local") {
    if (!draft?.preview?.differences) return;
    const updated = { ...resolutions };
    for (const diff of draft.preview.differences) {
      updated[diff.relPath] = choice;
    }
    resolutions = updated;
  }

  async function handleCommit() {
    if (!draft || isCommitting) return;
    isCommitting = true;
    errorMessage = null;

    try {
      const resolutionsList: orchestrator.ConflictResolution[] = Object.entries(resolutions).map(
        ([relPath, resolution]) => ({
          profile: selectedProfile,
          relPath,
          resolution,
        })
      );

      await CommitConnection(draft.id, resolutionsList);
      draft = null;
      onsuccess?.();
      onclose();
    } catch (err: any) {
      errorMessage = describeError(err);
    } finally {
      isCommitting = false;
    }
  }

  async function handleClose() {
    if (draft) {
      try {
        await CancelConnection(draft.id);
      } catch {
        // ignore draft cancellation error on close
      }
      draft = null;
    }
    onclose();
  }
</script>

<Modal
  {open}
  title={replacesConnectionId ? t("wizard.title_change_target") : t("wizard.title")}
  onclose={handleClose}
>
  <div class="wizard-container">
    <!-- Steps Indicator -->
    <div class="steps-indicator">
      <div class="step-item" class:active={currentStep === 1} class:done={currentStep > 1}>
        <div class="step-circle">
          {#if currentStep > 1}
            <Check size={14} />
          {:else}
            <span>1</span>
          {/if}
        </div>
        <span class="step-label">{t("wizard.step1_name")}</span>
      </div>
      <div class="step-connector" class:done={currentStep > 1}></div>

      <div class="step-item" class:active={currentStep === 2} class:done={currentStep > 2}>
        <div class="step-circle">
          {#if currentStep > 2}
            <Check size={14} />
          {:else}
            <span>2</span>
          {/if}
        </div>
        <span class="step-label">{t("wizard.step2_name")}</span>
      </div>
      <div class="step-connector" class:done={currentStep > 2}></div>

      <div class="step-item" class:active={currentStep === 3}>
        <div class="step-circle">
          <span>3</span>
        </div>
        <span class="step-label">{t("wizard.step3_name")}</span>
      </div>
    </div>

    {#if errorMessage}
      <div class="wizard-error">
        <AlertCircle size={16} />
        <span>{errorMessage}</span>
      </div>
    {/if}

    <!-- STEP 1: Repository Profile Selection -->
    {#if currentStep === 1}
      <div class="step-content">
        <p class="step-subtitle">{t("wizard.step1_subtitle")}</p>

        <div class="profiles-list">
          {#each availableProfiles as prof (prof.name)}
            {@const isConnected = Boolean(connectedProfilesMap[prof.name.toLowerCase()])}
            {@const connectedTarget = connectedProfilesMap[prof.name.toLowerCase()]}
            <label
              class="profile-radio-card"
              class:selected={selectedProfile === prof.name && !isNewProfile}
              class:disabled={isConnected}
            >
              <input
                type="radio"
                name="wizardProfile"
                disabled={isConnected}
                checked={selectedProfile === prof.name && !isNewProfile}
                onchange={() => handleProfileSelect(prof.name)}
              />
              <div class="profile-card-content">
                <div class="profile-card-header">
                  <Folder size={18} class="profile-icon" />
                  <span class="profile-name">{prof.name}</span>
                  {#if isConnected}
                    <span class="profile-badge-connected">
                      {t("wizard.already_connected_to", { target: connectedTarget || "" })}
                    </span>
                  {:else}
                    <span class="profile-file-count">
                      {t(`wizard.files_count_${prof.fileCount === 1 ? "one" : "other"}`, {
                        count: prof.fileCount,
                      })}
                    </span>
                  {/if}
                </div>
              </div>
            </label>
          {/each}

          <!-- Option to create new folder in repository -->
          <label class="profile-radio-card new-folder-card" class:selected={isNewProfile}>
            <input
              type="radio"
              name="wizardProfile"
              checked={isNewProfile}
              onchange={handleSelectNewProfile}
            />
            <div class="profile-card-content">
              <div class="profile-card-header">
                <FolderPlus size={18} class="profile-icon" />
                <span class="profile-name">{t("wizard.create_new_folder")}</span>
              </div>
              {#if isNewProfile}
                <div class="new-folder-input-box">
                  <label for="new-folder-input" class="new-folder-label">
                    {t("wizard.new_folder_name_label")}
                  </label>
                  <input
                    id="new-folder-input"
                    class="text-input"
                    type="text"
                    value={newProfileName}
                    placeholder={t("wizard.new_folder_name_placeholder")}
                    oninput={handleNewProfileInput}
                  />
                  {#if newProfileError && newProfileName.length > 0}
                    <span class="field-error">{newProfileError}</span>
                  {/if}
                </div>
              {/if}
            </div>
          </label>
        </div>

        <div class="wizard-actions">
          <Button variant="secondary" onclick={handleClose}>
            {t("common.cancel")}
          </Button>
          <Button variant="primary" disabled={!canAdvanceStep1} onclick={goToStep2}>
            <span>{t("wizard.btn_next")}</span>
            <ArrowRight size={16} />
          </Button>
        </div>
      </div>
    {/if}

    <!-- STEP 2: Target Folder & Initial Files Selection -->
    {#if currentStep === 2}
      <div class="step-content">
        <p class="step-subtitle">
          {t("wizard.step2_subtitle")} ({selectedProfile})
        </p>

        <PathField
          label={t("wizard.target_folder_label")}
          value={targetPath}
          mode="target"
          allowCreate={true}
          suggestions={detectedFolders}
          onchange={(val, info) => {
            targetPath = val;
            targetInfo = info;
          }}
        />

        {#if shouldOfferTargetImport && targetInfo?.exists && targetInfo?.isDir}
          <div class="import-section">
            <h4 class="import-title">{t("wizard.what_to_sync_title")}</h4>

            {#if isLoadingEntries}
              <div class="import-loading">
                <Spinner size="sm" />
                <span>{t("common.loading")}</span>
              </div>
            {:else if targetEntries.length > 0}
              <div class="target-entries-list">
                {#each targetEntries as entry (entry.relPath)}
                  <label
                    class="target-entry-row"
                    class:disabled={entry.tooLarge}
                    title={entry.tooLarge
                      ? entry.reason === "cache_dir"
                        ? t("wizard.cache_folder_reason")
                        : t("wizard.too_large_reason")
                      : ""}
                  >
                    <input
                      type="checkbox"
                      disabled={entry.tooLarge}
                      checked={selectedImportPaths.includes(entry.relPath)}
                      onchange={() => toggleImportEntry(entry.relPath)}
                    />
                    <span class="entry-path">{entry.relPath}</span>
                    {#if entry.tooLarge}
                      <span class="entry-badge-too-large">
                        {entry.reason === "cache_dir"
                          ? t("wizard.cache_folder_reason")
                          : t("wizard.too_large_reason")}
                      </span>
                    {:else if !entry.isDir}
                      <span class="entry-size">{(entry.size / 1024).toFixed(1)} KB</span>
                    {/if}
                  </label>
                {/each}
              </div>

              <div class="import-summary-row">
                <span class="summary-text">
                  {t("wizard.selection_summary", {
                    count: selectedSizeInfo.count,
                    size: selectedSizeInfo.size,
                  })}
                </span>
                <label class="start-empty-checkbox">
                  <input
                    type="checkbox"
                    checked={startEmpty}
                    onchange={(e) => {
                      startEmpty = (e.target as HTMLInputElement).checked;
                      if (startEmpty) selectedImportPaths = [];
                    }}
                  />
                  <span>{t("wizard.start_empty")}</span>
                </label>
              </div>
            {/if}
          </div>
        {/if}

        <div class="wizard-actions">
          {#if !replacesConnectionId}
            <Button variant="secondary" onclick={() => (currentStep = 1)}>
              <ArrowLeft size={16} />
              <span>{t("wizard.btn_back")}</span>
            </Button>
          {:else}
            <Button variant="secondary" onclick={handleClose}>
              {t("common.cancel")}
            </Button>
          {/if}

          <Button
            variant="primary"
            disabled={!canAdvanceStep2 || isPreparingStep3}
            loading={isPreparingStep3}
            onclick={goToStep3}
          >
            <span>{t("wizard.btn_next")}</span>
            <ArrowRight size={16} />
          </Button>
        </div>
      </div>
    {/if}

    <!-- STEP 3: Review & Commit -->
    {#if currentStep === 3 && draft}
      <div class="step-content">
        <p class="step-subtitle">{t("wizard.step3_subtitle")}</p>

        <div class="review-panels">
          <!-- Files to import into repository -->
          {#if draft.preview?.toImport && draft.preview.toImport.length > 0}
            <div class="review-panel">
              <h4 class="panel-title">{t("wizard.to_import")}</h4>
              <ul class="file-list">
                {#each draft.preview.toImport as file (file)}
                  <li class="file-item"><code>{file}</code></li>
                {/each}
              </ul>
            </div>
          {/if}

          <!-- Files to create in target -->
          {#if draft.preview?.toCreateInTarget && draft.preview.toCreateInTarget.length > 0}
            <div class="review-panel">
              <h4 class="panel-title">{t("wizard.to_create_in_target")}</h4>
              <ul class="file-list">
                {#each draft.preview.toCreateInTarget as file (file)}
                  <li class="file-item"><code>{file}</code></li>
                {/each}
              </ul>
            </div>
          {/if}

          <!-- Files to overwrite -->
          {#if draft.preview?.toOverwrite && draft.preview.toOverwrite.length > 0}
            <div class="review-panel">
              <h4 class="panel-title">{t("wizard.to_overwrite")}</h4>
              <ul class="file-list">
                {#each draft.preview.toOverwrite as file (file)}
                  <li class="file-item"><code>{file}</code></li>
                {/each}
              </ul>
            </div>
          {/if}

          <!-- Differences requiring decision -->
          {#if draft.preview?.differences && draft.preview.differences.length > 0}
            <div class="review-panel diff-panel">
              <div class="diff-header">
                <div>
                  <h4 class="panel-title">{t("wizard.differences_title")}</h4>
                  <p class="diff-desc">{t("wizard.differences_desc")}</p>
                </div>
                <div class="diff-bulk-actions">
                  <Button
                    variant="ghost"
                    size="sm"
                    onclick={() => applyAllDifferences("keep_repo")}
                  >
                    {t("wizard.apply_all_repo")}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onclick={() => applyAllDifferences("keep_local")}
                  >
                    {t("wizard.apply_all_local")}
                  </Button>
                </div>
              </div>

              <div class="diff-list">
                {#each draft.preview.differences as diff (diff.relPath)}
                  <div class="diff-item">
                    <code class="diff-relpath">{diff.relPath}</code>
                    <SegmentedControl
                      ariaLabel={diff.relPath}
                      options={[
                        { value: "keep_repo", label: t("wizard.choice_repo") },
                        { value: "keep_local", label: t("wizard.choice_local") },
                      ]}
                      value={resolutions[diff.relPath] || "keep_repo"}
                      onchange={(val) => {
                        resolutions = {
                          ...resolutions,
                          [diff.relPath]: val as "keep_repo" | "keep_local",
                        };
                      }}
                    />
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        </div>

        {#if (draft.preview?.toImport && draft.preview.toImport.length > 0) || Object.values(resolutions).includes("keep_local")}
          <div class="sync-reminder">
            <span>ℹ️</span>
            <span>{t("wizard.sync_reminder")}</span>
          </div>
        {/if}

        <div class="wizard-actions">
          <Button variant="secondary" onclick={() => (currentStep = 2)}>
            <ArrowLeft size={16} />
            <span>{t("wizard.btn_back")}</span>
          </Button>

          <Button
            variant="primary"
            disabled={isCommitting}
            loading={isCommitting}
            onclick={handleCommit}
          >
            <span>{t("wizard.btn_connect")}</span>
            <Check size={16} />
          </Button>
        </div>
      </div>
    {/if}
  </div>
</Modal>

<style>
  .wizard-container {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .steps-indicator {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-2) 0 var(--space-4);
    border-bottom: 1px solid var(--border-subtle);
  }

  .step-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    opacity: 0.5;
    transition: opacity var(--duration-fast) var(--ease-standard);
  }

  .step-item.active,
  .step-item.done {
    opacity: 1;
  }

  .step-circle {
    width: 24px;
    height: 24px;
    border-radius: var(--radius-sm);
    background-color: var(--surface-sunken);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: var(--text-xs);
    font-weight: var(--weight-bold);
    color: var(--text-muted);
  }

  .step-item.active .step-circle {
    background-color: var(--accent);
    color: white;
  }

  .step-item.done .step-circle {
    background-color: var(--success);
    color: white;
  }

  .step-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--ink);
  }

  .step-connector {
    flex: 1;
    max-width: 48px;
    height: 2px;
    background-color: var(--border-subtle);
    margin: 0 var(--space-2);
  }

  .step-connector.done {
    background-color: var(--success);
  }

  .step-content {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .step-subtitle {
    font-size: var(--text-sm);
    color: var(--text-muted);
    margin: 0;
  }

  .wizard-error {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--danger-bg);
    color: var(--danger);
    font-size: var(--text-sm);
  }

  .profiles-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    max-height: 280px;
    overflow-y: auto;
    padding-right: var(--space-1);
  }

  .profile-radio-card {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    padding: var(--space-3);
    border-radius: var(--radius-lg);
    border: 1px solid var(--border-subtle);
    background-color: var(--surface);
    cursor: pointer;
    transition: all var(--duration-fast) var(--ease-standard);
  }

  .profile-radio-card:hover:not(.disabled) {
    border-color: var(--border);
    background-color: var(--surface-hover);
  }

  .profile-radio-card.selected {
    border-color: var(--accent);
    background-color: var(--surface-sunken);
    box-shadow: 0 0 0 1px var(--accent);
  }

  .profile-radio-card.disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .profile-radio-card input[type="radio"] {
    margin-top: 3px;
  }

  .profile-card-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .profile-card-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  :global(.profile-icon) {
    color: var(--text-muted);
  }

  .profile-name {
    font-weight: var(--weight-semibold);
    font-size: var(--text-sm);
    color: var(--ink);
  }

  .profile-file-count {
    margin-left: auto;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .profile-badge-connected {
    margin-left: auto;
    font-size: var(--text-xs);
    color: var(--text-muted);
    background-color: var(--surface-sunken);
    padding: 2px var(--space-2);
    border-radius: var(--radius-xs);
  }

  .new-folder-input-box {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    margin-top: var(--space-2);
  }

  .new-folder-label {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .text-input {
    height: 32px;
    padding: 0 var(--space-2);
    border-radius: var(--radius-md);
    border: none;
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--ink);
    font-family: var(--font-sans);
    font-size: var(--text-sm);
  }

  .field-error {
    color: var(--danger);
    font-size: var(--text-xs);
  }

  .import-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    border-top: 1px solid var(--border-subtle);
    padding-top: var(--space-3);
  }

  .import-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    margin: 0;
    color: var(--ink);
  }

  .import-loading {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .target-entries-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    max-height: 180px;
    overflow-y: auto;
    background-color: var(--surface-sunken);
    padding: var(--space-2);
    border-radius: var(--radius-md);
  }

  .target-entry-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-xs);
    cursor: pointer;
    padding: 2px var(--space-1);
    border-radius: var(--radius-sm);
  }

  .target-entry-row:hover:not(.disabled) {
    background-color: var(--surface);
  }

  .target-entry-row.disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .entry-path {
    font-family: var(--font-mono);
    color: var(--ink);
    flex: 1;
  }

  .entry-size {
    color: var(--text-muted);
  }

  .entry-badge-too-large {
    color: var(--warning);
    font-size: 10px;
  }

  .import-summary-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .start-empty-checkbox {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    cursor: pointer;
  }

  .review-panels {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    max-height: 320px;
    overflow-y: auto;
  }

  .review-panel {
    background-color: var(--surface-sunken);
    border-radius: var(--radius-lg);
    padding: var(--space-3);
    box-shadow: var(--sunken);
  }

  .panel-title {
    font-size: var(--text-xs);
    font-weight: var(--weight-bold);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    margin: 0 0 var(--space-2) 0;
  }

  .file-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .file-item {
    font-size: var(--text-xs);
    color: var(--ink);
  }

  .diff-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: var(--space-2);
    margin-bottom: var(--space-2);
  }

  .diff-desc {
    font-size: var(--text-xs);
    color: var(--text-muted);
    margin: 0;
  }

  .diff-bulk-actions {
    display: flex;
    gap: var(--space-1);
  }

  .diff-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .diff-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-2);
    background-color: var(--surface);
    border-radius: var(--radius-md);
  }

  .diff-relpath {
    font-size: var(--text-xs);
    color: var(--ink);
  }

  .sync-reminder {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background-color: var(--surface-sunken);
    border-left: 3px solid var(--accent);
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .wizard-actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: var(--space-2);
    padding-top: var(--space-3);
    border-top: 1px solid var(--border-subtle);
  }
</style>
