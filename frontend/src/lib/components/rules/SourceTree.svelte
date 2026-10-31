<script lang="ts">
  import { Check, ChevronDown, ChevronRight, FileText, Folder, Search } from "@lucide/svelte";
  import type { service } from "../../../../wailsjs/go/models";
  import { t } from "../../i18n";
  import Spinner from "../Spinner.svelte";
  import Badge from "./Badge.svelte";

  interface Props {
    entries: service.SourceTreeEntry[];
    selected: string;
    loading?: boolean;
    emptyText: string;
    onselect: (entry: service.SourceTreeEntry) => void;
    /** Multi-select mode: files get a checkbox and folders only expand. */
    multi?: boolean;
    checked?: string[];
    oncheck?: (path: string) => void;
  }

  let {
    entries,
    selected,
    loading = false,
    emptyText,
    onselect,
    multi = false,
    checked = [],
    oncheck,
  }: Props = $props();

  /** Kinds worth highlighting; plain files and folders need no badge. */
  const HIGHLIGHT_KINDS = ["skill", "mcp", "instructions"];
  const INDENT_PX = 14;

  let filter = $state("");
  // Start with the folders leading to the current selection expanded.
  let expanded = $state<string[]>(initialExpanded());
  const filterId = `tree-filter-${Math.random().toString(36).slice(2, 8)}`;

  function depthOf(path: string): number {
    return path.split("/").length - 1;
  }

  function nameOf(path: string): string {
    const parts = path.split("/");
    return parts[parts.length - 1];
  }

  function ancestorsOf(path: string): string[] {
    const parts = path.split("/");
    return parts.slice(0, -1).map((_, i) => parts.slice(0, i + 1).join("/"));
  }

  function initialExpanded(): string[] {
    return selected ? ancestorsOf(selected) : [];
  }

  const query = $derived(filter.trim().toLowerCase());
  const visible = $derived.by(() => {
    if (query) return entries.filter((e) => e.path.toLowerCase().includes(query));
    const open = new Set(expanded);
    return entries.filter((e) => ancestorsOf(e.path).every((a) => open.has(a)));
  });

  function toggle(path: string) {
    expanded = expanded.includes(path) ? expanded.filter((p) => p !== path) : [...expanded, path];
  }
</script>

<div class="source-tree">
  <div class="filter-row">
    <Search size={14} aria-hidden="true" />
    <label class="sr-only" for={filterId}>{t("rules.source.filter_label")}</label>
    <input
      id={filterId}
      class="filter-input"
      type="search"
      bind:value={filter}
      placeholder={t("rules.source.filter_placeholder")}
    />
  </div>

  {#if loading}
    <div class="tree-state">
      <Spinner size="sm" />
      <span>{t("common.loading")}</span>
    </div>
  {:else if visible.length === 0}
    <p class="tree-state">{query ? t("rules.source.no_matches") : emptyText}</p>
  {:else}
    <ul class="tree" aria-label={t("rules.source.tree_label")}>
      {#each visible as entry (entry.path)}
        {@const isChecked = multi && !entry.isDir && checked.includes(entry.path)}
        {@const isSelected = multi ? isChecked : entry.path === selected}
        {@const isOpen = expanded.includes(entry.path)}
        <li
          class="tree-row"
          class:selected={isSelected}
          style:padding-left="{query ? 0 : depthOf(entry.path) * INDENT_PX}px"
        >
          {#if entry.isDir && !query}
            <button
              type="button"
              class="chevron"
              aria-expanded={isOpen}
              aria-label={t(isOpen ? "rules.source.collapse" : "rules.source.expand", {
                name: nameOf(entry.path),
              })}
              onclick={() => toggle(entry.path)}
            >
              {#if isOpen}
                <ChevronDown size={14} />
              {:else}
                <ChevronRight size={14} />
              {/if}
            </button>
          {:else}
            <span class="chevron-spacer" aria-hidden="true"></span>
          {/if}
          <button
            type="button"
            class="entry"
            role={multi && !entry.isDir ? "checkbox" : undefined}
            aria-checked={multi && !entry.isDir ? isChecked : undefined}
            aria-pressed={multi ? undefined : isSelected}
            title={entry.path}
            onclick={() => {
              if (!multi) onselect(entry);
              else if (entry.isDir) toggle(entry.path);
              else oncheck?.(entry.path);
            }}
          >
            {#if multi && !entry.isDir}
              <span class="check-box" class:on={isChecked} aria-hidden="true">
                {#if isChecked}<Check size={11} />{/if}
              </span>
            {/if}
            {#if entry.isDir}
              <Folder size={14} aria-hidden="true" />
            {:else}
              <FileText size={14} aria-hidden="true" />
            {/if}
            <span class="entry-name">{query ? entry.path : nameOf(entry.path)}</span>
            {#if HIGHLIGHT_KINDS.includes(entry.kind)}
              <Badge tone="info">{t(`rules.kind.${entry.kind}`)}</Badge>
            {/if}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .check-box {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 14px;
    height: 14px;
    flex-shrink: 0;
    border-radius: 4px;
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--accent-contrast);
  }

  .check-box.on {
    background-color: var(--accent);
    box-shadow: none;
  }

  .source-tree {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .filter-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    height: 32px;
    padding: 0 var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    color: var(--text-subtle);
  }

  .filter-row:focus-within {
    box-shadow: var(--focus-ring);
  }

  .filter-input {
    flex: 1;
    min-width: 0;
    border: none;
    background: transparent;
    outline: none;
    color: var(--ink);
    font-family: var(--font-sans);
    font-size: var(--text-sm);
  }

  .filter-input::placeholder {
    color: var(--text-subtle);
  }

  .tree {
    list-style: none;
    margin: 0;
    padding: var(--space-1);
    max-height: 240px;
    overflow-y: auto;
    border-radius: var(--radius-md);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
  }

  .tree-row {
    display: flex;
    align-items: center;
    gap: 2px;
    border-radius: var(--radius-xs);
  }

  .tree-row.selected {
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .chevron,
  .chevron-spacer {
    width: 22px;
    height: 26px;
    flex-shrink: 0;
  }

  .chevron {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: none;
    color: var(--text-muted);
    border-radius: var(--radius-xs);
    cursor: pointer;
    padding: 0;
  }

  .entry {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    height: 26px;
    padding: 0 var(--space-2);
    border: none;
    background: none;
    border-radius: var(--radius-xs);
    color: var(--ink);
    font-size: var(--text-xs);
    text-align: left;
    cursor: pointer;
  }

  .entry:hover {
    background-color: var(--surface-sunken);
  }

  .selected .entry {
    font-weight: var(--weight-semibold);
  }

  .chevron:focus-visible,
  .entry:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .entry-name {
    font-family: var(--font-mono);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .tree-state {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    padding: var(--space-3);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
</style>
