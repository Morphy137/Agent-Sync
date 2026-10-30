<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    label: string;
    disabled?: boolean;
    ariaExpanded?: boolean;
    onclick?: (e: MouseEvent) => void;
    children?: Snippet;
  }

  let {
    label,
    disabled = false,
    ariaExpanded,
    onclick,
    children,
  }: Props = $props();
</script>

<button
  type="button"
  class="icon-btn"
  aria-label={label}
  title={label}
  aria-expanded={ariaExpanded}
  {disabled}
  {onclick}
>
  {@render children?.()}
</button>

<style>
  .icon-btn {
    width: 32px;
    height: 32px;
    border-radius: var(--radius-sm);
    border: none;
    background-color: transparent;
    color: var(--text-muted);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    padding: 0;
    transition:
      background-color var(--duration-fast) var(--easing-out),
      box-shadow var(--duration-fast) var(--easing-out),
      color var(--duration-fast) var(--easing-out);
  }

  .icon-btn:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .icon-btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .icon-btn:hover:not(:disabled) {
    background-color: var(--surface);
    color: var(--ink);
    box-shadow: var(--elev-1);
  }

  .icon-btn:active:not(:disabled) {
    background-color: var(--surface-sunken);
    box-shadow: var(--pressed);
  }
</style>
