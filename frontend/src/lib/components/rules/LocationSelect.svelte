<script lang="ts">
  import { t } from "../../i18n";
  import LocationForm from "./LocationForm.svelte";
  import { BUILTIN_HOME_ALIAS, NEW_LOCATION_VALUE, type LocationOption } from "./ruleUi";

  interface Props {
    value: string;
    locations: LocationOption[];
    error?: string;
    onselect: (alias: string) => void;
    /** Called after a new location was saved from the inline form. */
    oncreated: (alias: string) => void;
  }

  let { value, locations, error, onselect, oncreated }: Props = $props();

  let creating = $state(false);
  const selectId = `location-select-${Math.random().toString(36).slice(2, 8)}`;
  const errorId = `${selectId}-error`;

  /** Includes the current value even when it is not mapped on this PC (e.g. rule shared from another PC). */
  const options = $derived.by((): LocationOption[] => {
    if (!value || locations.some((l) => l.alias === value)) return locations;
    return [...locations, { alias: value, path: "", mapped: false, builtin: false }];
  });

  function optionLabel(option: LocationOption): string {
    if (option.alias === BUILTIN_HOME_ALIAS) {
      return t("rules.location_select.home_option", { path: option.path });
    }
    if (!option.mapped) return t("rules.location_select.unmapped_option", { alias: option.alias });
    return t("rules.location_select.option", { alias: option.alias, path: option.path });
  }

  function handleChange(e: Event) {
    const select = e.currentTarget as HTMLSelectElement;
    if (select.value === NEW_LOCATION_VALUE) {
      select.value = value;
      creating = true;
      return;
    }
    onselect(select.value);
  }

  function handleCreated(alias: string) {
    creating = false;
    oncreated(alias);
  }
</script>

<div class="location-select">
  <label class="field-label" for={selectId}>{t("rules.location_select.label")}</label>
  <select
    id={selectId}
    class="select-input"
    class:invalid={Boolean(error)}
    {value}
    aria-invalid={Boolean(error)}
    aria-describedby={error ? errorId : undefined}
    onchange={handleChange}
  >
    <option value="" disabled>{t("rules.location_select.placeholder")}</option>
    {#each options as option (option.alias)}
      <option value={option.alias}>{optionLabel(option)}</option>
    {/each}
    <option value={NEW_LOCATION_VALUE}>{t("rules.location_select.new_option")}</option>
  </select>
  {#if error}
    <p id={errorId} class="field-error" role="alert">{error}</p>
  {/if}

  {#if creating}
    <LocationForm compact oncancel={() => (creating = false)} onsaved={handleCreated} />
  {/if}
</div>

<style>
  .location-select {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .field-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .select-input {
    height: 36px;
    padding: 0 var(--space-3);
    border-radius: var(--radius-md);
    border: none;
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    color: var(--ink);
    font-family: var(--font-sans);
    font-size: var(--text-sm);
    outline: none;
    cursor: pointer;
    width: 100%;
    min-width: 0;
  }

  .select-input:focus-visible {
    box-shadow: var(--focus-ring);
  }

  .select-input.invalid {
    box-shadow: 0 0 0 1px var(--dot-conflict);
  }

  .field-error {
    margin: 0;
    font-size: 11px;
    color: var(--dot-conflict);
  }
</style>
