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

  const inputId = $derived(`input-${Math.random().toString(36).slice(2, 8)}`);
</script>

<div class="input-group">
  {#if label}
    <label class="input-label" for={inputId}>{label}</label>
  {/if}
  <input
    id={inputId}
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
    height: 36px;
    padding: 0 var(--space-3);
    border-radius: var(--radius-md);
    border: none;
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--ink);
    font-family: var(--font-sans);
    font-size: var(--text-sm);
    outline: none;
    transition: box-shadow var(--duration-fast) var(--easing-out);
  }

  .text-input::placeholder {
    color: var(--text-subtle);
  }

  .text-input:focus {
    box-shadow: var(--focus-ring);
  }

  .text-input:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
