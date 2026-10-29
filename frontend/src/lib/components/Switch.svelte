<script lang="ts">
  import { t } from "../i18n";

  interface Props {
    checked: boolean;
    label?: string;
    onchange?: (checked: boolean) => void;
  }

  let { checked = false, label, onchange }: Props = $props();

  function toggle() {
    const next = !checked;
    onchange?.(next);
  }
</script>

<label class="switch-container">
  {#if label}
    <span class="label">{label}</span>
  {/if}
  <button
    type="button"
    role="switch"
    aria-checked={checked}
    aria-label={label || t("common.toggle_switch")}
    class="switch-track"
    class:checked
    onclick={toggle}
  >
    <span class="switch-thumb"></span>
  </button>
</label>

<style>
  .switch-container {
    display: inline-flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    cursor: pointer;
    user-select: none;
    gap: var(--space-4);
  }

  .label {
    font-size: var(--text-sm);
    color: var(--ink);
  }

  .switch-track {
    position: relative;
    width: 40px;
    height: 22px;
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-full);
    border: none;
    padding: 2px;
    cursor: pointer;
    flex-shrink: 0;
    transition: background-color var(--duration-fast) var(--easing-out);
  }

  .switch-track:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .switch-track.checked {
    background-color: var(--ink);
  }

  .switch-thumb {
    display: block;
    width: 18px;
    height: 18px;
    background-color: var(--surface);
    border-radius: var(--radius-full);
    box-shadow: var(--elev-1);
    transition: transform var(--duration-fast) var(--easing-out);
  }

  .switch-track.checked .switch-thumb {
    background-color: var(--ink-contrast);
    transform: translateX(18px);
  }

  @media (prefers-reduced-motion: reduce) {
    .switch-thumb {
      transition: none;
    }
  }
</style>
