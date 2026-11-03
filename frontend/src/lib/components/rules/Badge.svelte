<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    tone?: "neutral" | "info" | "warning" | "danger" | "success" | "accent";
    mono?: boolean;
    title?: string;
    /** Text-only badge that shrinks with its cell and ends in an ellipsis instead of overflowing. */
    truncate?: boolean;
    children?: Snippet;
  }

  let { tone = "neutral", mono = false, title, truncate = false, children }: Props = $props();
</script>

<span class="badge tone-{tone}" class:mono class:truncate {title}>
  {@render children?.()}
</span>

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    justify-content: center;
    height: 22px;
    padding: 0 var(--space-2);
    border-radius: var(--radius-xs);
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

  .truncate {
    display: block;
    min-width: 0;
    line-height: 22px;
  }

  .tone-info {
    color: var(--dot-down);
    background-color: color-mix(in srgb, var(--dot-down) 12%, transparent);
  }

  .tone-warning {
    box-shadow: var(--sunken);
    color: var(--dot-up);
    background-color: color-mix(in srgb, var(--dot-up) 14%, transparent);
  }

  .tone-danger {
    box-shadow: var(--sunken);
    color: var(--dot-conflict);
    background-color: color-mix(in srgb, var(--dot-conflict) 12%, transparent);
  }

  .tone-success {
    box-shadow: var(--sunken);
    color: var(--dot-ok);
    background-color: color-mix(in srgb, var(--dot-ok) 12%, transparent);
  }

  .tone-accent {
    color: var(--accent);
    background-color: color-mix(in srgb, var(--accent) 12%, transparent);
  }
</style>
