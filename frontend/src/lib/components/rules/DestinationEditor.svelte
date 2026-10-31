<script lang="ts">
  import { Sparkles, Trash2 } from "@lucide/svelte";
  import { t } from "../../i18n";
  import IconButton from "../IconButton.svelte";
  import Input from "../Input.svelte";
  import SegmentedControl from "../SegmentedControl.svelte";
  import Select from "../Select.svelte";
  import Spinner from "../Spinner.svelte";
  import Switch from "../Switch.svelte";
  import Badge from "./Badge.svelte";
  import LocationSelect from "./LocationSelect.svelte";
  import {
    DEFAULT_TRANSFORM_KEYS,
    MODES,
    TRANSFORM_NONE,
    TRANSFORM_TYPES,
    type DestinationDraft,
    type DestinationErrors,
    type DestinationMode,
    type LocationOption,
    type TransformChoice,
  } from "./ruleUi";

  interface Props {
    draft: DestinationDraft;
    index: number;
    locations: LocationOption[];
    /** Translation keys of the validation errors to show. */
    errors?: DestinationErrors;
    sourceIsDir: boolean;
    suggesting: boolean;
    onchange: (patch: Partial<DestinationDraft>) => void;
    onlocation: (alias: string) => void;
    onlocationcreated: (alias: string) => void;
    onremove: () => void;
  }

  let {
    draft,
    index,
    locations,
    errors = {},
    sourceIsDir,
    suggesting,
    onchange,
    onlocation,
    onlocationcreated,
    onremove,
  }: Props = $props();

  const modeOptions = $derived(MODES.map((m) => ({ value: m, label: t(`rules.mode.${m}`) })));
  const transformOptions = $derived([
    { value: TRANSFORM_NONE, label: t("rules.transform.none") },
    ...TRANSFORM_TYPES.map((type) => ({ value: type, label: t(`rules.transform.${type}`) })),
  ]);
  const hasTransform = $derived(!sourceIsDir && draft.transform !== TRANSFORM_NONE);
  const fullPath = $derived(
    draft.location && draft.path.trim() ? `${draft.location}/${draft.path.trim()}` : ""
  );

  function selectTransform(transform: TransformChoice) {
    const transformKey = draft.transformKey || (DEFAULT_TRANSFORM_KEYS[transform] ?? "");
    onchange({ transform, transformKey });
  }

  function removeIgnored(relPath: string) {
    onchange({ ignore: draft.ignore.filter((p) => p !== relPath) });
  }
</script>

