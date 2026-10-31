<script lang="ts">
  import { Plus } from "@lucide/svelte";
  import { t } from "../i18n";
  import {
    THEMES,
    FEATURED_THEME_IDS,
    SYSTEM_THEME,
    resolveTheme,
    type ThemeDefinition,
  } from "../theme/theme";
  import Modal from "./Modal.svelte";
  import Switch from "./Switch.svelte";

  interface Props {
    value: string;
    onchange: (themeId: string) => void;
  }

  let { value, onchange }: Props = $props();

  let moreOpen = $state(false);

  const followsSystem = $derived(value === SYSTEM_THEME);
  const activeId = $derived(resolveTheme(value).id);
  const featured = $derived(
    FEATURED_THEME_IDS.map((id) => THEMES.find((theme) => theme.id === id)!)
  );
  /** A non-featured active theme takes the "More" slot so the selection stays visible. */
  const activeExtra = $derived(
    FEATURED_THEME_IDS.includes(activeId) ? undefined : THEMES.find((th) => th.id === activeId)
  );

  function pick(id: string) {
    onchange(id);
    moreOpen = false;
  }

  function handleSystemToggle(checked: boolean) {
    onchange(checked ? SYSTEM_THEME : activeId);
  }
</script>

{#snippet themeCard(theme: ThemeDefinition)}
  <button
    type="button"
    role="radio"
    class="theme-card"
    aria-checked={!followsSystem && activeId === theme.id}
    class:selected={!followsSystem && activeId === theme.id}
    onclick={() => pick(theme.id)}
  >
    <span
      class="theme-preview"
      style="--p-canvas: {theme.colors.canvas}; --p-surface: {theme.colors.surface}; --p-ink: {theme
        .colors.textMuted}; --p-accent: {theme.colors.accent};"
      aria-hidden="true"
    >
      <span class="preview-bar"></span>
      <span class="preview-line"></span>
      <span class="preview-accent"></span>
    </span>
    <span class="theme-name">{t(`settings.themes.${theme.id}`)}</span>
  </button>
{/snippet}

<div class="theme-picker">
  <span class="picker-label" id="theme-picker-label">{t("settings.theme_label")}</span>
  <div class="theme-grid" role="radiogroup" aria-labelledby="theme-picker-label">
    {#each featured as theme (theme.id)}
      {@render themeCard(theme)}
    {/each}
    {#if activeExtra}
      {@render themeCard(activeExtra)}
    {:else}
      <button type="button" class="theme-card more" onclick={() => (moreOpen = true)}>
        <span class="theme-preview more-preview" aria-hidden="true"><Plus size={18} /></span>
        <span class="theme-name">{t("settings.themes_more")}</span>
      </button>
    {/if}
  </div>
  {#if activeExtra}
    <button type="button" class="link-button" onclick={() => (moreOpen = true)}>
      {t("settings.themes_more")}
    </button>
  {/if}

  <Switch
    label={t("settings.theme_follow_system")}
    checked={followsSystem}
    onchange={handleSystemToggle}
  />
</div>

<Modal open={moreOpen} title={t("settings.themes_more_title")} onclose={() => (moreOpen = false)}>
  <div class="theme-grid wide" role="radiogroup" aria-label={t("settings.themes_more_title")}>
    {#each THEMES as theme (theme.id)}
      {@render themeCard(theme)}
    {/each}
  </div>
</Modal>

<style>
  .theme-picker {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .picker-label {
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    color: var(--text-muted);
  }

  .theme-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: var(--space-2);
  }

  .theme-grid.wide {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .theme-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2);
    border: none;
    border-radius: var(--radius-sm);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    color: var(--ink);
    font-family: var(--font-sans);
    cursor: pointer;
    transition: box-shadow var(--duration-fast) var(--easing-out);
  }

  .theme-card:hover {
    box-shadow:
      var(--sunken),
      0 0 0 1px var(--border);
  }

  .theme-card.selected,
  .theme-card:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .theme-preview {
    position: relative;
    width: 100%;
    height: 40px;
    border-radius: var(--radius-xs);
    background-color: var(--p-canvas);
    overflow: hidden;
    box-shadow: inset 0 0 0 1px rgba(127, 127, 127, 0.2);
  }

  .preview-bar {
    position: absolute;
    left: 6px;
    right: 6px;
    top: 6px;
    height: 12px;
    border-radius: 3px;
    background-color: var(--p-surface);
  }

  .preview-line {
    position: absolute;
    left: 10px;
    top: 10px;
    width: 40%;
    height: 4px;
    border-radius: 2px;
    background-color: var(--p-ink);
    opacity: 0.6;
  }

  .preview-accent {
    position: absolute;
    right: 8px;
    bottom: 6px;
    width: 22px;
    height: 10px;
    border-radius: 3px;
    background-color: var(--p-accent);
  }

  .more-preview {
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: var(--surface);
    color: var(--text-muted);
  }

  .theme-name {
    width: 100%;
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    text-align: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .link-button {
    align-self: center;
    border: none;
    background: none;
    color: var(--text-muted);
    font-family: var(--font-sans);
    font-size: var(--text-xs);
    cursor: pointer;
    text-decoration: underline;
  }

  .link-button:hover {
    color: var(--ink);
  }
</style>
