<script lang="ts">
  import { api } from "../../api";
  import { t } from "../../i18n";
  import { appState } from "../../stores/appState.svelte";
  import Select from "../Select.svelte";
  import {
    BUILTIN_HOME_ALIAS,
    NEW_LOCATION_VALUE,
    type LocationOption,
    uniqueAliasForFolder,
  } from "./ruleUi";

  interface Props {
    value: string;
    locations: LocationOption[];
    error?: string;
    onselect: (alias: string) => void;
    /** Called after a new location was saved from a picked folder. */
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

  function samePath(a: string, b: string): boolean {
    const normalize = (p: string) =>
      p
        .replace(/[\\/]+$/, "")
        .replace(/\\/g, "/")
        .toLowerCase();
    return normalize(a) === normalize(b);
  }

  /** "New location" opens the folder dialog and saves the folder as a location right away. */
  async function createFromFolder() {
    if (creating) return;
    creating = true;
    try {
      const folder = await api.selectFolder(t("rules.location_select.pick_title"));
      if (!folder) return;
      const existing = locations.find((l) => l.mapped && samePath(l.path, folder));
      if (existing) {
        onselect(existing.alias);
        return;
      }
      const alias = uniqueAliasForFolder(
        folder,
        locations.map((l) => l.alias)
      );
      await api.saveLocation(alias, folder);
      appState.addToast("success", t("locations.toast.saved", { alias }));
      appState.bumpRules();
      oncreated(alias);
    } catch (err) {
      appState.showError(err);
    } finally {
      creating = false;
    }
  }

  function handleChange(alias: string) {
    if (alias === NEW_LOCATION_VALUE) {
      void createFromFolder();
      return;
    }
    onselect(alias);
  }
</script>

<div class="location-select">
  <Select
    label={t("rules.location_select.label")}
    {value}
    options={selectOptions}
    placeholder={t("rules.location_select.placeholder")}
    {error}
    disabled={creating}
    variant="raised"
    onchange={handleChange}
  />
</div>

<style>
  .location-select {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
</style>
