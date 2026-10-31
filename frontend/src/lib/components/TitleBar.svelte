<script lang="ts">
  import { onMount, type Snippet } from "svelte";
  import {
    Check,
    ArrowDown,
    ArrowUp,
    ArrowUpDown,
    TriangleAlert,
    CloudOff,
    Settings,
    RefreshCw,
    ChevronDown,
    Minus,
    Square,
    Copy,
    X,
  } from "@lucide/svelte";
  import {
    WindowMinimise,
    WindowToggleMaximise,
    WindowIsMaximised,
    Quit,
  } from "../../../wailsjs/runtime/runtime";
  import { appState } from "../stores/appState.svelte";
  import { t } from "../i18n";
  import Button from "./Button.svelte";
  import IconButton from "./IconButton.svelte";
  import Tooltip from "./Tooltip.svelte";

  interface Props {
    onsync: () => void;
    oncheckRemote: () => void;
    onopenSettings: () => void;
    onchangeRoot: () => void;
    /** Notification bell rendered next to the settings button. */
    notifications?: Snippet;
  }

  let { onsync, oncheckRemote, onopenSettings, onchangeRoot, notifications }: Props = $props();

  let isMaximised = $state(false);
  let isFocused = $state(true);

  async function updateMaximisedState() {
    try {
      isMaximised = await WindowIsMaximised();
    } catch {
      // ignore outside Wails runtime
    }
  }

  onMount(() => {
    updateMaximisedState();
  });

  function handleWindowResize() {
    updateMaximisedState();
  }

  function handleDoubleClick(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (target.closest("[data-no-drag]")) return;
    WindowToggleMaximise();
    setTimeout(updateMaximisedState, 60);
  }

  function handleMinimise() {
    WindowMinimise();
  }

  function handleToggleMaximise() {
    WindowToggleMaximise();
    setTimeout(updateMaximisedState, 60);
  }

  function handleClose() {
    Quit();
  }

  const direction = $derived(appState.statusPlan?.direction || "idle");
  const hasRuleWork = $derived((appState.statusPlan?.rulesPending ?? 0) > 0);
  const pendingCount = $derived.by(() => {
    if (!appState.statusPlan) return 0;
    const { downloads, uploads, conflicts } = appState.statusPlan;
    return (downloads?.length || 0) + (uploads?.length || 0) + (conflicts?.length || 0);
  });

  // Middle truncation for root path chip
  const rootTruncated = $derived.by(() => {
    const p = appState.config?.rootPath || "";
    if (p.length <= 36) return p;
    const head = p.slice(0, 16);
    const tail = p.slice(-17);
    return `${head}…${tail}`;
  });
</script>

<svelte:window
  onresize={handleWindowResize}
  onfocus={() => (isFocused = true)}
  onblur={() => (isFocused = false)}
/>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<header
  class="titlebar"
  class:unfocused={!isFocused}
  style="--wails-draggable:drag"
  ondblclick={handleDoubleClick}
