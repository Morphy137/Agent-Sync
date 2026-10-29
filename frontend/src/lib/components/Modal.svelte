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
      class="dialog"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-title"
      tabindex="-1"
    >
      <header class="modal-header">
        <h2 id="modal-title" class="modal-title">{title}</h2>
        <IconButton label={t("common.close")} onclick={onclose}>
          <X size={18} />
        </IconButton>
      </header>

      <div class="modal-body">
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
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .dialog {
    background-color: var(--surface);
    border-radius: var(--radius-xl);
    box-shadow: var(--elev-3);
    width: 90%;
    max-width: 600px;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: modalEnter var(--duration-normal) var(--easing-out);
  }

  @keyframes modalEnter {
    from {
      opacity: 0;
      transform: scale(0.98);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-4) var(--space-6);
    border-bottom: 1px solid var(--hairline);
  }

  .modal-title {
    font-size: var(--text-lg);
    font-weight: var(--weight-semibold);
    color: var(--ink);
    margin: 0;
  }

  .modal-body {
    padding: var(--space-6);
    overflow-y: auto;
  }

  @media (prefers-reduced-motion: reduce) {
    .dialog {
      animation: none;
    }
  }
</style>
