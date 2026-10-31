<script lang="ts">
  import { untrack } from "svelte";
  import { FolderOpen, Pencil, Plus, Trash2, FileCog } from "@lucide/svelte";
  import type { service } from "../../../wailsjs/go/models";
  import { api } from "../api";
  import { appState } from "../stores/appState.svelte";
  import { t } from "../i18n";
  import Button from "../components/Button.svelte";
  import IconButton from "../components/IconButton.svelte";
  import Modal from "../components/Modal.svelte";
  import Spinner from "../components/Spinner.svelte";
  import Badge from "../components/rules/Badge.svelte";
  import LocationForm from "../components/rules/LocationForm.svelte";

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
  <header class="view-header">
    <div class="title-group">
      <h2 class="view-title">{t("locations.title")}</h2>
      <span class="summary">{t("locations.summary", { count: mapped.length })}</span>
    </div>
    <Button variant="primary" size="sm" onclick={startAdd}>
      <Plus size={16} />
      <span>{t("locations.add")}</span>
    </Button>
  </header>
  <p class="intro">{t("locations.intro")}</p>

  {#if loading}
    <div class="loading"><Spinner size="sm" /></div>
  {:else}
    {#if unmapped.length > 0}
      <div class="unmapped" role="alert">
        <h3 class="card-title">{t("locations.unmapped_title")}</h3>
        <p class="hint">{t("locations.unmapped_desc")}</p>
        <ul class="list">
          {#each unmapped as loc (loc.alias)}
            <li class="row">
              <div class="row-main">
                <span class="alias">{loc.alias}</span>
                <span class="hint">{t("locations.used_by", { count: loc.ruleCount })}</span>
              </div>
              <Button
                variant="secondary"
                size="sm"
                onclick={() => (editing = { alias: loc.alias, path: "", locked: true })}
              >
                {t("locations.map_folder")}
              </Button>
            </li>
          {/each}
        </ul>
      </div>
    {/if}

    <ul class="list card">
      {#each mapped as loc (loc.alias)}
        <li class="row">
          <div class="row-main">
            <div class="row-title">
              <span class="alias">{loc.alias}</span>
              {#if loc.builtin}<Badge>{t("locations.builtin")}</Badge>{/if}
              {#if loc.profileLabel}
                <Badge tone="success">{t("locations.detected", { label: loc.profileLabel })}</Badge>
              {/if}
              {#if !loc.exists}<Badge tone="danger">{t("locations.missing")}</Badge>{/if}
            </div>
            <span class="path" title={loc.path}>{loc.path}</span>
            <span class="hint">{t("locations.used_by", { count: loc.ruleCount })}</span>
          </div>
          <div class="row-actions">
            <IconButton label={t("locations.open")} onclick={() => openFolder(loc.path)}>
              <FolderOpen size={16} />
            </IconButton>
            {#if !loc.builtin}
              <IconButton label={t("locations.edit")} onclick={() => startEdit(loc)}>
                <Pencil size={16} />
              </IconButton>
              <IconButton label={t("locations.remove")} onclick={() => (removing = loc)}>
                <Trash2 size={16} />
              </IconButton>
            {/if}
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
      {#if recognizersPath}<span class="path">{recognizersPath}</span>{/if}
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
        ? t("locations.remove_used", { count: removing.ruleCount })
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

  .view-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }

  .title-group {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
  }

  .view-title {
    margin: 0;
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .summary,
  .hint {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .intro {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
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
    padding: var(--space-4) var(--space-5);
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

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-3) 0;
  }

  .row + .row {
    border-top: 1px solid var(--hairline);
  }

  .row-main {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .row-title {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
    color: var(--ink);
  }

  .alias {
    font-weight: var(--weight-semibold);
    font-size: var(--text-sm);
    color: var(--ink);
  }

  .path {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .row-actions {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    flex-shrink: 0;
  }

  .row-actions.end {
    justify-content: flex-end;
    gap: var(--space-2);
  }

  .card-title {
    margin: 0;
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
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