>
  <div class="titlebar-left">
    <div class="brand">
      <svg
        class="brand-logo"
        viewBox="0 0 24 24"
        width="18"
        height="18"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <rect x="3" y="3" width="11" height="11" rx="2.5" />
        <rect x="10" y="10" width="11" height="11" rx="2.5" />
      </svg>
      <span class="brand-name">{t("app.title")}</span>
    </div>

    {#if appState.config?.rootPath}
      <div class="no-drag" data-no-drag style="--wails-draggable:no-drag">
        <Tooltip text={t("titlebar.change_root") + ": " + appState.config.rootPath}>
          <button type="button" class="root-chip" onclick={onchangeRoot}>
            <span class="chip-label">{rootTruncated}</span>
            <ChevronDown size={12} />
          </button>
        </Tooltip>
      </div>
    {/if}
  </div>

  <div class="titlebar-right no-drag" data-no-drag style="--wails-draggable:no-drag">
    {#if appState.config?.rootPath}
      <IconButton
        label={t("titlebar.check_remote")}
        disabled={appState.isChecking || appState.isSyncing}
        onclick={oncheckRemote}
      >
        <span class="check-icon" class:spinning={appState.isChecking}>
          <RefreshCw size={16} />
        </span>
      </IconButton>

      <Button
        variant="primary"
        size="sm"
        disabled={(direction === "idle" && !hasRuleWork) || appState.isSyncing}
        loading={appState.isSyncing}
        onclick={onsync}
      >
        <span class="sync-icon" class:conflict-icon={direction === "conflict"}>
          {#if direction === "down"}
            <ArrowDown size={14} />
          {:else if direction === "up"}
            <ArrowUp size={14} />
          {:else if direction === "both"}
            <ArrowUpDown size={14} />
          {:else if direction === "conflict"}
            <TriangleAlert size={14} />
          {:else if direction === "offline" || direction === "no_remote"}
            <CloudOff size={14} />
          {:else}
            <Check size={14} />
          {/if}
        </span>
        <span>{t("titlebar.sync_button")}</span>
        {#if pendingCount > 0}
          <span class="badge">{pendingCount}</span>
        {/if}
      </Button>
    {/if}

    {@render notifications?.()}

    <IconButton label={t("titlebar.settings_button")} onclick={onopenSettings}>
      <Settings size={16} />
    </IconButton>

    <div class="window-separator" aria-hidden="true"></div>

    <div class="window-controls">
      <button
        type="button"
        class="win-btn"
        aria-label={t("titlebar.window_minimize")}
        title={t("titlebar.window_minimize")}
        onclick={handleMinimise}
      >
        <Minus size={16} strokeWidth={1.5} />
      </button>

      <button
        type="button"
        class="win-btn"
        aria-label={isMaximised ? t("titlebar.window_restore") : t("titlebar.window_maximize")}
        title={isMaximised ? t("titlebar.window_restore") : t("titlebar.window_maximize")}
        onclick={handleToggleMaximise}
      >
        {#if isMaximised}
          <Copy size={15} strokeWidth={1.5} />
        {:else}
          <Square size={15} strokeWidth={1.5} />
        {/if}
      </button>

      <button
        type="button"
        class="win-btn win-close"
        aria-label={t("titlebar.window_close")}
        title={t("titlebar.window_close")}
        onclick={handleClose}
      >
        <X size={16} strokeWidth={1.5} />
      </button>
    </div>
  </div>
</header>

<style>
  .titlebar {
    height: var(--titlebar-height);
    padding-left: var(--space-4);
    padding-right: 0;
    background-color: var(--canvas);
    display: flex;
    align-items: center;
    justify-content: space-between;
    user-select: none;
    z-index: 20;
    transition: opacity var(--duration-fast) var(--easing-out);
  }

  .titlebar.unfocused .brand,
  .titlebar.unfocused .root-chip {
    opacity: 0.6;
  }

  .titlebar-left {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-width: 0;
  }

  .titlebar-right {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    height: 100%;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--ink);
  }

  .brand-logo {
    color: var(--ink);
    flex-shrink: 0;
  }

  .brand-name {
    font-size: var(--text-sm);
    font-weight: var(--weight-semibold);
    color: var(--ink);
    letter-spacing: -0.01em;
  }

  .root-chip {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border: none;
    border-radius: var(--radius-sm);
    padding: 3px var(--space-3);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-muted);
    cursor: pointer;
    transition:
      color var(--duration-fast) var(--easing-out),
      box-shadow var(--duration-fast) var(--easing-out);
    max-width: 320px;
  }

  .root-chip:hover {
    color: var(--ink);
  }

  .root-chip:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .chip-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .check-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }

  .check-icon.spinning {
    animation: spin 0.9s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .sync-icon {
    display: inline-flex;
    align-items: center;
  }

  .sync-icon.conflict-icon {
    color: var(--dot-conflict);
  }

  .badge {
    background-color: var(--badge-overlay);
    padding: 1px 6px;
    border-radius: var(--radius-full);
    font-size: var(--text-xs);
    font-weight: var(--weight-bold);
    font-variant-numeric: tabular-nums;
  }

  .window-separator {
    width: 1px;
    height: 18px;
    background-color: var(--hairline);
    margin: 0 var(--space-1);
  }

  .window-controls {
    display: flex;
    align-items: center;
    height: 100%;
  }

  .win-btn {
    width: 46px;
    height: 44px;
    border: none;
    border-radius: 0;
    box-shadow: none;
    background-color: transparent;
    color: var(--ink);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    padding: 0;
    transition:
      background-color var(--duration-fast) var(--easing-out),
      color var(--duration-fast) var(--easing-out);
  }

  .win-btn:hover {
    background-color: var(--surface-sunken);
  }

  .win-btn:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .win-close:hover {
    background-color: var(--win-close-hover-bg);
    color: var(--win-close-hover-fg);
  }

  @media (prefers-reduced-motion: reduce) {
    .check-icon.spinning {
      animation: none;
    }
  }
</style>
