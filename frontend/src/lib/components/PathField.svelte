<script lang="ts">
  import { describeError } from "../i18n/errors";
  import { FolderOpen } from "@lucide/svelte";
  import type { service } from "../../../wailsjs/go/models";
  import { InspectPath, SelectFolder, CreateFolder } from "../../../wailsjs/go/main/App";
  import { t } from "../i18n";
  import Button from "./Button.svelte";
  import Spinner from "./Spinner.svelte";

  interface Props {
    label?: string;
    value?: string;
    mode?: "root" | "target" | "backup";
    allowCreate?: boolean;
    suggestions?: service.AgentFolderSuggestion[];
    placeholder?: string;
    disabled?: boolean;
    onchange?: (val: string, info: service.PathInfo | null) => void;
  }

  let {
    label,
    value = "",
    mode = "target",
    allowCreate = false,
    suggestions = [],
    placeholder = "",
    disabled = false,
    onchange,
  }: Props = $props();

  let inputValue = $state("");
  let prevValue = "";
  let pathInfo = $state<service.PathInfo | null>(null);
  let isInspecting = $state(false);
  let isCreating = $state(false);
  let createError = $state<string | null>(null);
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  const idSuffix = Math.random().toString(36).slice(2, 8);
  const inputId = `path-input-${idSuffix}`;
  const statusId = `path-status-${idSuffix}`;

  $effect(() => {
    if (value !== prevValue) {
      prevValue = value;
      inputValue = value;
      inspectNow(value);
    }
  });

  async function inspectNow(target: string) {
    if (debounceTimer) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
    const trimmed = target.trim();
    if (!trimmed) {
      pathInfo = null;
      isInspecting = false;
      onchange?.("", null);
      return;
    }

    isInspecting = true;
    createError = null;
    try {
      const info = await InspectPath(trimmed);
      pathInfo = info;
      onchange?.(trimmed, info);
    } catch {
      pathInfo = null;
      onchange?.(trimmed, null);
    } finally {
      isInspecting = false;
    }
  }

  function handleInput(e: Event) {
    const newVal = (e.target as HTMLInputElement).value;
    inputValue = newVal;
    createError = null;
    if (debounceTimer) {
      clearTimeout(debounceTimer);
    }
    debounceTimer = setTimeout(() => {
      inspectNow(newVal);
    }, 300);
  }

  async function handleBrowse() {
    if (disabled) return;
    createError = null;
    try {
      const selected = await SelectFolder(t("path_field.browse_dialog_title"), inputValue.trim());
      if (selected && selected.trim()) {
        inputValue = selected.trim();
        await inspectNow(selected.trim());
      }
    } catch (err: unknown) {
      createError = t("path_field.browse_failed", {
        error: describeError(err),
      });
    }
  }

  async function handleCreate() {
    if (disabled || isCreating || !inputValue.trim()) return;
    isCreating = true;
    createError = null;
    try {
      const created = await CreateFolder(inputValue.trim());
      if (created) {
        inputValue = created;
        await inspectNow(created);
      }
    } catch (err: any) {
      createError = t("path_field.failed_create", { error: describeError(err) });
    } finally {
      isCreating = false;
    }
  }

  function handleSuggestionClick(sug: service.AgentFolderSuggestion) {
    if (disabled) return;
    inputValue = sug.path;
    inspectNow(sug.path);
  }

  function handleKeyDown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && (e.key === "o" || e.key === "O")) {
      e.preventDefault();
      handleBrowse();
    } else if (e.key === "Enter") {
      e.preventDefault();
      inspectNow(inputValue);
    }
  }

  // Derive status presentation
  const statusState = $derived.by(() => {
    if (!inputValue.trim()) {
      return { type: "empty", text: t("path_field.empty_path"), color: "muted" };
    }
    if (isInspecting) {
      return { type: "inspecting", text: t("common.loading"), color: "muted" };
    }
    if (!pathInfo) {
      return { type: "unknown", text: "", color: "muted" };
    }

    if (!pathInfo.exists) {
      return {
        type: "not_exists",
        text: t("path_field.does_not_exist"),
        color: "muted",
        canCreate: allowCreate,
      };
    }

    if (!pathInfo.isDir) {
      return { type: "error", text: t("path_field.not_a_dir"), color: "danger" };
    }

    if (!pathInfo.writable) {
      return { type: "error", text: t("path_field.not_writable"), color: "danger" };
    }

    if (mode === "target") {
      if (pathInfo.isInsideRoot) {
        return { type: "error", text: t("path_field.inside_root"), color: "danger" };
      }
      if (pathInfo.alreadyConnected) {
        return { type: "warning", text: t("path_field.already_connected"), color: "warning" };
      }
    }

    if (mode === "backup" && pathInfo.isInsideRoot) {
      return { type: "error", text: t("path_field.inside_root"), color: "danger" };
    }

    return { type: "valid", text: t("path_field.valid_folder"), color: "success" };
  });
