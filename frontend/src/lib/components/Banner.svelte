<script lang="ts">
  import Button from "./Button.svelte";

  interface Props {
    variant?: "info" | "warning" | "danger" | "success";
    title: string;
    message: string;
    actionLabel?: string;
    onaction?: () => void;
  }

  let { variant = "info", title, message, actionLabel, onaction }: Props = $props();
</script>

<div class="banner" role="alert">
  <span
    class="status-dot"
    class:dot-down={variant === "info"}
    class:dot-up={variant === "warning"}
    class:dot-conflict={variant === "danger"}
    class:dot-ok={variant === "success"}
  ></span>

  <div class="content">
    <span class="title">{title}</span>
    <span class="sep">·</span>
    <span class="message">{message}</span>
  </div>

  {#if actionLabel && onaction}
    <div class="action">
      <Button size="sm" variant="secondary" onclick={onaction}>
        {actionLabel}
      </Button>
    </div>
  {/if}
</div>

<style>
  .banner {
    display: flex;
    align-items: center;
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

  .dot-down {
    background-color: var(--dot-down);
  }

  .dot-up {
    background-color: var(--dot-up);
  }

  .dot-conflict {
    background-color: var(--dot-conflict);
  }

  .dot-ok {
    background-color: var(--dot-ok);
  }

  .content {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .title {
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .sep {
    color: var(--text-subtle);
  }

  .message {
    color: var(--text-muted);
  }

  .action {
    flex-shrink: 0;
  }
</style>
