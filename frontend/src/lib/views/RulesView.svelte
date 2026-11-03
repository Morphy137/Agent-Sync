<script lang="ts">
  import IconButton from "../components/IconButton.svelte";
  import ViewHeader from "../components/ViewHeader.svelte";
  import { untrack } from "svelte";
  import { RefreshCw, X } from "@lucide/svelte";
  import { rules, type rulesync } from "../../../wailsjs/go/models";
  import { api, type RuleScope } from "../api";
  import { appState } from "../stores/appState.svelte";
  import { t } from "../i18n";
  import type { DiffAction, FileDiff } from "../diff/types";
  import Button from "../components/Button.svelte";
  import Input from "../components/Input.svelte";
  import EmptyState from "../components/EmptyState.svelte";
  import Modal from "../components/Modal.svelte";
  import Spinner from "../components/Spinner.svelte";
  import DiffModal from "../components/DiffModal.svelte";
  import RuleCard from "../components/rules/RuleCard.svelte";
  import CollisionList from "../components/rules/CollisionList.svelte";
  import RuleSourceViewer from "../components/rules/RuleSourceViewer.svelte";
  import {
    RESOLUTION,
    canWriteBack,
    destinationLabel,
    duplicateRule,
    matchesRuleQuery,
    sourceAbsPath,
  } from "../components/rules/ruleUi";

  const SUMMARY_SEPARATOR = " · ";
  const SEARCH_ESCAPE_KEY = "Escape";

  interface ReviewState {
    status: rulesync.RuleStatus;
    destId: string;
    file: rulesync.FileStatus;
    diff: FileDiff | null;
    loading: boolean;
    busy: boolean;
  }

  const report = $derived(appState.ruleReport);
  let loading = $state(true);
  let syncing = $state(false);
  let deleteTarget = $state<rulesync.RuleStatus | null>(null);
  let deleting = $state(false);
  let review = $state<ReviewState | null>(null);
  /** The viewer lives here, outside the cards: a card's containment would trap its fixed overlay. */
  let viewer = $state<{ rule: rules.Rule; path: string } | null>(null);

  const ruleStatuses = $derived(report?.rules ?? []);
  const collisions = $derived(report?.collisions ?? []);
  let query = $state("");
  const visibleStatuses = $derived(
    ruleStatuses.filter((status) =>
      matchesRuleQuery(status, query, {
        origin: t(`rules.origin.${status.rule.source.origin}`),
        subscription: subscriptionName(status.rule.source.subscription),
      })
    )
  );
  const filtering = $derived(query.trim() !== "");
  const baseSummary = $derived(
    report
      ? t(`rules.summary_${ruleStatuses.length === 1 ? "one" : "other"}`, {
          rules: ruleStatuses.length,
          pending: t(`rules.pending_${report.pending === 1 ? "one" : "other"}`, {
            count: report.pending,
          }),
          conflicts: report.conflicts,
        })
      : undefined
  );
  const summary = $derived(
    baseSummary && filtering
      ? [
          baseSummary,
          t("rules.search.count", { shown: visibleStatuses.length, total: ruleStatuses.length }),
        ].join(SUMMARY_SEPARATOR)
      : baseSummary
  );
  const knownRules = $derived(ruleStatuses.map((s) => s.rule));
  const rootPath = $derived(appState.config?.rootPath ?? "");

  $effect(() => {
    // Reload whenever the backend reports rule, location or subscription changes.
    void appState.rulesVersion;
    untrack(() => {
      void load();
    });
  });

  async function load() {
    try {
      await appState.refreshRuleReport();
    } catch (err) {
      appState.showError(err);
    } finally {
      loading = false;
    }
  }

  function subscriptionName(id: string | undefined): string {
    if (!id) return "";
    return (
      appState.config?.subscriptions?.find((subscription) => subscription.id === id)?.name ?? id
    );
  }

  function clearSearch() {
    query = "";
  }

  function handleSearchKeydown(e: KeyboardEvent) {
    if (e.key !== SEARCH_ESCAPE_KEY || !query) return;
    e.stopPropagation();
    clearSearch();
  }

  function scopeOf(status: rulesync.RuleStatus): RuleScope {
    return status.scope === "local" ? "local" : "repo";
  }

  async function handleSync() {
    if (syncing) return;
    syncing = true;
    try {
      const result = await api.syncRules();
      const message = t("rules.toast.synced", {
        written: result.written,
        captured: result.captured,
        skipped: result.skipped,
      });
      if (result.skipped > 0) appState.addWarning(message);
      else appState.addToast("success", message);
      await load();
    } catch (err) {
      appState.showError(err);
    } finally {
      syncing = false;
    }
  }

  async function handleToggle(status: rulesync.RuleStatus, enabled: boolean) {
    try {
      await api.saveRule(rules.Rule.createFrom({ ...status.rule, enabled }), scopeOf(status));
      void appState.refreshLocalStatus();
      await load();
    } catch (err) {
      appState.showError(err);
    }
  }

  function handleEdit(status: rulesync.RuleStatus) {
    appState.openRuleEditor({ rule: status.rule, scope: scopeOf(status) });
  }

  function handleEditById(ruleId: string) {
    const status = ruleStatuses.find((s) => s.rule.id === ruleId);
    if (status) {
      appState.collisionDetailsOpen = false;
      handleEdit(status);
    }
  }

  function handleDuplicate(status: rulesync.RuleStatus) {
    const copy = duplicateRule(status.rule, t("rules.card.copy_name", { name: status.rule.name }));
    appState.openRuleEditor({ rule: copy, scope: scopeOf(status) });
  }

  async function handleConfirmDelete() {
    if (!deleteTarget || deleting) return;
    deleting = true;
    try {
      await api.deleteRule(deleteTarget.rule.id);
      appState.addToast("success", t("rules.toast.deleted", { name: deleteTarget.rule.name }));
      deleteTarget = null;
      appState.afterRuleChange();
    } catch (err) {
      appState.showError(err);
    } finally {
      deleting = false;
    }
  }

  async function handleOpen(path: string) {
    if (!path) return;
    try {
      await api.openPath(path);
    } catch (err) {
      appState.showError(err);
    }
  }

  async function openReview(
    status: rulesync.RuleStatus,
    destId: string,
    file: rulesync.FileStatus
  ) {
    review = { status, destId, file, diff: null, loading: true, busy: false };
    try {
      const diff = await api.getRuleDiff(status.rule.id, destId, file.relPath);
      if (review && review.file === file) review = { ...review, diff, loading: false };
    } catch (err) {
      appState.showError(err);
      review = null;
    }
  }

  async function resolve(choice: string, content?: string) {
    if (!review || review.busy) return;
    const current = review;
    review = { ...current, busy: true };
    try {
      await api.resolveRuleFile({
        ruleId: current.status.rule.id,
        destId: current.destId,
        relPath: current.file.relPath,
        choice,
        ...(content !== undefined ? { content } : {}),
      });
      appState.addToast("success", t("rules.toast.resolved", { file: current.file.relPath }));
      review = null;
      await load();
    } catch (err) {
      appState.showError(err);
      if (review) review = { ...review, busy: false };
    }
  }

  const reviewDestination = $derived(
    review?.status.rule.destinations.find((d) => d.id === review?.destId)
  );
  const reviewWritable = $derived(
    review ? canWriteBack(review.status.rule, reviewDestination) : false
  );

  const reviewActions = $derived.by((): DiffAction[] => {
    if (!review) return [];
    const busy = review.busy || review.loading;
    const actions: DiffAction[] = [
      {
        label: t("rules.review.keep_local"),
        variant: "ghost",
        disabled: busy,
        onclick: () => resolve(RESOLUTION.keepLocal),
      },
    ];
    if (reviewWritable) {
      actions.push({
        label: t("rules.review.use_target"),
        variant: "secondary",
        disabled: busy,
        onclick: () => resolve(RESOLUTION.useTarget),
      });
    }
    actions.push({
      label: t("rules.review.use_source"),
      variant: "primary",
      disabled: busy,
      onclick: () => resolve(RESOLUTION.useSource),
    });
    return actions;
  });

  const reviewSubtitle = $derived(
    review && reviewDestination
      ? t("rules.review.subtitle", {
          rule: review.status.rule.name,
          destination: destinationLabel(reviewDestination),
          file: review.file.relPath,
          status: t(`rules.file_status.${review.file.status}`),
        })
      : ""
  );
