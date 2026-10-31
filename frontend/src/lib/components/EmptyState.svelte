<script lang="ts">
  import { FolderOpen } from "@lucide/svelte";
  import Button from "./Button.svelte";

  interface Props {
    title: string;
    description: string;
    actionLabel?: string;
    onaction?: () => void;
  }

  let { title, description, actionLabel, onaction }: Props = $props();
</script>

<div class="empty-state">
  <div class="icon-wrap">
    <FolderOpen size={28} />
  </div>
  <h2 class="title">{title}</h2>
  <p class="description">{description}</p>
  {#if actionLabel && onaction}
    <div class="action-wrap">
      <Button variant="primary" onclick={onaction}>
        {actionLabel}
      </Button>
    </div>
  {/if}
</div>

<style>
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    padding: var(--space-10) var(--space-6);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    border-radius: var(--radius-lg);
    width: 100%;
    max-width: 540px;
    /* centered in the free space of the view (vertically too) */
    margin: auto;
  }

  .icon-wrap {
    width: 64px;
    height: 64px;
    border-radius: var(--radius-lg);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--text-muted);
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: var(--space-4);
  }

  .title {
    font-size: var(--text-xl);
    font-weight: var(--weight-semibold);
    color: var(--ink);
    margin: 0 0 var(--space-2) 0;
  }

  .description {
    font-size: var(--text-sm);
    color: var(--text-muted);
    line-height: 1.6;
    margin: 0 0 var(--space-6) 0;
  }

  .action-wrap {
    display: flex;
  }
</style>