</script>

<div class="path-field-group">
  {#if label}
    <label class="path-field-label" for={inputId}>{label}</label>
  {/if}

  <div class="input-row">
    <input
      id={inputId}
      class="path-input"
      type="text"
      value={inputValue}
      {placeholder}
      {disabled}
      aria-describedby={statusId}
      oninput={handleInput}
      onkeydown={handleKeyDown}
    />
    <Button
      variant="secondary"
      size="md"
      {disabled}
      onclick={handleBrowse}
      ariaLabel={t("path_field.browse")}
    >
      <FolderOpen size={16} />
      <span>{t("path_field.browse")}</span>
    </Button>
  </div>

  {#if suggestions && suggestions.length > 0}
    <div class="suggestions-row">
      {#each suggestions as sug (sug.path)}
        <button
          type="button"
          class="suggestion-chip"
          {disabled}
          title={sug.path}
          onclick={() => handleSuggestionClick(sug)}
        >
          {sug.label}
        </button>
      {/each}
    </div>
  {/if}

  <div id={statusId} class="status-row" aria-live="polite">
    {#if statusState.text}
      <span class="status-dot dot-{statusState.color}"></span>
      <span class="status-text">{statusState.text}</span>
      {#if statusState.canCreate}
        <button
          type="button"
          class="create-folder-btn"
          disabled={disabled || isCreating}
          onclick={handleCreate}
        >
          {#if isCreating}
            <Spinner size="sm" />
            <span>{t("path_field.creating")}</span>
          {:else}
            <span>[{t("path_field.create_folder")}]</span>
          {/if}
        </button>
      {/if}
    {/if}
    {#if createError}
      <span class="create-error">{createError}</span>
    {/if}
  </div>
</div>

<style>
  .path-field-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    width: 100%;
  }

  .path-field-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .input-row {
    display: flex;
    gap: var(--space-2);
    align-items: center;
    width: 100%;
  }

  .path-input {
    flex: 1;
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
    transition: box-shadow var(--duration-fast) var(--ease-standard);
  }

  .path-input:focus-visible {
    box-shadow:
      var(--sunken),
      0 0 0 2px var(--focus-ring);
  }

  .path-input:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .suggestions-row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
    align-items: center;
  }

  .suggestion-chip {
    background-color: var(--surface);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-xs);
    padding: 2px var(--space-2);
    font-size: var(--text-xs);
    color: var(--ink);
    cursor: pointer;
    font-family: var(--font-sans);
    transition: all var(--duration-fast) var(--ease-standard);
  }

  .suggestion-chip:hover:not(:disabled) {
    background-color: var(--surface-sunken);
    border-color: var(--border);
  }

  .suggestion-chip:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .status-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-xs);
    min-height: 20px;
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: var(--radius-full);
    flex-shrink: 0;
  }

  .dot-success {
    background-color: var(--success);
  }

  .dot-muted {
    background-color: var(--text-muted);
  }

  .dot-danger {
    background-color: var(--danger);
  }

  .dot-warning {
    background-color: var(--warning);
  }

  .status-text {
    color: var(--text-muted);
  }

  .create-folder-btn {
    background: none;
    border: none;
    color: var(--accent);
    cursor: pointer;
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    padding: 0;
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    text-decoration: underline;
  }

  .create-folder-btn:hover:not(:disabled) {
    color: var(--accent-hover);
  }

  .create-folder-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .create-error {
    color: var(--danger);
    font-size: var(--text-xs);
  }
</style>
