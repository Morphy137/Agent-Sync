<script lang="ts">
  import { untrack } from "svelte";
  import { FolderOpen, History } from "@lucide/svelte";
  import type { service } from "../../../../wailsjs/go/models";
  import { api } from "../../api";
  import { t, formatDate } from "../../i18n";
  import { describeError } from "../../i18n/errors";
  import { appState } from "../../stores/appState.svelte";
  import Modal from "../Modal.svelte";
  import Button from "../Button.svelte";
  import Input from "../Input.svelte";
  import Spinner from "../Spinner.svelte";
  import {
    SEARCH_THRESHOLD,
    countKey,
    filterBackups,
    formatSize,
    groupBackups,
    ownerLabel,
    destinationLabel,
    reasonLabel,
  } from "./backups";

  interface Props {
    open: boolean;
    onclose: () => void;
  }

  let { open, onclose }: Props = $props();

  let backups = $state<service.BackupEntry[]>([]);
  let loading = $state(false);
  let loadError = $state("");
  let query = $state("");
  let selectedId = $state<string | null>(null);
  let selectedFiles = $state<string[]>([]);
  let confirming = $state(false);
  let restoring = $state(false);

  const showSearch = $derived(backups.length > SEARCH_THRESHOLD);
  const visible = $derived(filterBackups(backups, query));
  const groups = $derived(groupBackups(visible));
  const selected = $derived(backups.find((b) => b.id === selectedId) ?? null);
  const allSelected = $derived(selected !== null && selectedFiles.length === selected.files.length);

  $effect(() => {
    if (!open) return;
    untrack(() => {
      query = "";
      selectedId = null;
      selectedFiles = [];
      confirming = false;
      void load();
    });
  });

  async function load() {
    loading = true;
    loadError = "";
    try {
      backups = (await api.listBackups()) ?? [];
    } catch (err) {
      loadError = describeError(err);
    } finally {
      loading = false;
    }
  }

  function select(entry: service.BackupEntry) {
    selectedId = entry.id;
    selectedFiles = [...entry.files];
    confirming = false;
  }

  function toggleFile(file: string) {
    selectedFiles = selectedFiles.includes(file)
      ? selectedFiles.filter((f) => f !== file)
      : [...selectedFiles, file];
  }

  function toggleAll() {
    if (!selected) return;
    selectedFiles = allSelected ? [] : [...selected.files];
  }

  async function openFolder() {
    if (!selected) return;
    try {
      await api.openPath(selected.path);
    } catch (err) {
      appState.showError(err);
    }
  }

  async function restore() {
    if (!selected || restoring) return;
    const entry = selected;
    // An empty list means "everything" to the backend, so send the files only for a partial restore.
    const files = allSelected ? [] : [...selectedFiles];
    restoring = true;
    try {
      const result = await api.restoreBackup(entry.id, files);
      appState.addToast(
        "success",
        t(countKey("backups.restored_toast", result.restored.length), {
          count: result.restored.length,
          target: result.target,
        })
      );
      confirming = false;
      await appState.refreshLocalStatus();
      appState.bumpRules();
      await load();
    } catch (err) {
      appState.showError(err);
      confirming = false;
    } finally {
      restoring = false;
    }
  }
</script>

