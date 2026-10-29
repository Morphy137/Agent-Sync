<script lang="ts">
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
    aria-label={label || "Toggle switch"}
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
    color: var(--text);
  }

  .switch-track {
    position: relative;
    width: 40px;
    height: 22px;
    background-color: var(--border-strong);
    border-radius: var(--radius-full);
    border: none;
    padding: 2px;
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--easing);
  }

  .switch-track:focus-visible {
    outline: none;
    box-shadow: 0 0 0 2px var(--accent-ring);
  }

  .switch-track.checked {
    background-color: var(--accent);
  }

  .switch-thumb {
    display: block;
    width: 18px;
    height: 18px;
    background-color: var(--surface);
    border-radius: var(--radius-full);
    transition: transform var(--duration-fast) var(--easing);
    box-shadow: var(--shadow-sm);
  }

  .switch-track.checked .switch-thumb {
    transform: translateX(18px);
  }
</style>
