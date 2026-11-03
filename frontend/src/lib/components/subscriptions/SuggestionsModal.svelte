<script lang="ts">
  import { untrack } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import { ArrowRight, FolderOpen, TriangleAlert } from "@lucide/svelte";
  import type { service } from "../../../../wailsjs/go/models";
  import { api } from "../../api";
  import { t } from "../../i18n";
  import { appState } from "../../stores/appState.svelte";
  import Button from "../Button.svelte";
  import Modal from "../Modal.svelte";
  import Spinner from "../Spinner.svelte";
  import { reportError } from "../remote/reportError";
  import {
    SUGGESTION_APPLIED,
    SUGGESTION_SKIPPED,
    detectedFolders,
    failureText,
    initialSelection,
    invalidReasonText,
    isSelectable,
    keepsExistingFile,
    mappingsFor,
    missingFolderAliases,
    type Folders,
  } from "./suggestions";

  interface Props {
    open: boolean;
    subscription: { id: string; name: string } | null;
    onclose: () => void;
  }

  let { open, subscription, onclose }: Props = $props();

  let items = $state<service.SuggestionInfo[]>([]);
  let loading = $state(false);
  let applying = $state(false);
  let folders = $state<Folders>({});
  let rowErrors = $state<Record<string, string>>({});
  let detected = $state<Folders>({});
  const selected = new SvelteSet<string>();
  let loadRequest = 0;

  const selectedCount = $derived(selected.size);
  const missingAliases = $derived(missingFolderAliases(items, selected, folders));
  const canApply = $derived(
    !loading && !applying && selectedCount > 0 && missingAliases.length === 0
  );
  const applyLabel = $derived(
    t(`subscriptions.suggestions.apply_${selectedCount === 1 ? "one" : "other"}`, {
      count: selectedCount,
    })
  );

  $effect(() => {
    if (!open || !subscription) return;
    const id = subscription.id;
    untrack(() => {
      rowErrors = {};
      folders = {};
      selected.clear();
      void load(id, true);
    });
  });

  async function load(id: string, resetSelection: boolean) {
    const request = ++loadRequest;
    loading = true;
    try {
      const result = (await api.listSubscriptionSuggestions(id)) ?? [];
      if (request !== loadRequest) return;
      items = result;
      detected = detectedFolders(result);
      folders = { ...detected, ...folders };
      if (resetSelection) {
        selected.clear();
        for (const itemId of initialSelection(result)) selected.add(itemId);
      } else {
        for (const itemId of [...selected]) {
          const item = result.find((candidate) => candidate.id === itemId);
          if (!item || !isSelectable(item)) selected.delete(itemId);
        }
      }
    } catch (err) {
      if (request === loadRequest) {
        items = [];
        reportError(err);
      }
    } finally {
      if (request === loadRequest) loading = false;
    }
  }

  function handleClose() {
    if (applying) return;
    onclose();
  }

  function toggle(item: service.SuggestionInfo) {
    if (!isSelectable(item) || applying) return;
    if (selected.has(item.id)) selected.delete(item.id);
    else selected.add(item.id);
  }

  async function chooseFolder(alias: string) {
    try {
      const folder = await api.selectFolder(t("subscriptions.suggestions.folder_title", { alias }));
      if (folder) folders = { ...folders, [alias]: folder };
    } catch (err) {
      reportError(err);
    }
  }

  function folderHint(alias: string): string {
    if (!folders[alias]) return t("subscriptions.suggestions.folder_needed", { alias });
    if (folders[alias] === detected[alias]) {
      return t("subscriptions.suggestions.folder_found", { alias });
    }
    return t("subscriptions.suggestions.folder_chosen", { alias });
  }

  async function handleApply() {
    if (!canApply || !subscription) return;
    applying = true;
    rowErrors = {};
    const id = subscription.id;
    const ids = items.filter((item) => selected.has(item.id)).map((item) => item.id);
    try {
      const outcome = await api.applySubscriptionSuggestions(
        id,
        ids,
        mappingsFor(items, selected, folders)
      );
      const results = outcome.results ?? [];
      const errors: Record<string, string> = {};
      let collisions = 0;
      for (const result of results) {
        collisions += result.collisions ?? 0;
        if (result.status !== SUGGESTION_APPLIED && result.status !== SUGGESTION_SKIPPED) {
          errors[result.id] = failureText(result);
        }
      }
      rowErrors = errors;
      const created = outcome.applied ?? 0;
      if (created > 0) {
        appState.addToast(
          "success",
          Object.keys(errors).length > 0
            ? t("subscriptions.suggestions.applied_partial", {
                applied: created,
                total: ids.length,
              })
            : t(`subscriptions.suggestions.applied_${created === 1 ? "one" : "other"}`, {
                count: created,
              })
        );
        if (collisions > 0) appState.addWarning(t("subscriptions.suggestions.collisions_warning"));
        appState.afterRuleChange();
      }
      if (Object.keys(errors).length === 0) {
        onclose();
        return;
      }
      if (created === 0) appState.addWarning(t("subscriptions.suggestions.nothing_applied"));
      await load(id, false);
    } catch (err) {
      reportError(err);
    } finally {
      applying = false;
    }
  }
