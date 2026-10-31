<script lang="ts">
  import { t } from "../i18n";
  import { buildLineDiff } from "../diff/lineDiff";
  import type { DiffAction, FileDiff } from "../diff/types";
  import Modal from "./Modal.svelte";
  import Button from "./Button.svelte";
  import Spinner from "./Spinner.svelte";

  interface Props {
    open: boolean;
    title?: string;
    subtitle?: string;
    diff: FileDiff | null;
    loading?: boolean;
    actions?: DiffAction[];
    /** When set, the user can edit a merged version (prefilled with the left side) and save it. */
    onsaveEdited?: (content: string) => void;
    onclose: () => void;
  }

  let {
    open,
    title,
    subtitle = "",
    diff,
    loading = false,
    actions = [],
    onsaveEdited,
    onclose,
  }: Props = $props();

  let editing = $state(false);
  let editedContent = $state("");

  const summary = $derived(diff && !diff.binary ? buildLineDiff(diff.left, diff.right) : null);

  function labelFor(key: string): string {
    const translationKey = `diff.labels.${key}`;
    const translated = t(translationKey);
    return translated === translationKey ? key : translated;
  }

  function startEditing() {
    editedContent = diff?.left ?? "";
    editing = true;
  }

  function handleClose() {
    editing = false;
    onclose();
  }

  function handleSaveEdited() {
    onsaveEdited?.(editedContent);
    editing = false;
  }
</script>

<Modal {open} size="lg" title={title ?? t("diff.title")} onclose={handleClose}>
  <div class="diff-modal">
    {#if subtitle}
      <p class="diff-subtitle">{subtitle}</p>
    {/if}

    {#if loading || !diff}
      <div class="diff-state">
        <Spinner size="sm" />
        <span>{t("diff.loading")}</span>
      </div>
    {:else if diff.binary}
      <p class="diff-state">{t("diff.binary")}</p>
    {:else if editing}
      <label class="edit-label" for="diff-edit-area">{t("diff.edit_label")}</label>
      <textarea id="diff-edit-area" class="edit-area" bind:value={editedContent} spellcheck="false"
      ></textarea>
    {:else}
      {#if diff.transform}
        <p class="diff-notice">{t("diff.transform_notice")}</p>
      {/if}
      <div class="diff-meta">
        <span>
          {t("diff.legend", { left: labelFor(diff.leftLabel), right: labelFor(diff.rightLabel) })}
        </span>
        {#if summary && !summary.identical}
          <span>{t("diff.summary", { added: summary.added, removed: summary.removed })}</span>
        {/if}
      </div>
      {#if summary?.identical}
        <p class="diff-state">{t("diff.identical")}</p>
      {:else if summary}
        <div class="diff-table" role="table">
          {#each summary.rows as row, index (index)}
            <div class="diff-row row-{row.kind}" role="row">
              <span class="line-no" role="cell">{row.oldLine ?? ""}</span>
              <span class="line-no" role="cell">{row.newLine ?? ""}</span>
              <span class="marker" role="cell" aria-hidden="true">
                {row.kind === "added" ? "+" : row.kind === "removed" ? "-" : ""}
              </span>
              <span class="line-text" role="cell">{row.text}</span>
            </div>
          {/each}
        </div>
      {/if}
    {/if}

    <footer class="diff-actions">
      {#if editing}
        <Button variant="ghost" size="sm" onclick={() => (editing = false)}>
          {t("diff.edit_cancel")}
        </Button>
        <Button variant="primary" size="sm" onclick={handleSaveEdited}>
          {t("diff.edit_save")}
        </Button>
      {:else}
        {#if onsaveEdited && diff && !diff.binary}
          <Button variant="ghost" size="sm" onclick={startEditing}>{t("diff.edit_button")}</Button>
        {/if}
        {#each actions as action (action.label)}
          <Button
            variant={action.variant ?? "secondary"}
            size="sm"
            disabled={action.disabled}
            onclick={action.onclick}
          >
            {action.label}
          </Button>
        {/each}
      {/if}
    </footer>
  </div>
</Modal>

<style>
  .diff-modal {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .diff-subtitle {
    margin: 0;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-muted);
    word-break: break-all;
  }

  .diff-state {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    padding: var(--space-4);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }

  .diff-notice {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .diff-meta {
    display: flex;
    justify-content: space-between;
    gap: var(--space-3);
    font-size: var(--text-xs);
    color: var(--text-subtle);
  }

  .diff-table {
    max-height: 55vh;
    overflow: auto;
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-md);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    padding: var(--space-2) 0;
  }

  .diff-row {
    display: grid;
    grid-template-columns: 40px 40px 16px 1fr;
    align-items: baseline;
    min-height: 20px;
  }

  .row-added {
    background-color: color-mix(in srgb, var(--dot-ok) 16%, transparent);
  }

  .row-removed {
    background-color: color-mix(in srgb, var(--dot-conflict) 16%, transparent);
  }

  .line-no {
    text-align: right;
    padding-right: var(--space-2);
    color: var(--text-subtle);
    user-select: none;
  }

  .marker {
    color: var(--text-muted);
    user-select: none;
  }

  .line-text {
    white-space: pre-wrap;
    word-break: break-word;
    color: var(--ink);
    padding-right: var(--space-3);
  }

  .edit-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .edit-area {
    min-height: 40vh;
    resize: vertical;
    padding: var(--space-3);
    border: none;
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--ink);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    outline: none;
  }

  .edit-area:focus-visible {
    box-shadow: var(--focus-ring);
  }

  .diff-actions {
    display: flex;
    justify-content: flex-end;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
</style>
