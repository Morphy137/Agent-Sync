<script lang="ts">
  import { ChevronDown, ChevronRight, Eye } from "@lucide/svelte";
  import { t } from "../../i18n";
  import Button from "../Button.svelte";

  interface Props {
    files: string[];
    onviewdiff: (path: string) => void;
  }

  let { files, onviewdiff }: Props = $props();

  let expanded = $state(false);
  const listId = `incoming-${Math.random().toString(36).slice(2, 8)}`;
</script>

<div class="incoming">
  <button
    type="button"
    class="incoming-toggle"
    aria-expanded={expanded}
    aria-controls={listId}
    onclick={() => (expanded = !expanded)}
  >
    {#if expanded}
      <ChevronDown size={14} />
    {:else}
      <ChevronRight size={14} />
    {/if}
    <span>
      {files.length === 1
        ? t("subscriptions.incoming.toggle_one")
        : t("subscriptions.incoming.toggle_other", { count: files.length })}
    </span>
  </button>

  {#if expanded}
    <ul class="incoming-list" id={listId}>
      {#each files as path (path)}
        <li class="incoming-row">
          <span class="incoming-path" title={path}>{path}</span>
          <Button
            size="sm"
            variant="ghost"
            ariaLabel={t("subscriptions.incoming.view_diff_for", { path })}
            onclick={() => onviewdiff(path)}
          >
            <Eye size={14} />
            <span>{t("subscriptions.incoming.view_diff")}</span>
          </Button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .incoming {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .incoming-toggle {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    align-self: flex-start;
    padding: 2px var(--space-1);
    border: none;
    border-radius: var(--radius-xs);
    background: none;
    color: var(--dot-down);
    font-family: var(--font-sans);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    cursor: pointer;
  }

  .incoming-toggle:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .incoming-list {
    list-style: none;
    margin: 0;
    padding: var(--space-1) var(--space-3);
    max-height: 180px;
    overflow-y: auto;
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-md);
  }

  .incoming-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    min-height: 34px;
  }

  .incoming-path {
    min-width: 0;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
