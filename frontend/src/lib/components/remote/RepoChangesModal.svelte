<script lang="ts">
  import { untrack } from "svelte";
  import {
    CloudDownload,
    Eye,
    FolderPen,
    GitCommitHorizontal,
    TriangleAlert,
  } from "@lucide/svelte";
  import type { RepoDiffMode } from "../../api";
  import { api } from "../../api";
  import type { DiffAction } from "../../diff/types";
  import { t } from "../../i18n";
  import { appState } from "../../stores/appState.svelte";
  import Modal from "../Modal.svelte";
  import Button from "../Button.svelte";
  import DiffModal from "../DiffModal.svelte";
  import { reportError } from "./reportError";
  import { diffModeTitle, downloadDiffMode } from "./repoChanges";
  import { RepoDiffViewer } from "./repoDiffViewer.svelte";

  interface Props {
    open: boolean;
    onclose: () => void;
    /** Starts the regular Sync flow (preview/confirm) after reviewing. */
    onsync?: () => void;
  }

  let { open, onclose, onsync }: Props = $props();

  const viewer = new RepoDiffViewer();

  let keptPaths = $state<string[]>([]);
  let pendingDiscard = $state<string | null>(null);
  let discardingPath = $state<string | null>(null);

  const plan = $derived(appState.statusPlan);
  const behind = $derived(plan?.behind ?? 0);
  const downloads = $derived(plan?.downloads ?? []);
  const uploads = $derived(plan?.uploads ?? []);
  const conflicts = $derived(plan?.conflicts ?? []);
  const localChanges = $derived(plan?.commitPreview ?? []);
  const outsideChanges = $derived(plan?.dirtyOutside ?? []);
  const downloadMode = $derived(downloadDiffMode(plan));
  const discardablePaths = $derived(new Set([...localChanges, ...outsideChanges]));
  const hasIncoming = $derived(behind > 0 || downloads.length > 0);
  const isEmpty = $derived(
    !hasIncoming &&
      conflicts.length === 0 &&
      localChanges.length === 0 &&
      uploads.length === 0 &&
      outsideChanges.length === 0
  );

  $effect(() => {
    if (!open) return;
    untrack(() => {
      keptPaths = [];
      pendingDiscard = null;
      viewer.close();
    });
  });

  const diffActions = $derived.by<DiffAction[]>(() => {
    const path = viewer.path;
    const closeAction: DiffAction = { label: t("common.close"), onclick: () => viewer.close() };
    if (viewer.mode !== "working" || !discardablePaths.has(path)) return [closeAction];
    return [
      {
        label: t("remote.changes.take_repo"),
        onclick: () => {
          pendingDiscard = path;
          viewer.close();
        },
      },
      {
        label: t("remote.changes.keep_mine"),
        variant: "primary",
        onclick: () => {
          keep(path);
          viewer.close();
        },
      },
    ];
  });

  function keep(path: string) {
    if (pendingDiscard === path) pendingDiscard = null;
    if (!keptPaths.includes(path)) keptPaths = [...keptPaths, path];
  }

  function askDiscard(path: string) {
    keptPaths = keptPaths.filter((p) => p !== path);
    pendingDiscard = path;
  }

  async function confirmDiscard(path: string) {
    if (discardingPath) return;
    discardingPath = path;
    try {
      await api.discardRepoFile(path);
      pendingDiscard = null;
      appState.addToast("success", t("remote.changes.discarded_toast", { path }));
      await appState.refreshAll();
    } catch (err) {
      reportError(err);
    } finally {
      discardingPath = null;
    }
  }

  function handleSync() {
    onclose();
    onsync?.();
  }
</script>

