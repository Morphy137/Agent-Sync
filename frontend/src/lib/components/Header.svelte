<script lang="ts">
  import {
    Check,
    ArrowDown,
    ArrowUp,
    ArrowUpDown,
    TriangleAlert,
    CloudOff,
    Settings,
    FolderGit2,
  } from "@lucide/svelte";
  import { appState } from "../stores/appState.svelte";
  import { t } from "../i18n";
  import Button from "./Button.svelte";
  import IconButton from "./IconButton.svelte";
  import Tooltip from "./Tooltip.svelte";

  interface Props {
    onsync: () => void;
    onopenSettings: () => void;
    onchangeRoot: () => void;
  }

  let { onsync, onopenSettings, onchangeRoot }: Props = $props();

  const direction = $derived(appState.statusPlan?.direction || "idle");
  const pendingCount = $derived.by(() => {
    if (!appState.statusPlan) return 0;
    const { downloads, uploads, conflicts } = appState.statusPlan;
    return (downloads?.length || 0) + (uploads?.length || 0) + (conflicts?.length || 0);
  });

  const rootTruncated = $derived.by(() => {
    const p = appState.config?.rootPath || "";
    if (p.length > 40) {
      return "..." + p.slice(-37);
    }
    return p;
  });
</script>

<header class="app-header">
  <div class="header-left">
    <div class="brand">
      <div class="brand-icon">
        <FolderGit2 size={22} />
      </div>
      <span class="brand-name">{t("app.title")}</span>
    </div>
  </div>

  <div class="header-center">
    {#if appState.config?.rootPath}
      <Tooltip text={t("header.change_root") + ": " + appState.config.rootPath}>
        <button type="button" class="root-chip" onclick={onchangeRoot}>
          <span class="chip-label">{rootTruncated}</span>
        </button>
      </Tooltip>
    {/if}
  </div>

  <div class="header-right">
    {#if appState.config?.rootPath}
      <Button
        variant={direction === "conflict" ? "danger" : "primary"}
        size="md"
        disabled={direction === "idle" || appState.isSyncing}
        onclick={onsync}
      >
        <span class="sync-icon">
          {#if direction === "down"}
            <ArrowDown size={16} />
          {:else if direction === "up"}
            <ArrowUp size={16} />
          {:else if direction === "both"}
            <ArrowUpDown size={16} />
          {:else if direction === "conflict"}
            <TriangleAlert size={16} />
          {:else if direction === "offline" || direction === "no_remote"}
            <CloudOff size={16} />
          {:else}
            <Check size={16} />
          {/if}
        </span>
        <span>{t("header.sync_button")}</span>
        {#if pendingCount > 0}
          <span class="badge">{pendingCount}</span>
        {/if}
      </Button>
    {/if}

    <IconButton
      label={t("header.settings_button")}
      variant="ghost"
      onclick={onopenSettings}
    >
      <Settings size={20} />
    </IconButton>
  </div>
</header>

<style>
  .app-header {
    height: 56px;
    padding: 0 var(--space-6);
    background-color: var(--surface);
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    user-select: none;
    z-index: 10;
  }

  .header-left,
  .header-right {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .header-center {
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 1;
    margin: 0 var(--space-4);
  }

  .brand {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .brand-icon {
    color: var(--accent);
    display: flex;
    align-items: center;
  }

  .brand-name {
    font-size: var(--text-md);
    font-weight: var(--weight-bold);
    color: var(--text);
    letter-spacing: -0.01em;
  }

  .root-chip {
    background-color: var(--surface-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-full);
    padding: var(--space-1) var(--space-4);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-muted);
    cursor: pointer;
    transition: all var(--duration-fast) var(--easing);
    max-width: 320px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .root-chip:hover {
    border-color: var(--border-strong);
    color: var(--text);
  }

  .root-chip:focus-visible {
    outline: none;
    box-shadow: 0 0 0 2px var(--accent-ring);
  }

  .sync-icon {
    display: flex;
    align-items: center;
  }

  .badge {
    background-color: rgba(255, 255, 255, 0.25);
    padding: 1px 6px;
    border-radius: var(--radius-full);
    font-size: var(--text-xs);
    font-weight: var(--weight-bold);
  }
</style>
