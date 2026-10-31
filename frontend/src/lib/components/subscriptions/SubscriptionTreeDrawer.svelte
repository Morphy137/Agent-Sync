<script lang="ts">
  import { untrack } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import { Check, ChevronDown, ChevronRight, Lock, X } from "@lucide/svelte";
  import type { service } from "../../../../wailsjs/go/models";
  import { api } from "../../api";
  import { t } from "../../i18n";
  import { appState } from "../../stores/appState.svelte";
  import Button from "../Button.svelte";
  import IconButton from "../IconButton.svelte";
  import Input from "../Input.svelte";
  import Spinner from "../Spinner.svelte";
  import { reportError } from "../remote/reportError";
  import { buildTree, countNodes, filterTree, resolveKind, type TreeNode } from "./tree";
  import { kindIcon, kindLabel, syncActionLabel } from "./kinds";

  interface Props {
    open: boolean;
    subscription: service.SubscriptionInfo | null;
    onclose: () => void;
  }

  let { open, subscription, onclose }: Props = $props();

  const SUBSCRIPTION_ORIGIN = "subscription";
  const LOCAL_SCOPE = "local";

  let entries = $state<service.SourceTreeEntry[]>([]);
  let loading = $state(false);
  let query = $state("");
  let selectedPath = $state<string | null>(null);
  let panel = $state<HTMLDivElement | null>(null);
  const expanded = new SvelteSet<string>();
  const picked = new SvelteSet<string>();
  let requestId = 0;

  const tree = $derived(buildTree(entries));
  const visibleTree = $derived(filterTree(tree, query));
  const filtering = $derived(query.trim().length > 0);
  const totalCount = $derived(countNodes(tree));
  const selectedNode = $derived(findNode(tree, selectedPath));
  const subscriptionId = $derived(open ? (subscription?.id ?? null) : null);

  $effect(() => {
    const id = subscriptionId;
    void appState.rulesVersion;
    if (!id) return;
    untrack(() => loadTree(id));
  });

  $effect(() => {
    if (!subscriptionId) return;
    untrack(() => {
      query = "";
      selectedPath = null;
      expanded.clear();
      picked.clear();
    });
  });

  $effect(() => {
    const element = panel;
    if (!element) return;
    element.focus();
    const handleKeydown = (event: KeyboardEvent) => {
      // While the rule editor is on top, Escape belongs to it.
      if (event.key !== "Escape" || appState.ruleEditor.open) return;
      event.stopPropagation();
      onclose();
    };
    element.addEventListener("keydown", handleKeydown);
    return () => element.removeEventListener("keydown", handleKeydown);
  });

  async function loadTree(id: string) {
    const request = ++requestId;
    loading = true;
    try {
      const result = await api.getSubscriptionTree(id);
      if (request === requestId) entries = result ?? [];
    } catch (err) {
      if (request === requestId) {
        entries = [];
        reportError(err);
      }
    } finally {
      if (request === requestId) loading = false;
    }
  }

  function findNode(nodes: TreeNode[], path: string | null): TreeNode | null {
    if (!path) return null;
    for (const node of nodes) {
      if (node.path === path) return node;
      if (path.startsWith(`${node.path}/`)) return findNode(node.children, path);
    }
    return null;
  }

  function isExpanded(path: string): boolean {
    return filtering || expanded.has(path);
  }

  function toggle(path: string) {
    if (expanded.has(path)) expanded.delete(path);
    else expanded.add(path);
  }

  function handleNodeKeydown(event: KeyboardEvent, node: TreeNode) {
    if (!node.isDir || filtering) return;
    if (event.key === "ArrowRight" && !expanded.has(node.path)) {
      event.preventDefault();
      expanded.add(node.path);
    } else if (event.key === "ArrowLeft" && expanded.has(node.path)) {
      event.preventDefault();
      expanded.delete(node.path);
    }
  }

  function syncSelected() {
    if (!subscription || !selectedNode) return;
    appState.openRuleEditor({
      presetSource: {
        origin: SUBSCRIPTION_ORIGIN,
        subscription: subscription.id,
        path: selectedNode.path,
      },
      scope: LOCAL_SCOPE,
    });
    picked.add(selectedNode.path);
  }

  function handleBackdropClick(event: MouseEvent) {
    if (event.target === event.currentTarget) onclose();
  }
