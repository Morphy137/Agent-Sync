<script lang="ts">
  import { untrack } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import { BellRing, Link2, ListFilter, Plus, RefreshCw, Rss } from "@lucide/svelte";
  import type { service, subscriptions } from "../../../wailsjs/go/models";
  import { api } from "../api";
  import type { FileDiff } from "../diff/types";
  import { t } from "../i18n";
  import { appState } from "../stores/appState.svelte";
  import Button from "../components/Button.svelte";
  import IconButton from "../components/IconButton.svelte";
  import Modal from "../components/Modal.svelte";
  import Spinner from "../components/Spinner.svelte";
  import DiffModal from "../components/DiffModal.svelte";
  import SubscriptionCard from "../components/subscriptions/SubscriptionCard.svelte";
  import AddSubscriptionModal from "../components/subscriptions/AddSubscriptionModal.svelte";
  import SubscriptionTreeDrawer from "../components/subscriptions/SubscriptionTreeDrawer.svelte";
  import { reportError } from "../components/remote/reportError";

  interface IncomingDiffState {
    open: boolean;
    loading: boolean;
    diff: FileDiff | null;
    path: string;
    subscriptionName: string;
  }

  let items = $state<service.SubscriptionInfo[]>([]);
  let loaded = $state(false);
  let addOpen = $state(false);
  let browseId = $state<string | null>(null);
  let removeTarget = $state<service.SubscriptionInfo | null>(null);
  let removing = $state(false);
  let checkingAll = $state(false);
  let incomingDiff = $state<IncomingDiffState>({
    open: false,
    loading: false,
    diff: null,
    path: "",
    subscriptionName: "",
  });
  const checking = new SvelteSet<string>();
  const updating = new SvelteSet<string>();
  let loadRequest = 0;
  let diffRequest = 0;

  const browseSubscription = $derived(items.find((item) => item.id === browseId) ?? null);
  const pendingUpdates = $derived(
    items.filter((item) => item.update && !item.update.offline && item.update.behind > 0).length
  );

  $effect(() => {
    void appState.rulesVersion;
    untrack(() => load());
  });

  async function load() {
    const request = ++loadRequest;
    try {
      const result = await api.listSubscriptions();
      if (request === loadRequest) items = result ?? [];
    } catch (err) {
      if (request === loadRequest) reportError(err);
    } finally {
      if (request === loadRequest) loaded = true;
    }
  }

  function patchItem(id: string, patch: Partial<service.SubscriptionInfo>) {
    items = items.map((item) => (item.id === id ? Object.assign({}, item, patch) : item));
  }

  function toastCheckResult(item: service.SubscriptionInfo, info: subscriptions.UpdateInfo) {
    if (info.offline) {
      appState.addToast("info", t("subscriptions.toasts.offline", { name: item.name }));
    } else if (info.behind > 0) {
      appState.addToast(
        "info",
        t("subscriptions.toasts.updates_available", { name: item.name, count: info.behind })
      );
    } else {
      appState.addToast("info", t("subscriptions.toasts.up_to_date", { name: item.name }));
    }
  }

  async function checkOne(item: service.SubscriptionInfo, notify: boolean) {
    checking.add(item.id);
    try {
      const info = await api.checkSubscription(item.id);
      patchItem(item.id, { update: info });
      if (notify) toastCheckResult(item, info);
    } catch (err) {
      reportError(err);
    } finally {
      checking.delete(item.id);
    }
  }

  async function handleCheck(item: service.SubscriptionInfo) {
    await checkOne(item, true);
    await load();
  }

  async function handleCheckAll() {
    if (checkingAll) return;
    checkingAll = true;
    try {
      for (const item of items) {
        await checkOne(item, false);
      }
      await load();
    } finally {
      checkingAll = false;
    }
  }

  async function handleUpdate(item: service.SubscriptionInfo) {
    updating.add(item.id);
    try {
      const result = await api.updateSubscription(item.id);
      const changed = result.changedFiles?.length ?? 0;
      if (changed === 0) {
        appState.addToast(
          "success",
          t("subscriptions.toasts.already_current", { name: item.name })
        );
      } else {
        appState.addToast(
          "success",
          t("subscriptions.toasts.updated", {
            name: item.name,
            files: changed,
            written: result.applied?.written ?? 0,
          })
        );
      }
      const skipped = result.applied?.skipped ?? 0;
      if (skipped > 0) {
        appState.addToast("info", t("subscriptions.toasts.skipped", { count: skipped }));
      }
      await load();
    } catch (err) {
      reportError(err);
    } finally {
      updating.delete(item.id);
    }
  }

  async function handleAutoUpdate(item: service.SubscriptionInfo, enabled: boolean) {
    const previous = item.autoUpdate;
    patchItem(item.id, { autoUpdate: enabled });
    try {
      appState.config = await api.setSubscriptionAutoUpdate(item.id, enabled);
      appState.addToast(
        "success",
        enabled
          ? t("subscriptions.toasts.auto_on", { name: item.name })
          : t("subscriptions.toasts.auto_off", { name: item.name })
      );
    } catch (err) {
      patchItem(item.id, { autoUpdate: previous });
      reportError(err);
    }
  }

  async function confirmRemove() {
    const target = removeTarget;
    if (!target || removing) return;
    removing = true;
    try {
      await api.removeSubscription(target.id);
      appState.addToast("success", t("subscriptions.toasts.removed", { name: target.name }));
      removeTarget = null;
      if (browseId === target.id) browseId = null;
      await load();
    } catch (err) {
      reportError(err);
    } finally {
      removing = false;
    }
  }

  async function openIncomingDiff(item: service.SubscriptionInfo, path: string) {
    const request = ++diffRequest;
    incomingDiff = { open: true, loading: true, diff: null, path, subscriptionName: item.name };
    try {
      const diff = await api.getSubscriptionIncomingDiff(item.id, path);
      if (request === diffRequest) incomingDiff = { ...incomingDiff, diff, loading: false };
    } catch (err) {
      if (request === diffRequest) {
        closeIncomingDiff();
        reportError(err);
      }
    }
  }

  function closeIncomingDiff() {
    diffRequest++;
    incomingDiff = { ...incomingDiff, open: false, loading: false, diff: null };
  }

  function handleAdded(subscription: { id: string }) {
    addOpen = false;
    void load();
    browseId = subscription.id;
  }
