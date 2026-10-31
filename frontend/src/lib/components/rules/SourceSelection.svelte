<script lang="ts">
  import { FileText, Folder, LayoutGrid, List, X } from "@lucide/svelte";
  import { t } from "../../i18n";
  import IconButton from "../IconButton.svelte";
  import { baseName, fileExtension } from "./ruleUi";

  interface Props {
    /** Folder or file the selection lives in, shown as context. */
    base: string;
    /** Items to show: selected files, or the single source path. */
    items: string[];
    isFolder?: boolean;
    /** When set, each item can be removed from the selection. */
    onremove?: (index: number) => void;
  }

  let { base, items, isFolder = false, onremove }: Props = $props();

  const VIEW_STORAGE_KEY = "agentsync_source_view";
  type ViewMode = "grid" | "list";

  function storedView(): ViewMode {
    try {
      return localStorage?.getItem(VIEW_STORAGE_KEY) === "list" ? "list" : "grid";
    } catch {
      return "grid";
    }
  }

  let view = $state<ViewMode>(storedView());

  function setView(mode: ViewMode) {
    view = mode;
    try {
      localStorage?.setItem(VIEW_STORAGE_KEY, mode);
    } catch {
      // ignore storage errors
    }
  }
</script>

<div class="selection">
  <div class="selection-head">
    <span class="selection-base" title={base}>{base}</span>
    <span class="selection-count">{t("rules.source.selection_count", { count: items.length })}</span
    >
    <div class="view-toggle" role="group" aria-label={t("rules.source.view_label")}>
      <IconButton label={t("rules.source.view_grid")} onclick={() => setView("grid")}>
        <LayoutGrid size={14} class={view === "grid" ? "on" : ""} />
      </IconButton>
      <IconButton label={t("rules.source.view_list")} onclick={() => setView("list")}>
        <List size={14} class={view === "list" ? "on" : ""} />
      </IconButton>
    </div>
  </div>

  <ul class="items {view}">
    {#each items as item, index (item)}
      {@const ext = fileExtension(item)}
      <li class="item" title={item}>
        <span class="item-icon" aria-hidden="true">
          {#if isFolder}
            <Folder size={view === "grid" ? 20 : 14} />
          {:else}
            <FileText size={view === "grid" ? 20 : 14} />
          {/if}
        </span>
        <span class="item-name">{baseName(item)}</span>
        {#if ext && view === "list"}
          <span class="item-ext">{ext}</span>
        {/if}
        {#if onremove}
          <button
            type="button"
            class="item-remove"
            aria-label={t("rules.source.remove_item", { name: baseName(item) })}
            onclick={() => onremove(index)}
          >
            <X size={12} />
          </button>
        {/if}
      </li>
    {/each}
  </ul>
</div>

<style>
  .selection {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .selection-head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }

  .selection-base {
    flex: 1;
    min-width: 0;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    direction: rtl;
    text-align: left;
  }

  .selection-count {
    font-size: var(--text-xs);
    color: var(--text-muted);
    white-space: nowrap;
  }

  .view-toggle {
    display: flex;
    gap: 2px;
  }

  .view-toggle :global(.on) {
    color: var(--accent);
  }

  .items {
    margin: 0;
    padding: 2px;
    list-style: none;
    max-height: 220px;
    overflow-y: auto;
  }

  .items.grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(112px, 1fr));
    gap: var(--space-2);
  }

  .items.list {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .item {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    border-radius: var(--radius-sm);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    color: var(--ink);
    font-size: var(--text-xs);
  }

  .grid .item {
    flex-direction: column;
    justify-content: center;
    padding: var(--space-3) var(--space-2) var(--space-2);
    text-align: center;
  }

  .list .item {
    padding: var(--space-2) var(--space-3);
  }

  .item-icon {
    display: inline-flex;
    color: var(--text-muted);
    flex-shrink: 0;
  }

  .item-name {
    min-width: 0;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .list .item-name {
    flex: 1;
  }

  .item-ext {
    padding: 1px 6px;
    border-radius: var(--radius-xs);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
  }

  .item-remove {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 18px;
    height: 18px;
    border: none;
    border-radius: var(--radius-xs);
    background: none;
    color: var(--text-subtle);
    cursor: pointer;
  }

  .grid .item-remove {
    position: absolute;
    top: 4px;
    right: 4px;
  }

  .item-remove:hover {
    color: var(--dot-conflict);
    background-color: var(--surface-sunken);
  }
</style>