</script>

{#snippet treeLevel(nodes: TreeNode[], depth: number)}
  {#each nodes as node (node.path)}
    {@const kind = resolveKind(node)}
    {@const KindIcon = kindIcon(kind)}
    {@const nodeOpen = node.isDir && isExpanded(node.path)}
    <li class="node">
      <div
        class="node-row"
        class:selected={selectedPath === node.path}
        style:padding-left="calc({depth} * var(--space-4))"
      >
        {#if node.isDir && node.children.length > 0 && !filtering}
          <button
            type="button"
            class="toggle"
            aria-expanded={nodeOpen}
            aria-label={nodeOpen
              ? t("subscriptions.tree.collapse", { name: node.name })
              : t("subscriptions.tree.expand", { name: node.name })}
            onclick={() => toggle(node.path)}
          >
            {#if nodeOpen}
              <ChevronDown size={14} />
            {:else}
              <ChevronRight size={14} />
            {/if}
          </button>
        {:else}
          <span class="toggle-spacer" aria-hidden="true"></span>
        {/if}
        <button
          type="button"
          class="node-select"
          aria-pressed={selectedPath === node.path}
          onclick={() => (selectedPath = node.path)}
          onkeydown={(event) => handleNodeKeydown(event, node)}
        >
          <span class="node-icon kind-{kind}"><KindIcon size={14} /></span>
          <span class="node-name">{node.name}</span>
          {#if picked.has(node.path)}
            <span class="picked" title={t("subscriptions.tree.picked")}>
              <Check size={13} />
              <span class="sr-only">{t("subscriptions.tree.picked")}</span>
            </span>
          {/if}
          <span class="kind-badge kind-{kind}">{kindLabel(kind)}</span>
        </button>
      </div>
      {#if nodeOpen && node.children.length > 0}
        <ul class="tree-level">
          {@render treeLevel(node.children, depth + 1)}
        </ul>
      {/if}
    </li>
  {/each}
{/snippet}

{#if open && subscription}
  <div class="backdrop" role="presentation" onclick={handleBackdropClick}>
    <div
      class="drawer"
      role="dialog"
      aria-modal="true"
      aria-labelledby="subscription-tree-title"
      tabindex="-1"
      bind:this={panel}
    >
      <header class="drawer-header">
        <div class="header-text">
          <h2 id="subscription-tree-title" class="drawer-title">
            {t("subscriptions.tree.title", { name: subscription.name })}
          </h2>
          <span class="header-sub">{subscription.url}</span>
        </div>
        <IconButton label={t("common.close")} onclick={onclose}>
          <X size={18} />
        </IconButton>
      </header>

      <div class="drawer-body">
        <p class="read-only-hint">
          <Lock size={14} />
          <span>{t("subscriptions.tree.read_only_hint")}</span>
        </p>

        <Input
          label={t("subscriptions.tree.filter_label")}
          placeholder={t("subscriptions.tree.filter_placeholder")}
          value={query}
          oninput={(value) => (query = value)}
        />

        {#if loading && entries.length === 0}
          <div class="state">
            <Spinner size="sm" />
            <span>{t("subscriptions.tree.loading")}</span>
          </div>
        {:else if totalCount === 0}
          <p class="state">{t("subscriptions.tree.empty")}</p>
        {:else if visibleTree.length === 0}
          <p class="state">{t("subscriptions.tree.no_matches")}</p>
        {:else}
          <p class="count">{t("subscriptions.tree.count", { count: totalCount })}</p>
          <ul class="tree-level root" aria-label={t("subscriptions.tree.aria_label")}>
            {@render treeLevel(visibleTree, 0)}
          </ul>
        {/if}
      </div>

      <footer class="selection-bar" aria-live="polite">
        {#if selectedNode}
          {@const selectedKind = resolveKind(selectedNode)}
          <div class="selection-info">
            <span class="selection-label">{t("subscriptions.tree.selected")}</span>
            <span class="selection-path" title={selectedNode.path}>{selectedNode.path}</span>
          </div>
          <Button variant="primary" size="sm" onclick={syncSelected}>
            {syncActionLabel(selectedKind)}
          </Button>
        {:else}
          <p class="selection-prompt">{t("subscriptions.tree.select_prompt")}</p>
        {/if}
      </footer>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background-color: var(--backdrop-overlay);
    display: flex;
    justify-content: flex-end;
    z-index: 90;
  }

  .drawer {
    background-color: var(--surface);
    width: 100%;
    max-width: 480px;
    height: 100%;
    display: flex;
    flex-direction: column;
    box-shadow: var(--elev-3);
    border-top-left-radius: var(--radius-xl);
    border-bottom-left-radius: var(--radius-xl);
    animation: drawerEnter var(--duration-normal) var(--easing-out);
    outline: none;
  }

  @keyframes drawerEnter {
    from {
      opacity: 0;
      transform: translateX(16px);
    }
    to {
      opacity: 1;
      transform: translateX(0);
    }
  }

  .drawer-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-6);
    border-bottom: 1px solid var(--hairline);
  }

  .header-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .drawer-title {
    margin: 0;
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
  }

  .header-sub {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .drawer-body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: var(--space-5);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .read-only-hint {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    margin: 0;
    padding: var(--space-3);
    border-radius: var(--radius-md);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    font-size: var(--text-xs);
    color: var(--text-muted);
    line-height: 1.5;
  }

  .read-only-hint :global(svg) {
    flex-shrink: 0;
    margin-top: 1px;
  }

  .state {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    font-size: var(--text-sm);
    color: var(--text-muted);
  }

  .count {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-subtle);
  }

  .tree-level {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .tree-level.root {
    margin-top: calc(-1 * var(--space-2));
  }

  .node-row {
    display: flex;
    align-items: center;
    gap: 2px;
    border-radius: var(--radius-sm);
  }

  .node-row.selected {
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
  }

  .toggle,
  .toggle-spacer {
    width: 22px;
    height: 22px;
    flex-shrink: 0;
  }

  .toggle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    border: none;
    border-radius: var(--radius-xs);
    background: none;
    color: var(--text-muted);
    cursor: pointer;
  }

  .toggle:hover {
    color: var(--ink);
  }

  .toggle:focus-visible,
  .node-select:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .node-select {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    height: 30px;
    padding: 0 var(--space-2);
    border: none;
    border-radius: var(--radius-sm);
    background: none;
    color: var(--ink);
    font-family: var(--font-sans);
    font-size: var(--text-sm);
    text-align: left;
    cursor: pointer;
  }

  .node-row:not(.selected) .node-select:hover {
    background-color: var(--surface-sunken);
  }

  .node-icon {
    display: inline-flex;
    flex-shrink: 0;
    color: var(--text-muted);
  }

  .node-name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .picked {
    display: inline-flex;
    color: var(--dot-ok);
  }

  .kind-badge {
    flex-shrink: 0;
    font-size: 11px;
    font-weight: var(--weight-medium);
    padding: 1px var(--space-2);
    border-radius: var(--radius-xs);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    color: var(--text-subtle);
  }

  .kind-badge.kind-skill,
  .node-icon.kind-skill {
    color: var(--dot-up);
  }

  .kind-badge.kind-mcp,
  .node-icon.kind-mcp {
    color: var(--dot-down);
  }

  .kind-badge.kind-instructions,
  .node-icon.kind-instructions {
    color: var(--dot-ok);
  }

  .selection-bar {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-6);
    border-top: 1px solid var(--hairline);
    min-height: 64px;
  }

  .selection-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .selection-label {
    font-size: 11px;
    color: var(--text-subtle);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .selection-path {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .selection-prompt {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
  }

  @media (max-width: 520px) {
    .drawer {
      border-radius: 0;
    }

    .drawer-header,
    .selection-bar {
      padding-left: var(--space-4);
      padding-right: var(--space-4);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .drawer {
      animation: none;
    }
  }
</style>
