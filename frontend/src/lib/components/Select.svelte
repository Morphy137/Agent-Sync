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
</script>

<div class="select-group">
  {#if label}
    <label class="select-label" for="select-field">{label}</label>
  {/if}
  <select
    id="select-field"
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
    height: 38px;
    padding: 0 var(--space-3);
    border-radius: var(--radius-md);
    border: 1px solid var(--border);
    background-color: var(--surface);
    color: var(--text);
    font-family: var(--font-sans);
    font-size: var(--text-sm);
    outline: none;
    cursor: pointer;
    transition: border-color var(--duration-fast) var(--easing);
  }

  .select-input:focus {
    border-color: var(--accent);
    box-shadow: 0 0 0 2px var(--accent-ring);
  }
</style>