<fieldset class="dest-editor" class:dest-disabled={!draft.enabled}>
  <legend class="sr-only">{t("rules.editor.destination_legend", { index: index + 1 })}</legend>
  <div class="dest-head">
    <span class="dest-title" aria-hidden="true">
      {t("rules.editor.destination_legend", { index: index + 1 })}
    </span>
    <div class="dest-head-actions">
      <div class="enabled-toggle">
        <Switch
          checked={draft.enabled}
          label={t("rules.editor.destination_enabled")}
          onchange={(enabled) => onchange({ enabled })}
        />
      </div>
      <IconButton
        label={t("rules.editor.remove_destination", { index: index + 1 })}
        onclick={onremove}
      >
        <Trash2 size={14} />
      </IconButton>
    </div>
  </div>

  <LocationSelect
    value={draft.location}
    {locations}
    error={errors.location ? t(errors.location) : undefined}
    onselect={onlocation}
    oncreated={onlocationcreated}
  />

  {#if suggesting}
    <div class="suggestion">
      <Spinner size="sm" />
      <span>{t("rules.editor.suggesting")}</span>
    </div>
  {:else if draft.suggestion}
    <div class="suggestion" aria-live="polite">
      {#if draft.suggestion.detected}
        <Badge tone="success">
          <Sparkles size={12} aria-hidden="true" />
          {t("rules.editor.detected_chip", { label: draft.suggestion.label })}
        </Badge>
      {/if}
      {#if draft.touched}
        <span class="suggestion-note">{t("rules.editor.suggestion_edited")}</span>
      {/if}
    </div>
  {/if}

  <div class="field">
    <Input
      label={t("rules.editor.path_label")}
      value={draft.path}
      placeholder={t("rules.editor.path_placeholder")}
      oninput={(path) => onchange({ path })}
    />
    {#if errors.path}
      <p class="field-error" role="alert">{t(errors.path)}</p>
    {:else if fullPath}
      <p class="field-hint">{t("rules.editor.path_preview", { path: fullPath })}</p>
    {:else}
      <p class="field-hint">{t("rules.editor.path_hint")}</p>
    {/if}
  </div>

  {#if !sourceIsDir}
    <div class="transform-row">
      <div class="transform-type">
        <Select
          label={t("rules.editor.transform_label")}
          value={draft.transform}
          options={transformOptions}
          onchange={(v) => selectTransform(v as TransformChoice)}
        />
      </div>
      {#if hasTransform}
        <div class="transform-key">
          <Input
            label={t("rules.editor.transform_key_label")}
            value={draft.transformKey}
            placeholder={t("rules.editor.transform_key_placeholder")}
            oninput={(transformKey) => onchange({ transformKey })}
          />
          {#if errors.transformKey}
            <p class="field-error" role="alert">{t(errors.transformKey)}</p>
          {/if}
        </div>
      {/if}
    </div>
    {#if hasTransform}
      <p class="field-hint">{t("rules.editor.transform_hint")}</p>
    {/if}
  {/if}

  {#if hasTransform}
    <p class="mode-note">{t("rules.editor.mode_forced")}</p>
  {:else}
    <div class="field">
      <SegmentedControl
        label={t("rules.editor.mode_label")}
        size="sm"
        options={modeOptions}
        value={draft.mode}
        onchange={(mode) => onchange({ mode: mode as DestinationMode })}
      />
      <p class="field-hint">{t(`rules.mode_hint.${draft.mode}`)}</p>
    </div>
  {/if}

  {#if draft.ignore.length > 0}
    <div class="ignored">
      <span class="ignored-title">{t("rules.editor.ignored_title")}</span>
      <ul class="ignored-list">
        {#each draft.ignore as relPath (relPath)}
          <li class="ignored-row">
            <span class="ignored-path">{relPath}</span>
            <button type="button" class="link-btn" onclick={() => removeIgnored(relPath)}>
              {t("rules.editor.resume_syncing")}
            </button>
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</fieldset>

<style>
  .dest-editor {
    margin: 0;
    min-width: 0;
    border: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-4);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .dest-disabled > :not(.dest-head) {
    opacity: 0.6;
  }

  .dest-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .dest-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .dest-head-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .enabled-toggle {
    width: auto;
  }

  .suggestion {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .suggestion-note {
    font-size: 11px;
    color: var(--text-subtle);
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .field-hint,
  .field-error,
  .mode-note {
    margin: 0;
    font-size: 11px;
  }

  .field-hint,
  .mode-note {
    color: var(--text-subtle);
  }

  .field-error {
    color: var(--dot-conflict);
  }

  .transform-row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-3);
  }

  .transform-type {
    flex: 1 1 160px;
    min-width: 0;
  }

  .transform-key {
    flex: 1 1 160px;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .ignored {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .ignored-title {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .ignored-list {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .ignored-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    font-size: var(--text-xs);
    min-height: 24px;
  }

  .ignored-path {
    font-family: var(--font-mono);
    color: var(--ink);
    overflow-wrap: anywhere;
  }

  .link-btn {
    background: none;
    border: none;
    padding: 0 var(--space-1);
    border-radius: var(--radius-xs);
    color: var(--ink);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    text-decoration: underline;
    text-underline-offset: 2px;
    cursor: pointer;
    flex-shrink: 0;
  }

  .link-btn:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
</style>
