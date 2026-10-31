<script lang="ts">
  import { t } from "../../i18n";
  import Select from "../Select.svelte";
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

  const selectOptions = $derived([
    ...options.map((option) => ({ value: option.alias, label: optionLabel(option) })),
    { value: NEW_LOCATION_VALUE, label: t("rules.location_select.new_option") },
  ]);

  function handleChange(alias: string) {
    if (alias === NEW_LOCATION_VALUE) {
      creating = true;
      return;
    }
    onselect(alias);
  }

  function handleCreated(alias: string) {
    creating = false;
    oncreated(alias);
  }
</script>

<div class="location-select">
  <Select
    label={t("rules.location_select.label")}
    {value}
    options={selectOptions}
    placeholder={t("rules.location_select.placeholder")}
    {error}
    variant="raised"
    onchange={handleChange}
  />

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
</style>
