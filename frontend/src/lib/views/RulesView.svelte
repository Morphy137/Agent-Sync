<script lang="ts">
  import IconButton from "../components/IconButton.svelte";
  import ViewHeader from "../components/ViewHeader.svelte";
  import { onDestroy, untrack } from "svelte";
  import { ChevronRight, GripVertical, RefreshCw, X } from "@lucide/svelte";
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
  import { categoryColor, categoryIcon, categoryLabel } from "../components/rules/categories";
  import RuleSourceViewer from "../components/rules/RuleSourceViewer.svelte";
  import {
    RESOLUTION,
    canWriteBack,
    destinationLabel,
    duplicateRule,
    matchesRuleQuery,
    needsReview,
    sourceAbsPath,
  } from "../components/rules/ruleUi";

  const SUMMARY_SEPARATOR = " · ";
  const SEARCH_ESCAPE_KEY = "Escape";
  const CATEGORY_STORAGE_PREFIX = "agentsync_collapsed_categories:";
  const UNCATEGORIZED = "uncategorized";
  const CATEGORY_ORDER_PREFIX = "agentsync_category_order:";
  const RULE_ORDER_PREFIX = "agentsync_rule_order:";
  const DRAG_THRESHOLD_PX = 6;
  type DragItem = { type: "category" | "rule"; id: string; categoryId: string };

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
  let categories = $state<rules.Category[]>([]);
  let collapsed = $state<string[]>([]);
  let categoryOrder = $state<string[]>([]);
  let ruleOrder = $state<string[]>([]);
  let dragging = $state<DragItem | null>(null);
  let previewOrder = $state<string[]>([]);
  let lastDragTarget = "";
  let pointer: {
    id: number;
    item: DragItem;
    handle: HTMLElement;
    component: HTMLElement;
    x: number;
    y: number;
    offsetX: number;
    offsetY: number;
  } | null = null;
  let dragPreview: HTMLElement | null = null;
  const categoryStorageKey = $derived(CATEGORY_STORAGE_PREFIX + (appState.config?.rootPath ?? ""));
  const categoryOrderKey = $derived(CATEGORY_ORDER_PREFIX + (appState.config?.rootPath ?? ""));
  const ruleOrderKey = $derived(RULE_ORDER_PREFIX + (appState.config?.rootPath ?? ""));

  function readOrder(key: string): string[] {
    try {
      const value: unknown = JSON.parse(localStorage.getItem(key) ?? "[]");
      return Array.isArray(value) ? value.filter((id): id is string => typeof id === "string") : [];
    } catch {
      return [];
    }
  }
  function persistOrder(key: string, ids: string[]) {
    try {
      localStorage.setItem(key, JSON.stringify(ids));
    } catch {
      /* Storage is optional. */
    }
  }
  function orderedIds(ids: string[], saved: string[]): string[] {
    const available = new Set(ids);
    return [
      ...saved.filter((id, index) => available.has(id) && saved.indexOf(id) === index),
      ...ids.filter((id) => !saved.includes(id)),
    ];
  }
  function moveId(ids: string[], source: string, target: string): string[] {
    const from = ids.indexOf(source),
      to = ids.indexOf(target);
    if (from < 0 || to < 0 || from === to) return ids;
    const next = [...ids];
    next.splice(from, 1);
    next.splice(to, 0, source);
    return next;
  }
  function reorder(item: DragItem, target: string) {
    if (filtering) return;
    if (item.type === "category") {
      const ids = orderedIds(
        [...categories.map((category) => category.id), UNCATEGORIZED],
        categoryOrder
      );
      categoryOrder = [
        ...moveId(ids, item.id, target),
        ...categoryOrder.filter((id) => !ids.includes(id)),
      ];
      persistOrder(categoryOrderKey, categoryOrder);
    } else {
      const group = groups.find((entry) => entry.category.id === item.categoryId);
      if (!group || !group.statuses.some((status) => status.rule.id === target)) return;
      const ids = orderedIds(
        ruleStatuses.map((status) => status.rule.id),
        ruleOrder
      );
      ruleOrder = [...moveId(ids, item.id, target), ...ruleOrder.filter((id) => !ids.includes(id))];
      persistOrder(ruleOrderKey, ruleOrder);
    }
  }
  function startPointerDrag(event: PointerEvent, item: DragItem) {
    if (filtering || event.button !== 0 || pointer) return;
    const handle = event.currentTarget as HTMLElement;
    const component = handle.closest<HTMLElement>(
      item.type === "category" ? ".category" : ".rule-order-row"
    );
    if (!component) return;
    const rect = component.getBoundingClientRect();
    pointer = {
      id: event.pointerId,
      item,
      handle,
      component,
      x: event.clientX,
      y: event.clientY,
      offsetX: event.clientX - rect.left,
      offsetY: event.clientY - rect.top,
    };
    handle.setPointerCapture?.(event.pointerId);
  }

  function dragTarget(x: number, y: number): HTMLElement | null {
    if (!pointer || x < 0 || y < 0 || x >= window.innerWidth || y >= window.innerHeight)
      return null;
    const target = document
      .elementFromPoint(x, y)
      ?.closest<HTMLElement>(pointer.item.type === "category" ? ".category" : ".rule-order-row");
    if (
      !target ||
      !target.isConnected ||
      (pointer.item.type === "rule" && target.dataset.categoryId !== pointer.item.categoryId)
    )
      return null;
    return target;
  }

  function isDropArea(x: number, y: number): boolean {
    if (!pointer || x < 0 || y < 0 || x >= window.innerWidth || y >= window.innerHeight)
      return false;
    const element = document.elementFromPoint(x, y);
    if (pointer.item.type === "category") return Boolean(element?.closest(".category-list"));
    const grid = element?.closest(".rules-grid");
    return grid?.closest<HTMLElement>(".category")?.dataset.dragId === pointer.item.categoryId;
  }

  function movePointerDrag(event: PointerEvent) {
    if (!pointer || event.pointerId !== pointer.id) return;
    if (!dragging) {
      if (Math.hypot(event.clientX - pointer.x, event.clientY - pointer.y) < DRAG_THRESHOLD_PX)
        return;
      const { item, component } = pointer;
      const ids =
        item.type === "category"
          ? [...categories.map((category) => category.id), UNCATEGORIZED]
          : ruleStatuses.map((status) => status.rule.id);
      const saved = item.type === "category" ? categoryOrder : ruleOrder;
      previewOrder = [...orderedIds(ids, saved), ...saved.filter((id) => !ids.includes(id))];
      const rect = component.getBoundingClientRect();
      dragPreview = document.createElement("div");
      dragPreview.className =
        item.type === "rule" ? "rule-drag-preview rule-drag-preview--rule" : "rule-drag-preview";
      dragPreview.setAttribute("aria-hidden", "true");
      dragPreview.setAttribute("inert", "");
      dragPreview.style.width = `${rect.width}px`;
      dragPreview.style.setProperty(
        "--category-color",
        getComputedStyle(component).getPropertyValue("--category-color")
      );
      const clone = component.cloneNode(true) as HTMLElement;
      clone.removeAttribute("id");
      clone.querySelectorAll("[id]").forEach((node) => node.removeAttribute("id"));
      dragPreview.appendChild(clone);
      document.body.appendChild(dragPreview);
      document.documentElement.classList.add("rules-dragging");
      dragging = item;
    }
    event.preventDefault();
    if (dragPreview) {
      dragPreview.style.left = `${event.clientX - pointer.offsetX}px`;
      dragPreview.style.top = `${event.clientY - pointer.offsetY}px`;
    }
    const target = dragTarget(event.clientX, event.clientY);
    const id = target?.dataset.dragId;
    if (id && id !== dragging.id && target) {
      const rect = target.getBoundingClientRect();
      const after = event.clientY >= rect.top + rect.height / 2;
      const marker = `${id}:${after}`;
      if (marker !== lastDragTarget) {
        lastDragTarget = marker;
        const remaining = previewOrder.filter((entry) => entry !== dragging?.id);
        const index = remaining.indexOf(id);
        if (index >= 0) {
          remaining.splice(index + (after ? 1 : 0), 0, dragging.id);
          previewOrder = remaining;
        }
      }
    }
  }

  function finishDrag(commit = false) {
    if (commit && dragging) {
      const order = [...previewOrder];
      if (dragging.type === "category") {
        categoryOrder = order;
        persistOrder(categoryOrderKey, order);
      } else {
        ruleOrder = order;
        persistOrder(ruleOrderKey, order);
      }
    }
    const current = pointer;
    pointer = null;
    if (current?.handle.hasPointerCapture?.(current.id))
      current.handle.releasePointerCapture(current.id);
    dragPreview?.remove();
    dragPreview = null;
    document.documentElement.classList.remove("rules-dragging");
    dragging = null;
    previewOrder = [];
    lastDragTarget = "";
  }

  function endPointerDrag(event: PointerEvent) {
    if (!pointer || event.pointerId !== pointer.id) return;
    finishDrag(Boolean(dragging && isDropArea(event.clientX, event.clientY)));
  }

  function cancelPointerDrag(event: PointerEvent) {
    if (pointer?.id === event.pointerId) finishDrag();
  }

  function handleDragKeydown(event: KeyboardEvent) {
    if (pointer && event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      finishDrag();
    }
  }

  onDestroy(() => finishDrag());

  function moveWithKeyboard(event: KeyboardEvent, item: DragItem, ids: string[]) {
    if (filtering || !["ArrowUp", "ArrowDown"].includes(event.key)) return;
    const index = ids.indexOf(item.id);
    const target = ids[index + (event.key === "ArrowUp" ? -1 : 1)];
    if (!target) return;
    event.preventDefault();
    reorder(item, target);
  }

  $effect(() => {
    const key = categoryStorageKey;
    categoryOrder = readOrder(categoryOrderKey);
    ruleOrder = readOrder(ruleOrderKey);
    try {
      const saved: unknown = JSON.parse(localStorage.getItem(key) ?? "[]");
      collapsed = Array.isArray(saved)
        ? saved.filter((id): id is string => typeof id === "string")
        : [];
    } catch {
      collapsed = [];
    }
  });

  function toggleCategory(id: string) {
    collapsed = collapsed.includes(id)
      ? collapsed.filter((item) => item !== id)
      : [...collapsed, id];
    try {
      localStorage.setItem(categoryStorageKey, JSON.stringify(collapsed));
    } catch {
      // The list remains usable when local storage is unavailable.
    }
  }

  function requiresAttention(status: rulesync.RuleStatus): boolean {
    if (!status.rule.enabled) return false;
    return status.destinations.some((dest) => {
      if (["blocked_collision", "location_unmapped", "source_missing"].includes(dest.state))
        return true;
      const ruleDest = status.rule.destinations.find((item) => item.id === dest.destId);
      return (dest.files ?? []).some((file) => needsReview(file.status, status.rule, ruleDest));
    });
  }
  const visibleStatuses = $derived(
    ruleStatuses.filter((status) =>
      matchesRuleQuery(status, query, {
        origin: t(`rules.origin.${status.rule.source.origin}`),
        subscription: subscriptionName(status.rule.source.subscription),
      })
    )
  );
  const filtering = $derived(query.trim() !== "");
  const groups = $derived.by(() => {
    const fallback = {
      id: UNCATEGORIZED,
      name: "",
      color: "gray",
      scope: "builtin",
    } as rules.Category;
    const definitions = [
      ...categories.filter((category) => category.id !== UNCATEGORIZED),
      fallback,
    ];
    const sorted = orderedIds(
      definitions.map((category) => category.id),
      categoryOrder
    ).map((id) => definitions.find((category) => category.id === id)!);
    return sorted
      .map((category) => {
        const statuses = visibleStatuses.filter((status) => {
          const id = status.rule.categoryId || UNCATEGORIZED;
          return category.id === UNCATEGORIZED
            ? id === UNCATEGORIZED || !categories.some((item) => item.id === id)
            : id === category.id;
        });
        const sorted = orderedIds(
          statuses.map((status) => status.rule.id),
          ruleOrder
        ).map((id) => statuses.find((status) => status.rule.id === id)!);
        return { category, statuses: sorted, attention: statuses.some(requiresAttention) };
      })
      .filter((group) => group.statuses.length > 0);
  });
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
      await api.migrateRuleCategories();
      categories = await api.getRuleCategories();
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

