<script lang="ts">
  import type { Snippet } from "svelte";
  import { Plus } from "@lucide/svelte";
  import Button from "./Button.svelte";

  interface Props {
    title: string;
    titleId?: string;
    summary?: string;
    intro?: string;
    /** Label of the main "add" action; every view uses the same button size. */
    primaryLabel: string;
    onprimary: () => void;
    /** Extra icon actions shown before the main action. */
    actions?: Snippet;
  }

  let { title, titleId, summary, intro, primaryLabel, onprimary, actions }: Props = $props();
</script>

<header class="view-header">
  <div class="header-row">
    <div class="title-group">
      <h2 id={titleId} class="view-title">{title}</h2>
      {#if summary}<span class="view-summary">{summary}</span>{/if}
    </div>
    <div class="header-actions">
      {@render actions?.()}
      <div class="primary-action">
        <Button variant="primary" size="sm" onclick={onprimary}>
          <Plus size={16} />
          <span>{primaryLabel}</span>
        </Button>
      </div>
    </div>
  </div>
  {#if intro}<p class="view-intro">{intro}</p>{/if}
</header>

<style>
  .view-header {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .header-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    min-height: 36px;
  }

  .title-group {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
    min-width: 0;
  }

  .view-title {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: var(--weight-semibold);
    color: var(--ink);
    white-space: nowrap;
  }

  .view-summary {
    font-size: var(--text-xs);
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-shrink: 0;
  }

  /* Same width in every view so switching tabs does not resize the main action. */
  .primary-action {
    width: 168px;
    display: flex;
  }

  .primary-action :global(.btn) {
    width: 100%;
  }

  .view-intro {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
</style>