<Modal {open} title={t("backups.title")} {onclose} size="lg">
  {#if loading && backups.length === 0}
    <div class="center"><Spinner /></div>
  {:else if loadError}
    <p class="notice notice-error" role="alert">{loadError}</p>
  {:else if backups.length === 0}
    <div class="center empty">
      <History size={28} />
      <p class="empty-title">{t("backups.empty_title")}</p>
      <p class="muted">{t("backups.empty_hint")}</p>
    </div>
  {:else}
    <div class="layout">
      <section class="list-pane" aria-label={t("backups.list_label")}>
        {#if showSearch}
          <Input
            value={query}
            placeholder={t("backups.search_placeholder")}
            oninput={(value) => (query = value)}
          />
        {/if}
        <div class="list">
          {#each groups as group (group.key)}
            <div class="group">
              <div class="group-head">
                <span class="group-owner">{group.owner}</span>
                {#if group.destination}
                  <span class="group-destination">{group.destination}</span>
                {/if}
              </div>
              {#each group.entries as entry (entry.id)}
                <button
                  type="button"
                  class="row"
                  class:row-selected={entry.id === selectedId}
                  aria-pressed={entry.id === selectedId}
                  onclick={() => select(entry)}
                >
                  <span class="row-date">{formatDate(entry.createdAt)}</span>
                  <span class="row-meta">
                    {reasonLabel(entry.reason)} · {t(
                      countKey("backups.file_count", entry.fileCount),
                      {
                        count: entry.fileCount,
                      }
                    )} · {formatSize(entry.totalSize)}
                  </span>
                </button>
              {/each}
            </div>
          {:else}
            <p class="muted no-match">{t("backups.no_match")}</p>
          {/each}
        </div>
      </section>

      <section class="detail-pane" aria-live="polite">
        {#if !selected}
          <p class="muted detail-placeholder">{t("backups.select_hint")}</p>
        {:else}
          <header class="detail-head">
            <h3 class="detail-title">{ownerLabel(selected)}</h3>
            {#if destinationLabel(selected)}
              <p class="muted">{destinationLabel(selected)}</p>
            {/if}
            <p class="muted">
              {formatDate(selected.createdAt)} · {reasonLabel(selected.reason)}
            </p>
            {#if selected.targetPath}
              <p class="muted path" title={selected.targetPath}>{selected.targetPath}</p>
            {/if}
          </header>

          {#if !selected.restorable}
            <p class="notice" role="note">{t("backups.not_restorable")}</p>
          {/if}

          <label class="check select-all">
            <input
              type="checkbox"
              checked={allSelected}
              disabled={!selected.restorable}
              onchange={toggleAll}
            />
            <span>{t("backups.select_all")}</span>
          </label>
          <ul class="files">
            {#each selected.files as file (file)}
              <li>
                <label class="check">
                  <input
                    type="checkbox"
                    checked={selectedFiles.includes(file)}
                    disabled={!selected.restorable}
                    onchange={() => toggleFile(file)}
                  />
                  <span class="file-name" title={file}>{file}</span>
                </label>
              </li>
            {/each}
          </ul>

          {#if confirming}
            <div class="confirm" role="alert">
              <p class="confirm-text">
                {t(countKey("backups.confirm_message", selectedFiles.length), {
                  count: selectedFiles.length,
                  target: selected.targetPath,
                })}
              </p>
              <div class="actions">
                <Button
                  variant="secondary"
                  onclick={() => (confirming = false)}
                  disabled={restoring}
                >
                  {t("common.cancel")}
                </Button>
                <Button variant="primary" onclick={restore} loading={restoring}>
                  {t("backups.confirm_button")}
                </Button>
              </div>
            </div>
          {:else}
            <div class="actions">
              <Button variant="ghost" size="sm" onclick={openFolder}>
                <FolderOpen size={14} />
                {t("backups.open_folder")}
              </Button>
              <Button
                variant="primary"
                disabled={!selected.restorable || selectedFiles.length === 0}
                onclick={() => (confirming = true)}
              >
                {allSelected
                  ? t("backups.restore_all")
                  : t("backups.restore_selected", { count: selectedFiles.length })}
              </Button>
            </div>
          {/if}
        {/if}
      </section>
    </div>
  {/if}
</Modal>

<style>
  .layout {
    display: grid;
    grid-template-columns: minmax(240px, 300px) minmax(0, 1fr);
    gap: var(--space-5);
    height: 100%;
    min-height: 0;
  }

  .list-pane,
  .detail-pane {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    min-height: 0;
    min-width: 0;
  }

  .detail-pane {
    overflow-y: auto;
  }

  .list {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    overflow-y: auto;
    min-height: 0;
    padding-right: var(--space-1);
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .group-head {
    display: flex;
    flex-direction: column;
    padding: var(--space-1) var(--space-2);
  }

  .group-owner {
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .group-destination {
    font-size: var(--text-xs);
    color: var(--text-muted);
    font-family: var(--font-mono);
  }

  .row {
    display: flex;
    flex-direction: column;
    gap: 2px;
    text-align: left;
    width: 100%;
    padding: var(--space-2) var(--space-3);
    border: none;
    border-radius: var(--radius-md);
    background-color: transparent;
    color: var(--ink);
    font-family: var(--font-sans);
    cursor: pointer;
  }

  .row:hover {
    background-color: var(--surface-hover);
  }

  .row:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .row-selected,
  .row-selected:hover {
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .row-date {
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
  }

  .row-meta {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .detail-head {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .detail-title {
    margin: 0;
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .detail-head p {
    margin: 0;
  }

  .muted {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.5;
  }

  .path {
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }

  .notice {
    margin: 0;
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    font-size: var(--text-xs);
    color: var(--ink);
    line-height: 1.5;
  }

  .notice-error {
    color: var(--danger);
  }

  .files {
    list-style: none;
    margin: 0;
    padding: var(--space-1) var(--space-3);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-md);
    max-height: 220px;
    overflow-y: auto;
  }

  .check {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: 30px;
    font-size: var(--text-xs);
    color: var(--ink);
    cursor: pointer;
  }

  .check input {
    accent-color: var(--accent);
  }

  .select-all {
    font-weight: var(--weight-medium);
  }

  .file-name {
    font-family: var(--font-mono);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .confirm {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .confirm-text {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--ink);
    line-height: 1.5;
    overflow-wrap: anywhere;
  }

  .confirm .actions {
    justify-content: flex-end;
  }

  .center {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-8) 0;
    color: var(--text-muted);
  }

  .empty-title {
    margin: 0;
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .detail-placeholder,
  .no-match {
    padding: var(--space-4) 0;
    text-align: center;
  }
</style>
