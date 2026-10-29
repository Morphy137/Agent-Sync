<script lang="ts">
  import type { Snippet } from "svelte";
  import Spinner from "./Spinner.svelte";

  interface Props {
    variant?: "primary" | "secondary" | "ghost";
    size?: "sm" | "md";
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
    loading?: boolean;
    onclick?: (e: MouseEvent) => void;
    children?: Snippet;
    ariaLabel?: string;
  }

  let {
    variant = "secondary",
    size = "md",
    type = "button",
    disabled = false,
    loading = false,
    onclick,
    children,
    ariaLabel,
  }: Props = $props();
</script>

<button
  {type}
  class="btn btn-{variant} btn-{size}"
  disabled={disabled || loading}
  {onclick}
  aria-label={ariaLabel}
>
  {#if loading}
    <Spinner size="sm" />
  {/if}
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
    border: none;
    cursor: pointer;
    transition:
      background-color var(--duration-fast) var(--easing-out),
      box-shadow var(--duration-fast) var(--easing-out),
      transform var(--duration-fast) var(--easing-out),
      opacity var(--duration-fast) var(--easing-out);
    user-select: none;
    white-space: nowrap;
  }

  .btn:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .btn:disabled {
    opacity: 0.45;
    box-shadow: none;
    cursor: not-allowed;
    transform: none;
  }

  .btn-sm {
    height: 30px;
    padding: 0 var(--space-3);
    font-size: var(--text-xs);
    border-radius: var(--radius-sm);
  }

  .btn-md {
    height: 36px;
    padding: 0 var(--space-4);
    font-size: var(--text-sm);
  }

  /* Primary: --ink background, --ink-contrast text, --primary-elev */
  .btn-primary {
    background-color: var(--ink);
    color: var(--ink-contrast);
    box-shadow: var(--primary-elev);
  }

  .btn-primary:hover:not(:disabled) {
    background-color: var(--ink-hover);
    transform: translateY(-1px);
  }

  .btn-primary:active:not(:disabled) {
    transform: translateY(0);
    box-shadow: var(--elev-1);
  }

  /* Secondary: --surface + --elev-1, hover --elev-2 + translateY(-1px), active --pressed + --surface-sunken */
  .btn-secondary {
    background-color: var(--surface);
    color: var(--ink);
    box-shadow: var(--elev-1);
  }

  .btn-secondary:hover:not(:disabled) {
    box-shadow: var(--elev-2);
    transform: translateY(-1px);
  }

  .btn-secondary:active:not(:disabled) {
    background-color: var(--surface-sunken);
    box-shadow: var(--pressed);
    transform: translateY(0);
  }

  /* Ghost: transparent rest, --surface + --elev-1 hover, --pressed active */
  .btn-ghost {
    background-color: transparent;
    color: var(--ink);
    box-shadow: none;
  }

  .btn-ghost:hover:not(:disabled) {
    background-color: var(--surface);
    box-shadow: var(--elev-1);
  }

  .btn-ghost:active:not(:disabled) {
    background-color: var(--surface-sunken);
    box-shadow: var(--pressed);
  }

  @media (prefers-reduced-motion: reduce) {
    .btn,
    .btn-primary:hover:not(:disabled),
    .btn-secondary:hover:not(:disabled) {
      transform: none;
    }
  }
</style>
