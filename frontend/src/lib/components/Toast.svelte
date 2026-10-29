<script lang="ts">
  import { CheckCircle2, AlertCircle, Info, X } from "@lucide/svelte";

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
    <div class="toast toast-{toast.type}">
      <div class="icon">
        {#if toast.type === "success"}
          <CheckCircle2 size={16} />
        {:else if toast.type === "error"}
          <AlertCircle size={16} />
        {:else}
          <Info size={16} />
        {/if}
      </div>
      <div class="message">{toast.message}</div>
      <button
        type="button"
        class="close-btn"
        onclick={() => ondismiss(toast.id)}
        aria-label="Dismiss toast"
      >
        <X size={14} />
      </button>
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    bottom: var(--space-6);
    right: var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    z-index: 200;
    pointer-events: none;
    max-width: 380px;
  }

  .toast {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-md);
    font-size: var(--text-sm);
    box-shadow: var(--shadow-md);
    border: 1px solid transparent;
    animation: fadeIn var(--duration-fast) var(--easing);
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .message {
    flex: 1;
    line-height: 1.4;
  }

  .close-btn {
    background: none;
    border: none;
    color: inherit;
    opacity: 0.6;
    cursor: pointer;
    padding: var(--space-1);
    display: flex;
    align-items: center;
  }

  .close-btn:hover {
    opacity: 1;
  }

  .toast-success {
    background-color: var(--surface);
    color: var(--success-fg);
    border-color: var(--success-border);
  }

  .toast-error {
    background-color: var(--surface);
    color: var(--danger-fg);
    border-color: var(--danger-border);
  }

  .toast-info {
    background-color: var(--surface);
    color: var(--info-fg);
    border-color: var(--info-border);
  }
</style>
