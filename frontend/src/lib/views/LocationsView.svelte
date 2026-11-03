<script lang="ts">
  import ViewHeader from "../components/ViewHeader.svelte";
  import { untrack } from "svelte";
  import { FolderOpen, Pencil, Trash2, FileCog } from "@lucide/svelte";
  import type { service } from "../../../wailsjs/go/models";
  import { api } from "../api";
  import { appState } from "../stores/appState.svelte";
  import { t } from "../i18n";
  import Button from "../components/Button.svelte";
  import IconButton from "../components/IconButton.svelte";
  import MiddleEllipsis from "../components/MiddleEllipsis.svelte";
  import Modal from "../components/Modal.svelte";
  import Spinner from "../components/Spinner.svelte";
  import Badge from "../components/rules/Badge.svelte";
  import LocationForm from "../components/rules/LocationForm.svelte";
  import "../components/cardZones.css";

  let locations = $state<service.LocationInfo[]>([]);
  let loading = $state(true);
  let editing = $state<{ alias: string; path: string; locked: boolean } | null>(null);
  let removing = $state<service.LocationInfo | null>(null);
  let recognizersPath = $state("");

  const mapped = $derived(locations.filter((l) => l.path));
  const unmapped = $derived(locations.filter((l) => !l.path));

  $effect(() => {
    void appState.rulesVersion;
    untrack(() => void load());
  });

  async function load() {
    try {
      locations = await api.listLocations();
    } catch (err) {
      appState.showError(err);
    } finally {
      loading = false;
    }
  }

  function startAdd() {
    editing = { alias: "", path: "", locked: false };
  }

  function startEdit(loc: service.LocationInfo) {
    editing = { alias: loc.alias, path: loc.path, locked: true };
  }

  async function confirmRemove() {
    if (!removing) return;
    try {
      await api.removeLocation(removing.alias);
      appState.addToast("success", t("locations.toast.removed", { alias: removing.alias }));
      removing = null;
      appState.bumpRules();
    } catch (err) {
      appState.showError(err);
    }
  }

  async function openFolder(path: string) {
    try {
      await api.openPath(path);
    } catch (err) {
      appState.showError(err);
    }
  }

  function tagLabels(loc: service.LocationInfo): string[] {
    return [
      ...(loc.builtin ? [t("locations.builtin")] : []),
      ...(loc.profileLabel ? [t("locations.detected", { label: loc.profileLabel })] : []),
      ...(loc.exists ? [] : [t("locations.missing")]),
    ];
  }

  async function openRecognizers() {
    try {
      recognizersPath = await api.getRecognizersPath();
      await api.openPath(recognizersPath);
    } catch (err) {
      appState.showError(err);
    }
  }
</script>

