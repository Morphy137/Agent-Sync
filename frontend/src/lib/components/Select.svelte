<script lang="ts">
  interface Option {
    value: string | number;
    label: string;
  }

  interface Props {
    value: string | number;
    options: Option[];
    label?: string;
    onchange: (val: any) => void;
  }

  let { value, options, label, onchange }: Props = $props();

  const selectId = $derived(`select-${Math.random().toString(36).slice(2, 8)}`);
</script>

<div class="select-group">
  {#if label}
    <label class="select-label" for={selectId}>{label}</label>
  {/if}
  <select
    id={selectId}
    class="select-input"
    {value}
    onchange={(e) => onchange((e.target as HTMLSelectElement).value)}
  >
    {#each options as opt (opt.value)}
      <option value={opt.value}>{opt.label}</option>
    {/each}
  </select>
</div>

<style>
  .select-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    width: 100%;
  }

  .select-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .select-input {
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
    cursor: pointer;
    transition: box-shadow var(--duration-fast) var(--easing-out);
  }

  .select-input:focus {
    box-shadow: var(--focus-ring);
  }
</style>
