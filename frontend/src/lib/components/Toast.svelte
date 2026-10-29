<script lang="ts">
  import { X } from "@lucide/svelte";
  import { t } from "../i18n";

  export interface ToastItem {
    id: string;
    type: "success" | "error" | "info";
    message: string;
  }

  interface Props {
    toasts: ToastItem[];
    ondismiss: (id: string) => void;
  }

  let { toasts, ondismiss }: Props = $props();
</script>

<div class="toast-container" aria-live="polite">
  {#each toasts.slice(-3) as toast (toast.id)}
    <div class="toast">
      <span
        class="status-dot"
        class:dot-ok={toast.type === "success"}
        class:dot-conflict={toast.type === "error"}
        class:dot-down={toast.type === "info"}
      ></span>
      <div class="message">{toast.message}</div>
      <button
        type="button"
        class="close-btn"
        onclick={() => ondismiss(toast.id)}
        aria-label={t("common.dismiss_toast")}
      >
        <X size={14} />
      </button>
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    bottom: var(--space-8);
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    z-index: 200;
    pointer-events: none;
    width: 100%;
    max-width: 420px;
    padding: 0 var(--space-4);
  }

  .toast {
    pointer-events: auto;
    width: 100%;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-lg);
    background-color: var(--surface);
    color: var(--ink);
    box-shadow: var(--elev-3);
    font-size: var(--text-sm);
    animation: fadeIn var(--duration-fast) var(--easing-out);
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: translateY(6px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: var(--radius-full);
    flex-shrink: 0;
  }

  .dot-ok {
    background-color: var(--dot-ok);
  }

  .dot-conflict {
    background-color: var(--dot-conflict);
  }

  .dot-down {
    background-color: var(--dot-down);
  }

  .message {
    flex: 1;
    line-height: 1.4;
  }

  .close-btn {
    background: none;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: var(--space-1);
    display: flex;
    align-items: center;
    border-radius: var(--radius-xs);
  }

  .close-btn:hover {
    color: var(--ink);
  }

  @media (prefers-reduced-motion: reduce) {
    .toast {
      animation: none;
    }
  }
</style>