<svelte:window
  onpointermove={movePointerDrag}
  onpointerup={endPointerDrag}
  onpointercancel={cancelPointerDrag}
  onkeydown={handleDragKeydown}
  onblur={() => finishDrag()}
/>

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
    <div class="category-list">
      {#each groups as group (group.category.id)}
        {@const expanded = filtering || !collapsed.includes(group.category.id)}
        {@const Icon = categoryIcon(group.category.icon, group.category.id)}
        <section
          class="category"
          class:drag-source={dragging?.type === "category" && dragging.id === group.category.id}
          aria-label={categoryLabel(group.category)}
          style:--category-color={categoryColor(group.category.color)}
          style:order={dragging?.type === "category"
            ? previewOrder.indexOf(group.category.id)
            : undefined}
          data-drag-id={group.category.id}
        >
          <div class="category-bar">
            <button
              class="drag-handle"
              type="button"
              disabled={filtering}
              aria-label={t("rules.categories.reorder", { name: categoryLabel(group.category) })}
              title={t("rules.categories.reorder", { name: categoryLabel(group.category) })}
              onlostpointercapture={cancelPointerDrag}
              onpointerdown={(event) =>
                startPointerDrag(event, {
                  type: "category",
                  id: group.category.id,
                  categoryId: group.category.id,
                })}
              onkeydown={(event) =>
                moveWithKeyboard(
                  event,
                  { type: "category", id: group.category.id, categoryId: group.category.id },
                  groups.map((item) => item.category.id)
                )}
            >
              <GripVertical size={16} aria-hidden="true" />
            </button>
            <button
              class="category-heading"
              type="button"
              aria-expanded={expanded}
              onclick={() => toggleCategory(group.category.id)}
            >
              <span class="category-chevron" class:category-expanded={expanded}
                ><ChevronRight size={16} aria-hidden="true" /></span
              >
              <Icon size={16} class="category-icon" aria-hidden="true" />
              <span class="category-name">{categoryLabel(group.category)}</span>
              <span class="category-count"
                >{t(`rules.categories.count_${group.statuses.length === 1 ? "one" : "other"}`, {
                  count: group.statuses.length,
                })}</span
              >
              {#if group.attention}<span class="category-attention"
                  >{t("rules.categories.attention")}</span
                >{/if}
            </button>
          </div>
          {#if expanded}
            <div class="rules-grid">
              {#each group.statuses as status (status.rule.id)}
                <div
                  class="rule-order-row"
                  class:drag-source={dragging?.type === "rule" && dragging.id === status.rule.id}
                  style:order={dragging?.type === "rule"
                    ? previewOrder.indexOf(status.rule.id)
                    : undefined}
                  data-drag-id={status.rule.id}
                  data-category-id={group.category.id}
                >
                  <button
                    class="drag-handle rule-drag-handle"
                    type="button"
                    disabled={filtering}
                    aria-label={t("rules.categories.reorder_rule", { name: status.rule.name })}
                    title={t("rules.categories.reorder_rule", { name: status.rule.name })}
                    onlostpointercapture={cancelPointerDrag}
                    onpointerdown={(event) =>
                      startPointerDrag(event, {
                        type: "rule",
                        id: status.rule.id,
                        categoryId: group.category.id,
                      })}
                    onkeydown={(event) =>
                      moveWithKeyboard(
                        event,
                        { type: "rule", id: status.rule.id, categoryId: group.category.id },
                        group.statuses.map((item) => item.rule.id)
                      )}
                  >
                    <GripVertical size={16} aria-hidden="true" />
                  </button>
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
                </div>
              {/each}
            </div>
          {/if}
        </section>
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

  .category-list,
  .category {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    min-width: 0;
  }

  .category,
  :global(.rule-drag-preview) {
    --category-tint: color-mix(in srgb, var(--category-color) 8%, transparent);
  }
  .category {
    padding: var(--space-2);
    border-radius: var(--radius-lg);
    background: var(--category-tint);
  }
  .drag-source {
    visibility: hidden;
  }
  :global(html.rules-dragging),
  :global(html.rules-dragging *) {
    cursor: none !important;
  }
  :global(.rule-drag-preview) {
    position: fixed;
    z-index: 9999;
    pointer-events: none;
    opacity: 1;
    background-color: var(--canvas);
    border-radius: var(--radius-lg);
  }
  :global(.rule-drag-preview--rule) {
    background-image: linear-gradient(var(--category-tint), var(--category-tint));
  }
  :global(.rule-drag-preview > *) {
    box-sizing: border-box;
    width: 100%;
  }

  .category-bar {
    display: flex;
    align-items: center;
    min-width: 0;
    min-height: var(--card-zone-row);
    border-radius: var(--radius-md);
    background: color-mix(in srgb, var(--category-color) 18%, var(--surface));
  }
  .category-heading {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: var(--card-zone-row);
    padding: 0 var(--space-3) 0 0;
    border: none;
    border-radius: var(--radius-md);
    background: transparent;
    color: var(--ink);
    cursor: pointer;
    text-align: left;
  }
  .category-heading:hover {
    background: color-mix(in srgb, var(--ink) 4%, transparent);
  }
  .category-heading:focus-visible,
  .drag-handle:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }
  .category-chevron {
    display: inline-flex;
    flex-shrink: 0;
    transition: transform var(--duration-fast) var(--easing-out);
  }
  .category-chevron.category-expanded {
    transform: rotate(90deg);
  }
  .category-heading :global(.category-icon) {
    color: var(--category-color);
    flex-shrink: 0;
  }
  .drag-handle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex: 0 0 var(--card-zone-icon);
    width: var(--card-zone-icon);
    min-height: var(--card-zone-icon);
    padding: 0;
    border: 0;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--text-muted);
    cursor: grab;
    touch-action: none;
  }
  .drag-handle:active {
    cursor: grabbing;
  }
  .drag-handle:disabled {
    opacity: 0.4;
    cursor: default;
  }
  .rule-order-row {
    display: flex;
    align-items: flex-start;
    min-width: 0;
  }
  .rule-order-row > :global(article) {
    flex: 1;
    min-width: 0;
  }
  .rule-drag-handle {
    align-self: center;
  }
  .category-name {
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .category-count {
    color: var(--text-muted);
    font-size: var(--text-xs);
    white-space: nowrap;
  }
  .category-attention {
    margin-left: auto;
    color: var(--dot-conflict);
    font-size: var(--text-xs);
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
