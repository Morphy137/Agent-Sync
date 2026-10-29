<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    variant?: "primary" | "secondary" | "ghost" | "danger";
    size?: "sm" | "md";
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
    onclick?: (e: MouseEvent) => void;
    children?: Snippet;
    ariaLabel?: string;
  }

  let {
    variant = "secondary",
    size = "md",
    type = "button",
    disabled = false,
    onclick,
    children,
    ariaLabel,
  }: Props = $props();
</script>

<button
  {type}
  class="btn btn-{variant} btn-{size}"
  {disabled}
  {onclick}
  aria-label={ariaLabel}
>
  {@render children?.()}
</button>

<style>
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    font-family: var(--font-sans);
    font-weight: var(--weight-medium);
    border-radius: var(--radius-md);
    border: 1px solid transparent;
    cursor: pointer;
    transition: all var(--duration-fast) var(--easing);
    user-select: none;
    white-space: nowrap;
  }

  .btn:focus-visible {
    outline: none;
    box-shadow: 0 0 0 2px var(--accent-ring);
  }

  .btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn-sm {
    height: 30px;
    padding: 0 var(--space-3);
    font-size: var(--text-xs);
  }

  .btn-md {
    height: 38px;
    padding: 0 var(--space-4);
    font-size: var(--text-sm);
  }

  .btn-primary {
    background-color: var(--accent);
    color: var(--accent-contrast);
  }

  .btn-primary:hover:not(:disabled) {
    background-color: var(--accent-hover);
  }

  .btn-secondary {
    background-color: var(--surface);
    color: var(--text);
    border-color: var(--border);
  }

  .btn-secondary:hover:not(:disabled) {
    background-color: var(--surface-raised);
    border-color: var(--border-strong);
  }

  .btn-ghost {
    background-color: transparent;
    color: var(--text);
  }

  .btn-ghost:hover:not(:disabled) {
    background-color: var(--surface-raised);
  }

  .btn-danger {
    background-color: var(--danger-bg);
    color: var(--danger-fg);
    border-color: var(--danger-border);
  }

  .btn-danger:hover:not(:disabled) {
    background-color: var(--danger-border);
  }
</style>