</script>

<section class="view" aria-labelledby="subscriptions-title">
  <header class="view-header">
    <div class="title-group">
      <h2 id="subscriptions-title" class="section-title">{t("subscriptions.title")}</h2>
      {#if items.length > 0}
        <span class="section-summary">
          {pendingUpdates > 0
            ? t("subscriptions.summary_updates", { count: items.length, updates: pendingUpdates })
            : t("subscriptions.summary", { count: items.length })}
        </span>
      {/if}
    </div>
    <div class="header-actions">
      {#if items.length > 0}
        <IconButton
          label={t("subscriptions.check_all")}
          disabled={checkingAll}
          onclick={handleCheckAll}
        >
          <RefreshCw size={16} class={checkingAll ? "spin" : ""} />
        </IconButton>
      {/if}
      <Button variant="primary" size="sm" onclick={() => (addOpen = true)}>
        <Plus size={16} />
        <span>{t("subscriptions.add_button")}</span>
      </Button>
    </div>
  </header>

  {#if !loaded}
    <div class="loading" role="status">
      <Spinner size="sm" />
      <span>{t("subscriptions.loading")}</span>
    </div>
  {:else if items.length === 0}
    <div class="empty">
      <div class="empty-icon" aria-hidden="true"><Rss size={28} /></div>
      <h3 class="empty-title">{t("subscriptions.empty.title")}</h3>
      <p class="empty-desc">{t("subscriptions.empty.description")}</p>
      <ol class="steps">
        <li class="step">
          <span class="step-icon" aria-hidden="true"><Link2 size={16} /></span>
          <div>
            <p class="step-title">{t("subscriptions.empty.step_share_title")}</p>
            <p class="step-desc">{t("subscriptions.empty.step_share_desc")}</p>
          </div>
        </li>
        <li class="step">
          <span class="step-icon" aria-hidden="true"><ListFilter size={16} /></span>
          <div>
            <p class="step-title">{t("subscriptions.empty.step_pick_title")}</p>
            <p class="step-desc">{t("subscriptions.empty.step_pick_desc")}</p>
          </div>
        </li>
        <li class="step">
          <span class="step-icon" aria-hidden="true"><BellRing size={16} /></span>
          <div>
            <p class="step-title">{t("subscriptions.empty.step_update_title")}</p>
            <p class="step-desc">{t("subscriptions.empty.step_update_desc")}</p>
          </div>
        </li>
      </ol>
      <Button variant="primary" onclick={() => (addOpen = true)}>
        <Plus size={16} />
        <span>{t("subscriptions.add_button")}</span>
      </Button>
    </div>
  {:else}
    <p class="view-intro">{t("subscriptions.intro")}</p>
    <div class="grid">
      {#each items as item (item.id)}
        <SubscriptionCard
          subscription={item}
          checking={checking.has(item.id)}
          updating={updating.has(item.id)}
          oncheck={() => handleCheck(item)}
          onupdate={() => handleUpdate(item)}
          onautoupdate={(enabled) => handleAutoUpdate(item, enabled)}
          onbrowse={() => (browseId = item.id)}
          onremove={() => (removeTarget = item)}
          onviewdiff={(path) => openIncomingDiff(item, path)}
        />
      {/each}
    </div>
  {/if}
</section>

<AddSubscriptionModal open={addOpen} onclose={() => (addOpen = false)} onadded={handleAdded} />

<SubscriptionTreeDrawer
  open={browseSubscription !== null}
  subscription={browseSubscription}
  onclose={() => (browseId = null)}
/>

<Modal
  open={removeTarget !== null}
  title={t("subscriptions.remove.title", { name: removeTarget?.name ?? "" })}
  onclose={() => (removeTarget = null)}
>
  <div class="remove-body">
    <p class="remove-text">{t("subscriptions.remove.message")}</p>
    {#if removeTarget && removeTarget.ruleCount > 0}
      <p class="remove-rules">
        {removeTarget.ruleCount === 1
          ? t("subscriptions.remove.rules_one")
          : t("subscriptions.remove.rules_other", { count: removeTarget.ruleCount })}
      </p>
    {/if}
    <p class="remove-text">{t("subscriptions.remove.files_kept")}</p>
    <div class="remove-actions">
      <Button variant="secondary" onclick={() => (removeTarget = null)}>
        {t("common.cancel")}
      </Button>
      <Button variant="primary" loading={removing} onclick={confirmRemove}>
        {t("subscriptions.remove.confirm")}
      </Button>
    </div>
  </div>
</Modal>

<DiffModal
  open={incomingDiff.open}
  title={t("subscriptions.diff.title", { name: incomingDiff.subscriptionName })}
  subtitle={incomingDiff.path}
  diff={incomingDiff.diff}
  loading={incomingDiff.loading}
  actions={[{ label: t("common.close"), onclick: closeIncomingDiff }]}
  onclose={closeIncomingDiff}
/>

<style>
  .view {
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
    font-size: var(--text-sm);
    color: var(--text-muted);
    line-height: 1.55;
  }

  .loading {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-sm);
    color: var(--text-muted);
    padding: var(--space-6) 0;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 380px), 1fr));
    gap: var(--space-4);
  }

  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: var(--space-3);
    padding: var(--space-8) var(--space-6);
    background-color: var(--surface);
    border: 1px dashed var(--border-subtle, var(--hairline));
    border-radius: var(--radius-xl);
  }

  .empty-icon {
    width: 56px;
    height: 56px;
    border-radius: var(--radius-full);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
  }

  .empty-title {
    margin: 0;
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .empty-desc {
    margin: 0;
    max-width: 480px;
    font-size: var(--text-sm);
    color: var(--text-muted);
    line-height: 1.55;
  }

  .steps {
    list-style: none;
    margin: var(--space-2) 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: var(--space-3);
    width: 100%;
    max-width: 760px;
    text-align: left;
  }

  .step {
    display: flex;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .step-icon {
    width: 30px;
    height: 30px;
    flex-shrink: 0;
    border-radius: var(--radius-sm);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    color: var(--ink);
  }

  .step-title {
    margin: 0;
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .step-desc {
    margin: 2px 0 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.5;
  }

  .remove-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .remove-text,
  .remove-rules {
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
    line-height: 1.55;
  }

  .remove-rules {
    color: var(--ink);
    font-weight: var(--weight-medium);
  }

  .remove-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-top: var(--space-2);
  }
</style>
