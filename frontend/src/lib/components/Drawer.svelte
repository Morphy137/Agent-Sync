<script lang="ts">
  import type { Snippet } from "svelte";
  import { X } from "@lucide/svelte";
  import IconButton from "./IconButton.svelte";
  import { t } from "../i18n";

  interface Props {
    open: boolean;
    title: string;
    onclose: () => void;
    children?: Snippet;
  }

  let { open, title, onclose, children }: Props = $props();

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape" && open) {
      onclose();
    }
  }
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
  <div class="backdrop" onclick={onclose} role="presentation">
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <div
      class="drawer"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      aria-label={title}
      tabindex="-1"
    >
      <header class="drawer-header">
        <h2 class="drawer-title">{title}</h2>
        <IconButton label={t("common.close")} onclick={onclose}>
          <X size={18} />
        </IconButton>
      </header>

      <div class="drawer-body">
        {@render children?.()}
      </div>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background-color: var(--backdrop-overlay);
    display: flex;
    justify-content: flex-end;
    z-index: 90;
  }

  .drawer {
    background-color: var(--surface);
    width: 100%;
    max-width: 420px;
    height: 100%;
    display: flex;
    flex-direction: column;
    box-shadow: var(--elev-3);
    border-top-left-radius: var(--radius-xl);
    border-bottom-left-radius: var(--radius-xl);
    animation: drawerEnter var(--duration-normal) var(--easing-out);
  }

  @keyframes drawerEnter {
    from {
      opacity: 0;
      transform: translateX(16px);
    }
    to {
      opacity: 1;
      transform: translateX(0);
    }
  }

  .drawer-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-4) var(--space-6);
    border-bottom: 1px solid var(--hairline);
  }

  .drawer-title {
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
    margin: 0;
  }

  .drawer-body {
    padding: var(--space-5);
    overflow-y: auto;
    flex: 1;
  }

  @media (prefers-reduced-motion: reduce) {
    .drawer {
      animation: none;
    }
  }
</style>