</script>

<Modal
  {open}
  title={t("subscriptions.suggestions.title_for", { name: subscription?.name ?? "" })}
  onclose={handleClose}
  size="lg"
>
  <div class="suggestions">
    <p class="intro">{t("subscriptions.suggestions.intro")}</p>

    {#if loading && items.length === 0}
      <div class="status" role="status">
        <Spinner size="sm" />
        <span>{t("subscriptions.suggestions.loading")}</span>
      </div>
    {:else if items.length === 0}
      <p class="status">{t("subscriptions.suggestions.empty")}</p>
    {:else}
      <ul class="list" aria-label={t("subscriptions.suggestions.list_label")}>
        {#each items as item (item.id)}
          {@const selectable = isSelectable(item)}
          <li class="row" class:row-muted={!selectable}>
            <label class="row-head">
              <input
                type="checkbox"
                class="check"
                checked={selected.has(item.id)}
                disabled={!selectable || applying}
                onchange={() => toggle(item)}
              />
              <span class="row-name" title={item.name}>{item.name}</span>
              {#if item.applied}
                <span class="tag">{t("subscriptions.suggestions.applied_tag")}</span>
              {:else if !item.valid}
                <span class="tag tag-warn">{t("subscriptions.suggestions.invalid_tag")}</span>
              {/if}
            </label>

            {#if item.description}
              <p class="description">{item.description}</p>
            {/if}
            {#if !item.valid}
              <p class="invalid">{invalidReasonText(item.invalidReason)}</p>
            {/if}

            <div class="flow">
              <span class="flow-label">{t("subscriptions.suggestions.source_label")}</span>
              <code class="path" title={item.source}>{item.source}</code>
            </div>

            {#each item.destinations ?? [] as dest, index (index)}
              <div class="dest">
                <div class="flow">
                  <span class="flow-label">{t("subscriptions.suggestions.target_label")}</span>
                  <ArrowRight size={12} aria-hidden="true" />
                  <code class="alias">{dest.location}</code>
                  <code class="path" title={dest.path}>{dest.path}</code>
                  {#if dest.applied}
                    <span class="tag">{t("subscriptions.suggestions.applied_tag")}</span>
                  {/if}
                </div>
                {#if item.valid && dest.mapped}
                  <code class="resolved" title={dest.targetPath}>{dest.targetPath}</code>
                  {#if dest.targetExists && !dest.applied}
                    <p class="note">
                      {keepsExistingFile(dest)
                        ? t("subscriptions.suggestions.keeps_existing")
                        : t("subscriptions.suggestions.replaces_existing")}
                    </p>
                  {/if}
                {/if}
                {#if item.valid && !dest.mapped && !dest.applied}
                  <div class="folder" class:folder-missing={!folders[dest.location]}>
                    <span class="folder-hint">{folderHint(dest.location)}</span>
                    {#if folders[dest.location]}
                      <code class="resolved" title={folders[dest.location]}>
                        {folders[dest.location]}
                      </code>
                    {/if}
                    <Button
                      size="sm"
                      variant="secondary"
                      disabled={applying}
                      onclick={() => chooseFolder(dest.location)}
                    >
                      <FolderOpen size={14} />
                      <span
                        >{folders[dest.location]
                          ? t("subscriptions.suggestions.change_folder")
                          : t("subscriptions.suggestions.choose_folder")}</span
                      >
                    </Button>
                  </div>
                {/if}
                {#if item.valid && dest.mode}
                  <p class="mode">
                    {keepsExistingFile(dest)
                      ? t("subscriptions.suggestions.mode_create_only")
                      : t("subscriptions.suggestions.mode_one_way")}
                  </p>
                {/if}
              </div>
            {/each}

            {#if rowErrors[item.id]}
              <p class="row-error" role="alert">
                <TriangleAlert size={14} aria-hidden="true" />
                <span>{rowErrors[item.id]}</span>
              </p>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}

    <p class="note-line">{t("subscriptions.suggestions.note")}</p>
    {#if missingAliases.length > 0}
      <p class="folders-missing" role="status">{t("subscriptions.suggestions.folders_missing")}</p>
    {/if}

    <div class="actions">
      <Button variant="secondary" disabled={applying} onclick={handleClose}>
        {t("common.close")}
      </Button>
      <Button variant="primary" loading={applying} disabled={!canApply} onclick={handleApply}>
        {applyLabel}
      </Button>
    </div>
  </div>
</Modal>

<style>
  .suggestions {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    min-width: 0;
  }

  .intro,
  .note-line {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
    line-height: 1.55;
  }

  .note-line {
    font-size: var(--text-xs);
  }

  .status {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }

  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .row {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    min-width: 0;
  }

  .row-muted {
    opacity: 0.8;
  }

  .row-head {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
    cursor: pointer;
  }

  .row-muted .row-head {
    cursor: default;
  }

  .check {
    flex-shrink: 0;
    width: 16px;
    height: 16px;
    margin: 0;
    accent-color: var(--accent);
  }

  .check:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .row-name {
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .tag {
    flex-shrink: 0;
    padding: 1px var(--space-2);
    border-radius: var(--radius-sm);
    background-color: var(--surface);
    font-size: 11px;
    color: var(--text-muted);
  }

  .tag-warn {
    color: var(--dot-conflict);
  }

  .description,
  .invalid,
  .note,
  .mode {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.5;
  }

  .invalid {
    color: var(--dot-conflict);
  }

  .flow {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .flow :global(svg) {
    flex-shrink: 0;
  }

  .flow-label {
    flex-shrink: 0;
    min-width: 36px;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-subtle);
  }

  .path,
  .alias,
  .resolved {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .alias {
    flex-shrink: 0;
    padding: 1px var(--space-2);
    border-radius: var(--radius-sm);
    background-color: var(--surface);
  }

  .resolved {
    display: block;
    color: var(--text-muted);
    padding-left: calc(36px + var(--space-2));
  }

  .dest {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    min-width: 0;
  }

  .mode,
  .dest .note {
    padding-left: calc(36px + var(--space-2));
  }

  .folder {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    margin-left: calc(36px + var(--space-2));
    border-radius: var(--radius-sm);
    background-color: var(--surface);
    min-width: 0;
  }

  .folder .resolved {
    flex: 1;
    padding-left: 0;
    min-width: 0;
  }

  .folder-missing {
    box-shadow: inset 0 0 0 1px var(--dot-conflict);
  }

  .folder-hint {
    font-size: var(--text-xs);
    color: var(--ink);
  }

  .row-error {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    font-size: var(--text-xs);
    color: var(--dot-conflict);
  }

  .row-error :global(svg) {
    flex-shrink: 0;
  }

  .folders-missing {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--dot-conflict);
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-3);
  }
</style>
