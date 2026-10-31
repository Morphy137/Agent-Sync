<script lang="ts" module>
  export interface RepoBannerAction {
    label: string;
    primary?: boolean;
    onclick: () => void;
  }
</script>

<script lang="ts">
  import Button from "../Button.svelte";

  interface Props {
    variant?: "info" | "warning" | "danger" | "success";
    title: string;
    message: string;
    actions?: RepoBannerAction[];
  }

  let { variant = "info", title, message, actions = [] }: Props = $props();
</script>

<div class="banner" role="status">
  <span class="status-dot dot-{variant}" aria-hidden="true"></span>

  <div class="content">
    <span class="title">{title}</span>
    <span class="sep" aria-hidden="true">·</span>
    <span class="message">{message}</span>
  </div>

  {#if actions.length > 0}
    <div class="actions">
      {#each actions as action (action.label)}
        <Button
          size="sm"
          variant={action.primary ? "primary" : "secondary"}
          onclick={action.onclick}
        >
          {action.label}
        </Button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .banner {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-lg);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    color: var(--ink);
    font-size: var(--text-xs);
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: var(--radius-full);
    flex-shrink: 0;
  }

  .dot-info {
    background-color: var(--dot-down);
  }

  .dot-warning {
    background-color: var(--dot-up);
  }

  .dot-danger {
    background-color: var(--dot-conflict);
  }

  .dot-success {
    background-color: var(--dot-ok);
  }

  .content {
    flex: 1;
    min-width: 220px;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .title {
    font-weight: var(--weight-semibold);
  }

  .sep {
    color: var(--text-subtle);
  }

  .message {
    color: var(--text-muted);
  }

  .actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-shrink: 0;
    margin-left: auto;
  }
</style>
