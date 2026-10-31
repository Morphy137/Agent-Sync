<script lang="ts">
  import { untrack } from "svelte";
  import { ChevronDown, ChevronRight, Files, Folder, FolderInput } from "@lucide/svelte";
  import { rules, type service } from "../../../../wailsjs/go/models";
  import { api } from "../../api";
  import { t } from "../../i18n";
  import { appState } from "../../stores/appState.svelte";
  import Button from "../Button.svelte";
  import Input from "../Input.svelte";
  import SegmentedControl from "../SegmentedControl.svelte";
  import Select from "../Select.svelte";
  import Switch from "../Switch.svelte";
  import SourceTree from "./SourceTree.svelte";
  import SourceSelection from "./SourceSelection.svelte";
  import { ROOT_SOURCE_PATH, selectionFromPaths, type SourceOrigin } from "./ruleUi";

  interface Props {
    value: rules.Source | null;
    hasRoot: boolean;
    /** Whether the current single source is a folder (from detection). */
    sourceIsFolder?: boolean;
    onchange: (source: rules.Source) => void;
  }

  let { value, hasRoot, sourceIsFolder = false, onchange }: Props = $props();

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
  let importOpen = $state(false);
  /** Multi-select mode for the repository and subscription trees. */
  let multi = $state(Boolean(initialSelection()));

  function initialSelection(): boolean {
    return Boolean(value?.include?.length);
  }

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

  function emit(origin: SourceOrigin, path: string, subscription?: string, include?: string[]) {
    onchange(
      rules.Source.createFrom({
        origin,
        path,
        ...(subscription ? { subscription } : {}),
        ...(include?.length ? { include } : {}),
      })
    );
  }

  const isSelection = $derived(Boolean(value?.include?.length));

  /** Full paths of the selected files: absolute for local sources, relative otherwise. */
  const selectedPaths = $derived.by((): string[] => {
    if (!value?.path) return [];
    if (!value.include?.length) return [value.path];
    if (value.origin !== "local" && value.path === ROOT_SOURCE_PATH) return [...value.include];
    const backslash = value.origin === "local" && value.path.includes("\\");
    const sep = backslash ? "\\" : "/";
    const base = value.path.replace(/[\\/]+$/, "");
    return value.include.map((rel) => base + sep + (backslash ? rel.replaceAll("/", sep) : rel));
  });

  /** One file stays a plain source; several become a folder plus a selection. */
  function emitPaths(origin: SourceOrigin, paths: string[], subscription?: string) {
    if (paths.length === 0) return;
    if (paths.length === 1) {
      emit(origin, paths[0], subscription);
      return;
    }
    const selection = selectionFromPaths(paths, origin === "local");
    if (!selection) {
      appState.addToast("error", t("rules.source.selection_no_common_folder"));
      return;
    }
    emit(origin, selection.base, subscription, selection.include);
  }

  function toggleChecked(origin: SourceOrigin, path: string, subscription?: string) {
    const sameSource =
      value?.origin === origin &&
      (origin !== "subscription" || value.subscription === subscription);
    const current = sameSource ? selectedPaths : [];
    const next = current.includes(path) ? current.filter((p) => p !== path) : [...current, path];
    emitPaths(origin, next, subscription);
  }

  function removeFromSelection(index: number) {
    if (!value) return;
    const remaining = selectedPaths.filter((_, i) => i !== index);
    emitPaths(value.origin as SourceOrigin, remaining, value.subscription);
  }

  async function pickLocal(kind: "files" | "folder") {
    try {
      if (kind === "folder") {
        const folder = await api.selectFolder(t("rules.source.pick_folder_title"));
        if (folder) emit("local", folder);
        return;
      }
      emitPaths("local", await api.selectFiles(t("rules.source.pick_files_title")));
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
      importOpen = false;
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
  const repoChecked = $derived(value?.origin === "repo" ? selectedPaths : []);
  const subChecked = $derived(
    value?.origin === "subscription" && value.subscription === subscriptionId ? selectedPaths : []
  );
  const selectionBase = $derived.by(() => {
    if (!value?.path) return "";
    if (isSelection && value.path === ROOT_SOURCE_PATH) return t("rules.source.root_label");
    return value.path;
  });
  const displayItems = $derived(
    isSelection ? (value?.include ?? []) : value?.path ? [value.path] : []
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
      <Switch label={t("rules.source.multi_label")} checked={multi} onchange={(v) => (multi = v)} />
      <SourceTree
        entries={repoEntries}
        selected={selectedRepoPath}
        loading={repoLoading}
        emptyText={t("rules.source.repo_empty")}
        onselect={(entry) => emit("repo", entry.path)}
        {multi}
        checked={repoChecked}
        oncheck={(path) => toggleChecked("repo", path)}
      />
    {:else}
      <p class="tab-note">{t("rules.source.repo_unavailable")}</p>
    {/if}
  {:else if tab === "local"}
    <div class="local-tab">
      <p class="tab-note">{t("rules.source.local_hint")}</p>
      <div class="local-actions">
        <Button size="sm" onclick={() => pickLocal("files")}>
          <Files size={14} />
          <span>{t("rules.source.pick_files")}</span>
        </Button>
        <Button size="sm" onclick={() => pickLocal("folder")}>
          <Folder size={14} />
          <span>{t("rules.source.pick_folder")}</span>
        </Button>
      </div>
      {#if value?.origin === "local" && !isSelection && hasRoot}
        <div class="import-box">
          <button
            type="button"
            class="import-toggle"
            aria-expanded={importOpen}
            onclick={() => (importOpen = !importOpen)}
          >
            {#if importOpen}<ChevronDown size={14} />{:else}<ChevronRight size={14} />{/if}
            <span>{t("rules.source.import_title")}</span>
          </button>
          {#if importOpen}
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
          {/if}
        </div>
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
          <Switch
            label={t("rules.source.multi_label")}
            checked={multi}
            onchange={(v) => (multi = v)}
          />
          <SourceTree
            entries={subEntries}
            selected={selectedSubPath}
            loading={subLoading}
            emptyText={t("rules.source.subscription_empty")}
            onselect={(entry) => emit("subscription", entry.path, subscriptionId)}
            {multi}
            checked={subChecked}
            oncheck={(path) => toggleChecked("subscription", path, subscriptionId)}
          />
        {/if}
      {/if}
    </div>
  {/if}

  {#if value?.path && tab === value.origin}
    <SourceSelection
      base={selectionBase}
      items={displayItems}
      isFolder={!isSelection && sourceIsFolder}
      onremove={isSelection ? removeFromSelection : undefined}
    />
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

  .import-box {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .import-toggle {
    align-self: flex-start;
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 0;
    border: none;
    background: none;
    color: var(--text-muted);
    font-family: var(--font-sans);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    cursor: pointer;
  }

  .import-toggle:hover {
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
