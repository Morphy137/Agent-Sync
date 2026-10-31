<script lang="ts">
  import { untrack } from "svelte";
  import { FileText, Folder, FolderInput } from "@lucide/svelte";
  import { rules, type service } from "../../../../wailsjs/go/models";
  import { api } from "../../api";
  import { t } from "../../i18n";
  import { appState } from "../../stores/appState.svelte";
  import Button from "../Button.svelte";
  import Input from "../Input.svelte";
  import SegmentedControl from "../SegmentedControl.svelte";
  import Select from "../Select.svelte";
  import SourceTree from "./SourceTree.svelte";
  import type { SourceOrigin } from "./ruleUi";

  interface Props {
    value: rules.Source | null;
    hasRoot: boolean;
    onchange: (source: rules.Source) => void;
  }

  let { value, hasRoot, onchange }: Props = $props();

  function initialTab(): SourceOrigin {
    if (value?.origin === "local" || value?.origin === "subscription") return value.origin;
    return hasRoot ? "repo" : "local";
  }

  function initialSubscription(): string {
    return value?.origin === "subscription" ? (value.subscription ?? "") : "";
  }

  let tab = $state<SourceOrigin>(initialTab());
  let repoEntries = $state<service.SourceTreeEntry[]>([]);
  let repoLoaded = $state(false);
  let repoLoading = $state(false);
  let subscriptions = $state<service.SubscriptionInfo[]>([]);
  let subscriptionsLoaded = $state(false);
  let subscriptionId = $state(initialSubscription());
  let subEntries = $state<service.SourceTreeEntry[]>([]);
  let subLoading = $state(false);
  let importDir = $state("");
  let importing = $state(false);

  const tabOptions = $derived([
    { value: "repo", label: t("rules.source.tab_repo") },
    { value: "local", label: t("rules.source.tab_local") },
    { value: "subscription", label: t("rules.source.tab_subscription") },
  ]);

  const subscriptionOptions = $derived([
    { value: "", label: t("rules.source.subscription_placeholder") },
    ...subscriptions.map((s) => ({ value: s.id, label: s.name || s.url })),
  ]);

  // Load each tab's data lazily, the first time it is shown.
  $effect(() => {
    const current = tab;
    const canUseRepo = hasRoot;
    untrack(() => {
      if (current === "repo" && canUseRepo && !repoLoaded) void loadRepoTree();
      if (current === "subscription" && !subscriptionsLoaded) void loadSubscriptions();
    });
  });

  async function loadRepoTree() {
    repoLoaded = true;
    repoLoading = true;
    try {
      repoEntries = await api.listRepoTree();
    } catch (err) {
      appState.showError(err);
    } finally {
      repoLoading = false;
    }
  }

  async function loadSubscriptions() {
    subscriptionsLoaded = true;
    try {
      subscriptions = await api.listSubscriptions();
      if (subscriptionId) await loadSubscriptionTree(subscriptionId);
    } catch (err) {
      appState.showError(err);
    }
  }

  async function loadSubscriptionTree(id: string) {
    subEntries = [];
    if (!id) return;
    subLoading = true;
    try {
      subEntries = await api.getSubscriptionTree(id);
    } catch (err) {
      appState.showError(err);
    } finally {
      subLoading = false;
    }
  }

  function selectSubscription(id: string) {
    subscriptionId = id;
    void loadSubscriptionTree(id);
  }

  function emit(origin: SourceOrigin, path: string, subscription?: string) {
    onchange(rules.Source.createFrom({ origin, path, ...(subscription ? { subscription } : {}) }));
  }

  async function pickLocal(kind: "file" | "folder") {
    try {
      const picked =
        kind === "file"
          ? await api.selectFile(t("rules.source.pick_file_title"))
          : await api.selectFolder(t("rules.source.pick_folder_title"));
      if (picked) emit("local", picked);
    } catch (err) {
      appState.showError(err);
    }
  }

  async function importIntoRepo() {
    if (!value || value.origin !== "local" || importing) return;
    importing = true;
    try {
      const relPath = await api.importIntoRepo(value.path, importDir.trim());
      appState.addToast("success", t("rules.source.import_success", { path: relPath }));
      emit("repo", relPath);
      tab = "repo";
      await loadRepoTree();
    } catch (err) {
      appState.showError(err);
    } finally {
      importing = false;
    }
  }

  const selectedRepoPath = $derived(value?.origin === "repo" ? value.path : "");
  const selectedSubPath = $derived(
    value?.origin === "subscription" && value.subscription === subscriptionId ? value.path : ""
  );
</script>

<div class="source-picker">
  <SegmentedControl
    label={t("rules.source.tabs_label")}
    size="sm"
    options={tabOptions}
    value={tab}
    onchange={(v) => (tab = v as SourceOrigin)}
  />

  {#if tab === "repo"}
    {#if hasRoot}
      <SourceTree
        entries={repoEntries}
        selected={selectedRepoPath}
        loading={repoLoading}
        emptyText={t("rules.source.repo_empty")}
        onselect={(entry) => emit("repo", entry.path)}
      />
    {:else}
      <p class="tab-note">{t("rules.source.repo_unavailable")}</p>
    {/if}
  {:else if tab === "local"}
    <div class="local-tab">
      <p class="tab-note">{t("rules.source.local_hint")}</p>
      <div class="local-actions">
        <Button size="sm" onclick={() => pickLocal("file")}>
          <FileText size={14} />
          <span>{t("rules.source.pick_file")}</span>
        </Button>
        <Button size="sm" onclick={() => pickLocal("folder")}>
          <Folder size={14} />
          <span>{t("rules.source.pick_folder")}</span>
        </Button>
      </div>
      {#if value?.origin === "local"}
        <p class="picked-path">{value.path}</p>
        {#if hasRoot}
          <div class="import-box">
            <p class="import-title">{t("rules.source.import_title")}</p>
            <p class="tab-note">{t("rules.source.import_hint")}</p>
            <div class="import-row">
              <div class="import-input">
                <Input
                  label={t("rules.source.import_dir_label")}
                  value={importDir}
                  placeholder={t("rules.source.import_dir_placeholder")}
                  oninput={(v) => (importDir = v)}
                />
              </div>
              <Button size="sm" loading={importing} disabled={importing} onclick={importIntoRepo}>
                <FolderInput size={14} />
                <span>{t("rules.source.import_button")}</span>
              </Button>
            </div>
          </div>
        {/if}
      {/if}
    </div>
  {:else}
    <div class="subscription-tab">
      {#if subscriptionsLoaded && subscriptions.length === 0}
        <p class="tab-note">{t("rules.source.no_subscriptions")}</p>
      {:else}
        <Select
          label={t("rules.source.subscription_label")}
          value={subscriptionId}
          options={subscriptionOptions}
          onchange={(v) => selectSubscription(String(v))}
        />
        {#if subscriptionId}
          <SourceTree
            entries={subEntries}
            selected={selectedSubPath}
            loading={subLoading}
            emptyText={t("rules.source.subscription_empty")}
            onselect={(entry) => emit("subscription", entry.path, subscriptionId)}
          />
        {/if}
      {/if}
    </div>
  {/if}
</div>

<style>
  .source-picker {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .local-tab,
  .subscription-tab {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .tab-note {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .local-actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .picked-path {
    margin: 0;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    overflow-wrap: anywhere;
  }

  .import-box {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
  }

  .import-title {
    margin: 0;
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .import-row {
    display: flex;
    align-items: flex-end;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .import-input {
    flex: 1 1 200px;
    min-width: 0;
  }
</style>
