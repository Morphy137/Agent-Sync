<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    tone?: "neutral" | "info" | "warning" | "danger" | "success" | "accent";
    mono?: boolean;
    title?: string;
    children?: Snippet;
  }

  let { tone = "neutral", mono = false, title, children }: Props = $props();
</script>

<span class="badge tone-{tone}" class:mono {title}>
  {@render children?.()}
</span>

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    height: 20px;
    padding: 0 var(--space-2);
    border-radius: var(--radius-full);
    font-size: 11px;
    font-weight: var(--weight-medium);
    line-height: 1;
    white-space: nowrap;
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--text-muted);
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .mono {
    font-family: var(--font-mono);
  }

  .tone-info {
    color: var(--dot-down);
    background-color: color-mix(in srgb, var(--dot-down) 12%, transparent);
    box-shadow: none;
  }

  .tone-warning {
    color: var(--dot-up);
    background-color: color-mix(in srgb, var(--dot-up) 14%, transparent);
    box-shadow: none;
  }

  .tone-danger {
    color: var(--dot-conflict);
    background-color: color-mix(in srgb, var(--dot-conflict) 12%, transparent);
    box-shadow: none;
  }

  .tone-success {
    color: var(--dot-ok);
    background-color: color-mix(in srgb, var(--dot-ok) 12%, transparent);
    box-shadow: none;
  }

  .tone-accent {
    color: var(--ink);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
  }
</style>
