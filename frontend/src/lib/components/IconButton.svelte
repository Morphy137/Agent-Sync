<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    label: string;
    variant?: "ghost" | "secondary" | "primary";
    size?: "sm" | "md";
    disabled?: boolean;
    onclick?: (e: MouseEvent) => void;
    children?: Snippet;
  }

  let {
    label,
    variant = "ghost",
    size = "md",
    disabled = false,
    onclick,
    children,
  }: Props = $props();
</script>

<button
  type="button"
  class="icon-btn btn-{variant} btn-{size}"
  aria-label={label}
  title={label}
  {disabled}
  {onclick}
>
  {@render children?.()}
</button>

<style>
  .icon-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--radius-md);
    border: 1px solid transparent;
    cursor: pointer;
    transition: all var(--duration-fast) var(--easing);
  }

  .icon-btn:focus-visible {
    outline: none;
    box-shadow: 0 0 0 2px var(--accent-ring);
  }

  .icon-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn-sm {
    width: 28px;
    height: 28px;
    padding: var(--space-1);
  }

  .btn-md {
    width: 36px;
    height: 36px;
    padding: var(--space-2);
  }

  .btn-ghost {
    background-color: transparent;
    color: var(--text-muted);
  }

  .btn-ghost:hover:not(:disabled) {
    background-color: var(--surface-raised);
    color: var(--text);
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

  .btn-primary {
    background-color: var(--accent);
    color: var(--accent-contrast);
  }

  .btn-primary:hover:not(:disabled) {
    background-color: var(--accent-hover);
  }
</style>
