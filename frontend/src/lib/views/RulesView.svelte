<script lang="ts">
  import { untrack } from "svelte";
  import { Plus, RefreshCw } from "@lucide/svelte";
  import { rules, type rulesync } from "../../../wailsjs/go/models";
  import { api, type RuleScope } from "../api";
  import { appState } from "../stores/appState.svelte";
  import { t } from "../i18n";
  import type { DiffAction, FileDiff } from "../diff/types";
  import Banner from "../components/Banner.svelte";
  import Button from "../components/Button.svelte";
  import EmptyState from "../components/EmptyState.svelte";
  import Modal from "../components/Modal.svelte";
  import Spinner from "../components/Spinner.svelte";
  import DiffModal from "../components/DiffModal.svelte";
  import RuleCard from "../components/rules/RuleCard.svelte";
  import CollisionBanner from "../components/rules/CollisionBanner.svelte";
  import {
    RESOLUTION,
    canWriteBack,
    destinationLabel,
    duplicateRule,
    sourceAbsPath,
  } from "../components/rules/ruleUi";

  interface ReviewState {
    status: rulesync.RuleStatus;
    destId: string;
    file: rulesync.FileStatus;
    diff: FileDiff | null;
    loading: boolean;
    busy: boolean;
  }

  let report = $state<rulesync.Report | null>(null);
  let loading = $state(true);
  let syncing = $state(false);
  let deleteTarget = $state<rulesync.RuleStatus | null>(null);
  let deleting = $state(false);
  let review = $state<ReviewState | null>(null);

  const ruleStatuses = $derived(report?.rules ?? []);
  const collisions = $derived(report?.collisions ?? []);
  const unmapped = $derived(report?.unmapped ?? []);
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
      report = await api.getRulesReport();
    } catch (err) {
      appState.showError(err);
    } finally {
      loading = false;
    }
  }

  function scopeOf(status: rulesync.RuleStatus): RuleScope {
    return status.scope === "local" ? "local" : "repo";
  }

  async function handleSync() {
    if (syncing) return;
    syncing = true;
    try {
      const result = await api.syncRules();
      appState.addToast(
        "success",
        t("rules.toast.synced", {
          written: result.written,
          captured: result.captured,
          skipped: result.skipped,
        })
      );
      await load();
    } catch (err) {
      appState.showError(err);
    } finally {
      syncing = false;
    }
  }

  async function handleToggle(status: rulesync.RuleStatus, enabled: boolean) {
    try {
      const result = await api.saveRule(
        rules.Rule.createFrom({ ...status.rule, enabled }),
        scopeOf(status)
      );
      if (result.collisions?.length) {
        appState.addToast("info", t("rules.toast.saved_with_collisions"));
      }
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
    if (status) handleEdit(status);
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
      appState.bumpRules();
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
  <div class="view-header">
    <div class="title-group">
      <h2 id="rules-view-title" class="section-title">{t("rules.title")}</h2>
      {#if report}
        <span class="section-summary">
          {t("rules.summary", {
            rules: ruleStatuses.length,
            pending: report.pending,
            conflicts: report.conflicts,
          })}
        </span>
      {/if}
    </div>
    <div class="header-actions">
      <Button
        variant="secondary"
        size="sm"
        disabled={syncing || ruleStatuses.length === 0}
        loading={syncing}
        onclick={handleSync}
      >
        <RefreshCw size={14} />
        <span>{t("rules.sync_button")}</span>
      </Button>
      <Button variant="primary" size="sm" onclick={() => appState.openRuleEditor()}>
        <Plus size={16} />
        <span>{t("rules.new_button")}</span>
      </Button>
    </div>
  </div>

  <p class="view-intro">{t("rules.intro")}</p>

  {#if unmapped.length > 0}
    <Banner
      variant="warning"
      title={t("rules.unmapped.title", { count: unmapped.length })}
      message={t("rules.unmapped.message", { aliases: unmapped.join(", ") })}
      actionLabel={t("rules.unmapped.action")}
      onaction={() => (appState.activeView = "locations")}
    />
  {/if}

  {#if collisions.length > 0}
    <CollisionBanner {collisions} {knownRules} onedit={handleEditById} />
  {/if}

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
  {:else}
    <div class="rules-grid">
      {#each ruleStatuses as status (status.rule.id)}
        <RuleCard
          {status}
          sourcePath={sourceAbsPath(status, rootPath)}
          ontoggle={(enabled) => handleToggle(status, enabled)}
          onedit={() => handleEdit(status)}
          onduplicate={() => handleDuplicate(status)}
          ondelete={() => (deleteTarget = status)}
          onopen={handleOpen}
          onreview={(destId, file) => openReview(status, destId, file)}
        />
      {/each}
    </div>
  {/if}
</section>

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
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .view-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-3);
  }

  .title-group {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: var(--space-3);
  }

  .section-title {
    margin: 0;
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
    letter-spacing: -0.01em;
  }

  .section-summary {
    font-size: var(--text-xs);
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .view-intro {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
    max-width: 72ch;
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
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 440px), 1fr));
    gap: var(--space-4);
    align-items: start;
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