<section class="view">
  <ViewHeader
    title={t("locations.title")}
    summary={t("locations.summary", { count: mapped.length })}
    intro={t("locations.intro")}
    primaryLabel={t("locations.add")}
    onprimary={startAdd}
  />

  {#if loading}
    <div class="loading"><Spinner size="sm" /></div>
  {:else}
    {#if unmapped.length > 0}
      <div class="unmapped">
        <h3 class="card-title">{t("locations.unmapped_title")}</h3>
        <p class="hint">{t("locations.unmapped_desc")}</p>
        <ul class="list">
          {#each unmapped as loc (loc.alias)}
            <li class="row row-unmapped">
              <span class="alias" title={loc.alias}>{loc.alias}</span>
              <span class="hint usage"
                >{t(`locations.used_by_${loc.ruleCount === 1 ? "one" : "other"}`, {
                  count: loc.ruleCount,
                })}</span
              >
              <div class="map-action">
                <Button
                  variant="secondary"
                  size="sm"
                  onclick={() => (editing = { alias: loc.alias, path: "", locked: true })}
                >
                  {t("locations.map_folder")}
                </Button>
              </div>
            </li>
          {/each}
        </ul>
      </div>
    {/if}

    <ul class="list card">
      {#each mapped as loc (loc.alias)}
        <li class="row">
          <span class="alias" title={loc.alias}>{loc.alias}</span>
          <span class="path"><MiddleEllipsis text={loc.path} /></span>
          <div class="tags" title={tagLabels(loc).join(" · ")}>
            {#if loc.builtin}<Badge>{t("locations.builtin")}</Badge>{/if}
            {#if loc.profileLabel}
              <Badge tone="success">{t("locations.detected", { label: loc.profileLabel })}</Badge>
            {/if}
            {#if !loc.exists}<Badge tone="danger">{t("locations.missing")}</Badge>{/if}
          </div>
          <span class="hint usage"
            >{t(`locations.used_by_${loc.ruleCount === 1 ? "one" : "other"}`, {
              count: loc.ruleCount,
            })}</span
          >
          <div class="row-actions">
            <IconButton label={t("locations.open")} onclick={() => openFolder(loc.path)}>
              <FolderOpen size={16} />
            </IconButton>
            {#if !loc.builtin}
              <IconButton label={t("locations.edit")} onclick={() => startEdit(loc)}>
                <Pencil size={16} />
              </IconButton>
            {:else}
              <span class="action-slot" aria-hidden="true"></span>
            {/if}
            <IconButton label={t("locations.remove")} onclick={() => (removing = loc)}>
              <Trash2 size={16} />
            </IconButton>
          </div>
        </li>
      {/each}
    </ul>

    <section class="card recognizers">
      <div class="row-title">
        <FileCog size={16} aria-hidden="true" />
        <h3 class="card-title">{t("locations.recognizers_title")}</h3>
      </div>
      <p class="hint">{t("locations.recognizers_desc")}</p>
      {#if recognizersPath}<span class="path"><MiddleEllipsis text={recognizersPath} /></span>{/if}
      <div>
        <Button variant="secondary" size="sm" onclick={openRecognizers}>
          {t("locations.recognizers_open")}
        </Button>
      </div>
    </section>
  {/if}
</section>

<Modal
  open={editing !== null}
  title={editing?.locked
    ? t("locations.edit_title", { alias: editing.alias })
    : t("locations.add_title")}
  onclose={() => (editing = null)}
>
  {#if editing}
    <LocationForm
      initialAlias={editing.alias}
      initialPath={editing.path}
      aliasLocked={editing.locked}
      oncancel={() => (editing = null)}
      onsaved={() => (editing = null)}
    />
  {/if}
</Modal>

<Modal
  open={removing !== null}
  title={t("locations.remove_title", { alias: removing?.alias ?? "" })}
  onclose={() => (removing = null)}
>
  <div class="confirm">
    <p class="hint">
      {removing && removing.ruleCount > 0
        ? t(`locations.remove_used_${removing.ruleCount === 1 ? "one" : "other"}`, {
            count: removing.ruleCount,
          })
        : t("locations.remove_desc")}
    </p>
    <div class="row-actions end">
      <Button variant="ghost" size="sm" onclick={() => (removing = null)}>
        {t("common.cancel")}
      </Button>
      <Button variant="primary" size="sm" onclick={confirmRemove}>{t("locations.remove")}</Button>
    </div>
  </div>
</Modal>

<style>
  .view {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .loading {
    display: flex;
    justify-content: center;
    padding: var(--space-6);
  }

  .card,
  .unmapped {
    background-color: var(--surface);
    border-radius: var(--radius-lg);
    box-shadow: var(--elev-1);
    padding: var(--space-3) var(--space-5);
  }

  .unmapped {
    box-shadow:
      var(--elev-1),
      inset 3px 0 0 var(--dot-up);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .list {
    list-style: none;
    margin: 0;
    display: flex;
    flex-direction: column;
  }

  .unmapped .list {
    padding: 0;
  }

  /*
   * One line per row on shared tracks, so alias, path, tags, usage and actions line up from row to row.
   * The path takes the flexible space and is cut in the middle when it does not fit.
   */
  .list {
    container-type: inline-size;
  }

  .row {
    display: grid;
    grid-template-columns:
      var(--card-zone-alias)
      minmax(0, 1fr)
      var(--card-zone-tags)
      var(--card-zone-usage)
      calc(3 * var(--card-zone-icon) + 2 * var(--space-1));
    grid-template-areas: "alias path tags usage actions";
    align-items: center;
    column-gap: var(--space-3);
    min-height: var(--card-zone-row);
  }

  .row-unmapped {
    grid-template-columns: var(--card-zone-alias) minmax(0, 1fr) var(--card-zone-usage) var(
        --card-zone-action
      );
    grid-template-areas: "alias . usage map";
  }

  @container (max-width: 900px) {
    .row {
      grid-template-columns:
        var(--card-zone-alias-compact)
        minmax(0, 1fr)
        var(--card-zone-tags-compact)
        var(--card-zone-usage-compact)
        calc(3 * var(--card-zone-icon) + 2 * var(--space-1));
    }
  }

  .row + .row {
    border-top: 1px solid var(--hairline);
  }

  .row-title {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
    color: var(--ink);
  }

  .alias {
    grid-area: alias;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: var(--weight-semibold);
    font-size: var(--text-sm);
    color: var(--ink);
  }

  .tags {
    grid-area: tags;
    display: flex;
    align-items: center;
    flex-wrap: nowrap;
    gap: var(--space-1);
    min-width: 0;
    overflow: hidden;
  }

  .usage {
    grid-area: usage;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .path {
    display: block;
    grid-area: path;
    min-width: 0;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .map-action {
    grid-area: map;
    display: flex;
    justify-content: flex-end;
  }

  .row-actions {
    grid-area: actions;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-1);
  }

  .row-actions.end {
    gap: var(--space-2);
  }

  .action-slot {
    width: var(--card-zone-icon);
    height: var(--card-zone-icon);
    flex-shrink: 0;
  }

  .card-title {
    margin: 0;
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .hint {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .recognizers {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .confirm {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
</style>