{#snippet fileRow(path: string, mode: RepoDiffMode, discardable: boolean)}
  <li class="file-row">
    <div class="file-line">
      <span class="file-path" title={path}>{path}</span>
      {#if keptPaths.includes(path)}
        <span class="kept-tag">{t("remote.changes.kept_tag")}</span>
      {/if}
      <div class="file-actions">
        <Button
          size="sm"
          variant="ghost"
          ariaLabel={t("remote.changes.view_diff_for", { path })}
          onclick={() => viewer.show(path, mode)}
        >
          <Eye size={14} />
          <span>{t("remote.changes.view_diff")}</span>
        </Button>
        {#if discardable}
          <Button
            size="sm"
            variant="ghost"
            ariaLabel={t("remote.changes.keep_mine_for", { path })}
            onclick={() => keep(path)}
          >
            {t("remote.changes.keep_mine")}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            ariaLabel={t("remote.changes.take_repo_for", { path })}
            onclick={() => askDiscard(path)}
          >
            {t("remote.changes.take_repo")}
          </Button>
        {/if}
      </div>
    </div>
    {#if pendingDiscard === path}
      <div
        class="confirm-strip"
        role="group"
        aria-label={t("remote.changes.discard_confirm_label")}
      >
        <p class="confirm-text">{t("remote.changes.discard_confirm", { path })}</p>
        <div class="confirm-actions">
          <Button size="sm" variant="ghost" onclick={() => (pendingDiscard = null)}>
            {t("common.cancel")}
          </Button>
          <Button
            size="sm"
            variant="primary"
            loading={discardingPath === path}
            onclick={() => confirmDiscard(path)}
          >
            {t("remote.changes.discard_confirm_button")}
          </Button>
        </div>
      </div>
    {/if}
  </li>
{/snippet}

<Modal open={open && !viewer.open} title={t("remote.changes.title")} {onclose}>
  <div class="review">
    <div class="legend">
      <p class="legend-title">{t("remote.changes.legend_title")}</p>
      <dl class="legend-list">
        <div>
          <dt>{t("diff.labels.repository")}</dt>
          <dd>{t("remote.changes.legend_repository")}</dd>
        </div>
        <div>
          <dt>{t("diff.labels.local")}</dt>
          <dd>{t("remote.changes.legend_local")}</dd>
        </div>
        <div>
          <dt>{t("diff.labels.remote")}</dt>
          <dd>{t("remote.changes.legend_remote")}</dd>
        </div>
      </dl>
    </div>

    {#if isEmpty}
      <p class="empty">{t("remote.changes.empty")}</p>
    {/if}

    {#if hasIncoming}
      <section class="group" aria-labelledby="review-incoming">
        <header class="group-header">
          <span class="group-icon dot-down"><CloudDownload size={15} /></span>
          <h3 id="review-incoming" class="group-title">{t("remote.changes.incoming_title")}</h3>
        </header>
        <p class="group-hint">
          {behind > 0
            ? t("remote.changes.incoming_behind", { count: behind })
            : t("remote.changes.incoming_hint")}
        </p>
        {#if downloads.length > 0}
          <ul class="file-list">
            {#each downloads as item (item.fullPath)}
              {@render fileRow(item.fullPath, downloadMode, false)}
            {/each}
          </ul>
        {/if}
      </section>
    {/if}

    {#if conflicts.length > 0}
      <section class="group" aria-labelledby="review-conflicts">
        <header class="group-header">
          <span class="group-icon dot-conflict"><TriangleAlert size={15} /></span>
          <h3 id="review-conflicts" class="group-title">
            {t("remote.changes.conflicts_title", { count: conflicts.length })}
          </h3>
        </header>
        <p class="group-hint">{t("remote.changes.conflicts_hint")}</p>
        <ul class="file-list">
          {#each conflicts as conflict (`${conflict.profile}/${conflict.relPath}`)}
            {@const path = `${conflict.profile}/${conflict.relPath}`}
            <li class="file-row">
              <div class="file-line">
                <span class="file-path" title={path}>{path}</span>
                <div class="file-actions">
                  {#if behind > 0}
                    <Button
                      size="sm"
                      variant="ghost"
                      ariaLabel={t("remote.changes.view_remote_for", { path })}
                      onclick={() => viewer.show(path, "incoming")}
                    >
                      <Eye size={14} />
                      <span>{t("remote.changes.view_remote")}</span>
                    </Button>
                  {/if}
                  <Button
                    size="sm"
                    variant="ghost"
                    ariaLabel={t("remote.changes.view_mine_for", { path })}
                    onclick={() => viewer.show(path, "working")}
                  >
                    <Eye size={14} />
                    <span>{t("remote.changes.view_mine")}</span>
                  </Button>
                </div>
              </div>
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if localChanges.length > 0}
      <section class="group" aria-labelledby="review-local">
        <header class="group-header">
          <span class="group-icon dot-up"><GitCommitHorizontal size={15} /></span>
          <h3 id="review-local" class="group-title">
            {t("remote.changes.local_title", { count: localChanges.length })}
          </h3>
        </header>
        <p class="group-hint">{t("remote.changes.local_hint")}</p>
        <ul class="file-list">
          {#each localChanges as path (path)}
            {@render fileRow(path, "working", true)}
          {/each}
        </ul>
      </section>
    {/if}

    {#if uploads.length > 0}
      <section class="group" aria-labelledby="review-uploads">
        <header class="group-header">
          <span class="group-icon dot-up"><FolderPen size={15} /></span>
          <h3 id="review-uploads" class="group-title">
            {t("remote.changes.uploads_title", { count: uploads.length })}
          </h3>
        </header>
        <p class="group-hint">{t("remote.changes.uploads_hint")}</p>
        <ul class="file-list">
          {#each uploads as item (item.fullPath)}
            {@render fileRow(item.fullPath, "working", false)}
          {/each}
        </ul>
      </section>
    {/if}

    {#if outsideChanges.length > 0}
      <section class="group" aria-labelledby="review-outside">
        <header class="group-header">
          <span class="group-icon dot-idle"><TriangleAlert size={15} /></span>
          <h3 id="review-outside" class="group-title">
            {t("remote.changes.outside_title", { count: outsideChanges.length })}
          </h3>
        </header>
        <p class="group-hint">{t("remote.changes.outside_hint")}</p>
        <ul class="file-list">
          {#each outsideChanges as path (path)}
            {@render fileRow(path, "working", true)}
          {/each}
        </ul>
      </section>
    {/if}

    <footer class="footer">
      <Button variant="secondary" onclick={onclose}>{t("common.close")}</Button>
      {#if onsync && !isEmpty}
        <Button variant="primary" onclick={handleSync}>{t("remote.changes.sync_now")}</Button>
      {/if}
    </footer>
  </div>
</Modal>

<DiffModal
  open={open && viewer.open}
  title={diffModeTitle(viewer.mode)}
  subtitle={viewer.path}
  diff={viewer.diff}
  loading={viewer.loading}
  actions={diffActions}
  onclose={() => viewer.close()}
/>

<style>
  .review {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  .legend {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-md);
  }

  .legend-title {
    margin: 0;
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .legend-list {
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .legend-list div {
    display: flex;
    gap: var(--space-2);
    font-size: var(--text-xs);
    line-height: 1.5;
  }

  .legend-list dt {
    flex-shrink: 0;
    min-width: 84px;
    font-weight: var(--weight-medium);
    color: var(--ink);
  }

  .legend-list dd {
    margin: 0;
    color: var(--text-muted);
  }

  .empty {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
    text-align: center;
    padding: var(--space-4) 0;
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .group-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .group-icon {
    display: inline-flex;
  }

  .dot-down {
    color: var(--dot-down);
  }

  .dot-up {
    color: var(--dot-up);
  }

  .dot-conflict {
    color: var(--dot-conflict);
  }

  .dot-idle {
    color: var(--text-muted);
  }

  .group-title {
    margin: 0;
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .group-hint {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.5;
  }

  .file-list {
    list-style: none;
    margin: 0;
    padding: var(--space-1) var(--space-3);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-md);
    max-height: 260px;
    overflow-y: auto;
  }

  .file-row {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-1) 0;
    border-bottom: 1px solid var(--hairline);
  }

  .file-row:last-child {
    border-bottom: none;
  }

  .file-line {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
    min-height: 32px;
  }

  .file-path {
    flex: 1 1 200px;
    min-width: 0;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .kept-tag {
    flex-shrink: 0;
    font-size: 11px;
    font-weight: var(--weight-medium);
    color: var(--dot-ok);
    padding: 1px var(--space-2);
    border-radius: var(--radius-full);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
  }

  .file-actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-1);
    margin-left: auto;
  }

  .confirm-strip {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    margin-bottom: var(--space-1);
    border-radius: var(--radius-sm);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
  }

  .confirm-text {
    flex: 1 1 240px;
    margin: 0;
    font-size: var(--text-xs);
    color: var(--ink);
    line-height: 1.5;
  }

  .confirm-actions {
    display: flex;
    gap: var(--space-2);
    margin-left: auto;
  }

  .footer {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-3);
  }
</style>
