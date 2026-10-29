<script lang="ts">
  interface Props {
    value?: string | number;
    label?: string;
    placeholder?: string;
    type?: string;
    disabled?: boolean;
    oninput?: (val: string) => void;
  }

  let {
    value = "",
    label,
    placeholder = "",
    type = "text",
    disabled = false,
    oninput,
  }: Props = $props();
</script>

<div class="input-group">
  {#if label}
    <label class="input-label" for="input-field">{label}</label>
  {/if}
  <input
    id="input-field"
    class="text-input"
    {type}
    {value}
    {placeholder}
    {disabled}
    oninput={(e) => oninput?.((e.target as HTMLInputElement).value)}
  />
</div>

<style>
  .input-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    width: 100%;
  }

  .input-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .text-input {
    height: 38px;
    padding: 0 var(--space-3);
    border-radius: var(--radius-md);
    border: 1px solid var(--border);
    background-color: var(--surface);
    color: var(--text);
    font-family: var(--font-sans);
    font-size: var(--text-sm);
    outline: none;
    transition: border-color var(--duration-fast) var(--easing);
  }

  .text-input:focus {
    border-color: var(--accent);
    box-shadow: 0 0 0 2px var(--accent-ring);
  }

  .text-input:disabled {
    opacity: 0.6;
    cursor: not-allowed;
    background-color: var(--surface-raised);
  }
</style>