</script>

<section class="rules-view" aria-labelledby="rules-view-title">
  <ViewHeader
    title={t("rules.title")}
    titleId="rules-view-title"
    {summary}
    intro={t("rules.intro")}
    primaryLabel={t("rules.new_button")}
    onprimary={() => appState.openRuleEditor()}
  >
    {#snippet actions()}
      {#if ruleStatuses.length > 0}
        <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
        <div
          class="search"
          role="search"
          aria-label={t("rules.search.aria")}
          onkeydown={handleSearchKeydown}
        >
          <Input
            value={query}
            placeholder={t("rules.search.placeholder")}
            oninput={(value) => (query = value)}
          />
          {#if query}
            <span class="search-clear">
              <IconButton label={t("rules.search.clear")} onclick={clearSearch}>
                <X size={14} />
              </IconButton>
            </span>
          {/if}
        </div>
      {/if}
      <IconButton
        label={t("rules.sync_button")}
        disabled={syncing || ruleStatuses.length === 0}
        onclick={handleSync}
      >
        <RefreshCw size={16} class={syncing ? "spin" : ""} />
      </IconButton>
    {/snippet}
  </ViewHeader>

  {#if loading && !report}
    <div class="loading-row">
      <Spinner size="sm" />
      <span>{t("rules.loading")}</span>
    </div>
  {:else if ruleStatuses.length === 0}
    <EmptyState
      title={t("rules.empty.title")}
      description={t("rules.empty.description")}
      actionLabel={t("rules.empty.action")}
      onaction={() => appState.openRuleEditor()}
    />
  {:else if visibleStatuses.length === 0}
    <EmptyState
      title={t("rules.search.empty.title")}
      description={t("rules.search.empty.description", { query: query.trim() })}
      actionLabel={t("rules.search.empty.action")}
      onaction={clearSearch}
    />
  {:else}
    <div class="rules-grid">
      {#each visibleStatuses as status (status.rule.id)}
        <RuleCard
          {status}
          compact={true}
          sourcePath={sourceAbsPath(status, rootPath)}
          onsource={() => {
            appState.fileRuleId = status.rule.id;
            appState.activeView = "files";
          }}
          ontoggle={(enabled) => handleToggle(status, enabled)}
          onedit={() => handleEdit(status)}
          onduplicate={() => handleDuplicate(status)}
          ondelete={() => (deleteTarget = status)}
          onopen={handleOpen}
          onview={(path) => (viewer = { rule: status.rule, path: path ?? "" })}
          onreview={(destId, file) => openReview(status, destId, file)}
        />
      {/each}
    </div>
  {/if}
</section>

<RuleSourceViewer
  open={Boolean(viewer)}
  ruleId={viewer?.rule.id ?? ""}
  ruleName={viewer?.rule.name ?? ""}
  initialPath={viewer?.path ?? ""}
  onclose={() => (viewer = null)}
/>

<Modal
  open={appState.collisionDetailsOpen}
  title={t(`rules.collisions.banner_title_${collisions.length === 1 ? "one" : "other"}`, {
    count: collisions.length,
  })}
  onclose={() => (appState.collisionDetailsOpen = false)}
>
  <CollisionList {collisions} {knownRules} onedit={handleEditById} />
</Modal>

<Modal
  open={Boolean(deleteTarget)}
  title={t("rules.delete.title", { name: deleteTarget?.rule.name ?? "" })}
  onclose={() => (deleteTarget = null)}
>
  <div class="confirm-body">
    <p class="confirm-msg">{t("rules.delete.message")}</p>
    <div class="confirm-actions">
      <Button variant="secondary" onclick={() => (deleteTarget = null)}>
        {t("common.cancel")}
      </Button>
      <Button
        variant="primary"
        loading={deleting}
        disabled={deleting}
        onclick={handleConfirmDelete}
      >
        {t("rules.delete.confirm")}
      </Button>
    </div>
  </div>
</Modal>

<DiffModal
  open={Boolean(review)}
  title={t("rules.review.title")}
  subtitle={reviewSubtitle}
  diff={review?.diff ?? null}
  loading={review?.loading ?? false}
  actions={reviewActions}
  onsaveEdited={reviewWritable ? (content) => resolve(RESOLUTION.custom, content) : undefined}
  onclose={() => (review = null)}
/>

<style>
  .rules-view {
    --rules-search-width: 220px;
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .search {
    position: relative;
    width: var(--rules-search-width);
  }

  /* Room for the clear button inside the field. */
  .search :global(.text-input) {
    padding-right: var(--card-zone-icon);
  }

  .search-clear {
    position: absolute;
    top: 50%;
    right: var(--space-1);
    transform: translateY(-50%);
    display: flex;
  }

  .loading-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-muted);
    padding: var(--space-6) 0;
  }

  .rules-grid {
    display: grid;
    grid-template-columns: 1fr;
    gap: var(--space-3);
    align-items: stretch;
  }

  .confirm-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .confirm-msg {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }

  .confirm-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }
</style>
