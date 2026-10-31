<script lang="ts">
  import { api } from "../../api";
  import { t } from "../../i18n";
  import { appState } from "../../stores/appState.svelte";
  import Button from "../Button.svelte";
  import Input from "../Input.svelte";
  import PathField from "../PathField.svelte";
  import { aliasError, aliasFromFolder, normalizeAlias, slugifyAlias } from "./ruleUi";

  interface Props {
    initialAlias?: string;
    initialPath?: string;
    /** Editing an existing alias: only the folder can change. */
    aliasLocked?: boolean;
    compact?: boolean;
    oncancel: () => void;
    onsaved: (alias: string) => void;
  }

  let {
    initialAlias = "",
    initialPath = "",
    aliasLocked = false,
    compact = false,
    oncancel,
    onsaved,
  }: Props = $props();

  function initialValues() {
    return { alias: initialAlias, path: initialPath };
  }

  const initial = initialValues();
  let alias = $state(initial.alias);
  let folder = $state(initial.path);
  let showErrors = $state(false);
  /** Stops auto-suggesting the alias from the folder once the user typed one. */
  let aliasTouched = $state(Boolean(initial.alias));

  function handleFolderChange(value: string) {
    folder = value;
    if (aliasLocked || aliasTouched) return;
    alias = aliasFromFolder(value);
  }

  function handleAliasInput(value: string) {
    aliasTouched = true;
    alias = slugifyAlias(value);
  }
  let saving = $state(false);

  const aliasErrorKey = $derived(aliasLocked ? null : aliasError(alias));
  const pathMissing = $derived(!folder.trim());

  async function save() {
    showErrors = true;
    if (aliasErrorKey || pathMissing || saving) return;
    saving = true;
    try {
      const normalized = normalizeAlias(alias);
      await api.saveLocation(normalized, folder.trim());
      appState.addToast("success", t("locations.toast.saved", { alias: normalized }));
      appState.bumpRules();
      onsaved(normalized);
    } catch (err) {
      appState.showError(err);
    } finally {
      saving = false;
    }
  }
</script>

<div class="location-form" class:compact role="group" aria-label={t("locations.form.group_label")}>
  <div class="field">
    <PathField
      label={t("locations.form.path_label")}
      value={folder}
      mode="root"
      allowCreate
      placeholder={t("locations.form.path_placeholder")}
      onchange={handleFolderChange}
    />
    {#if showErrors && pathMissing}
      <p class="field-error" role="alert">{t("locations.form.validation.path_required")}</p>
    {/if}
  </div>

  <div class="field">
    <Input
      label={t("locations.form.alias_label")}
      value={alias}
      placeholder={t("locations.form.alias_placeholder")}
      disabled={aliasLocked}
      oninput={handleAliasInput}
    />
    {#if showErrors && aliasErrorKey}
      <p class="field-error" role="alert">{t(aliasErrorKey)}</p>
    {:else}
      <p class="field-hint">{t("locations.form.alias_hint")}</p>
    {/if}
  </div>

  <div class="form-actions">
    <Button variant="ghost" size="sm" onclick={oncancel}>{t("common.cancel")}</Button>
    <Button variant="primary" size="sm" loading={saving} disabled={saving} onclick={save}>
      {t("locations.form.save")}
    </Button>
  </div>
</div>

<style>
  .location-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .location-form.compact {
    gap: var(--space-3);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .field-hint,
  .field-error {
    margin: 0;
    font-size: 11px;
  }

  .field-hint {
    color: var(--text-subtle);
  }

  .field-error {
    color: var(--dot-conflict);
  }

  .form-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }
</style>
