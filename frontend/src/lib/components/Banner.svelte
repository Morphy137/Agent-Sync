<script lang="ts">
  import { AlertCircle, Info, TriangleAlert, CheckCircle } from "@lucide/svelte";
  import Button from "./Button.svelte";

  interface Props {
    variant?: "info" | "warning" | "danger" | "success";
    title: string;
    message: string;
    actionLabel?: string;
    onaction?: () => void;
  }

  let {
    variant = "info",
    title,
    message,
    actionLabel,
    onaction,
  }: Props = $props();
</script>

<div class="banner banner-{variant}" role="alert">
  <div class="icon-wrap">
    {#if variant === "info"}
      <Info size={18} />
    {:else if variant === "warning"}
      <TriangleAlert size={18} />
    {:else if variant === "danger"}
      <AlertCircle size={18} />
    {:else}
      <CheckCircle size={18} />
    {/if}
  </div>

  <div class="content">
    <div class="title">{title}</div>
    <div class="message">{message}</div>
  </div>

  {#if actionLabel && onaction}
    <div class="action">
      <Button size="sm" variant={variant === "danger" ? "danger" : "secondary"} onclick={onaction}>
        {actionLabel}
      </Button>
    </div>
  {/if}
</div>

<style>
  .banner {
    display: flex;
    align-items: center;
    gap: var(--space-4);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-md);
    border: 1px solid transparent;
    font-size: var(--text-sm);
  }

  .icon-wrap {
    display: flex;
    align-items: center;
    flex-shrink: 0;
  }

  .content {
    flex: 1;
    min-width: 0;
  }

  .title {
    font-weight: var(--weight-semibold);
    margin-bottom: var(--space-1);
  }

  .message {
    opacity: 0.9;
    font-size: var(--text-xs);
  }

  .action {
    flex-shrink: 0;
  }

  .banner-info {
    background-color: var(--info-bg);
    color: var(--info-fg);
    border-color: var(--info-border);
  }

  .banner-warning {
    background-color: var(--warning-bg);
    color: var(--warning-fg);
    border-color: var(--warning-border);
  }

  .banner-danger {
    background-color: var(--danger-bg);
    color: var(--danger-fg);
    border-color: var(--danger-border);
  }

  .banner-success {
    background-color: var(--success-bg);
    color: var(--success-fg);
    border-color: var(--success-border);
  }
</style>
