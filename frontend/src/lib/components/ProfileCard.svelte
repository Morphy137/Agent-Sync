<script lang="ts">
  import {
    ChevronDown,
    ChevronRight,
    MoreHorizontal,
    ExternalLink,
    FolderSync,
    Unlink,
  } from "@lucide/svelte";
  import type { profiles, config, syncer } from "../../../wailsjs/go/models";
  import { t } from "../i18n";
  import Button from "./Button.svelte";
  import IconButton from "./IconButton.svelte";
  import StatusPill from "./StatusPill.svelte";
  import Tooltip from "./Tooltip.svelte";

  interface Props {
    profile: profiles.Profile;
    connection?: config.Connection;
    status?: syncer.ConnectionStatus;
    onconnect: (prof: string) => void;
    onupdate: (connId: string) => void;
    onbackups: (connId: string, prof: string) => void;
    onchangeTarget: (connId: string, prof: string) => void;
    onopenExplorer: (path: string) => void;
    ondisconnect: (connId: string) => void;
  }

  let {
    profile,
    connection,
    status,
    onconnect,
    onupdate,
    onbackups,
    onchangeTarget,
    onopenExplorer,
    ondisconnect,
  }: Props = $props();

  let expanded = $state(false);
  let menuOpen = $state(false);
  let menuContainer = $state<HTMLDivElement | null>(null);

  function closeMenu() {
    menuOpen = false;
  }

  function handleWindowPointerDown(e: PointerEvent) {
    if (!menuOpen || !menuContainer) return;
    if (menuContainer.contains(e.target as Node)) return;
    closeMenu();
  }

  function handleWindowKeyDown(e: KeyboardEvent) {
    if (menuOpen && e.key === "Escape") closeMenu();
  }

  const isConnected = $derived(!!connection);
  const aggregateStatus = $derived(status?.status || (isConnected ? "in_sync" : "not_connected"));
  const initialLetter = $derived((profile.name || "P").trim().charAt(0).toUpperCase());

  const targetFolderName = $derived.by(() => {
    if (!connection) return "";
    const parts = connection.targetPath.split(/[/\\]/).filter(Boolean);
    return parts[parts.length - 1] || connection.targetPath;
  });

  const targetTruncated = $derived.by(() => {
    if (!connection) return "";
    const p = connection.targetPath;
    if (p.length <= 42) return p;
    const head = p.slice(0, 18);
    const tail = p.slice(-21);
    return `${head}…${tail}`;
  });

  const canUpdate = $derived.by(() => {
    if (!status?.files) return false;
    return status.files.some(
      (f) =>
        f.status === "source_changed" ||
        f.status === "new_in_source" ||
        f.status === "target_missing"
    );
  });

  function getFileDotClass(fileStatus: string): string {
    switch (fileStatus) {
      case "in_sync":
        return "dot-ok";
      case "source_changed":
      case "new_in_source":
      case "target_missing":
        return "dot-down";
      case "target_changed":
      case "untracked_diff":
        return "dot-up";
      case "conflict":
      case "unreadable":
        return "dot-conflict";
      default:
        return "dot-idle";
    }
  }
</script>

<svelte:window onpointerdown={handleWindowPointerDown} onkeydown={handleWindowKeyDown} />

<div class="profile-card" class:disconnected={!isConnected}>
  <!-- Top section: Tile + Info + StatusPill -->
  <div class="card-top">
    <div class="tile" class:tile-muted={!isConnected} aria-hidden="true">
      {initialLetter}
    </div>

    <div class="info-col">
      <div class="header-row">
        <span class="profile-name">
          {profile.name}{#if connection && targetFolderName}
            ⇄ {targetFolderName}{/if}
        </span>
        <StatusPill status={aggregateStatus} text={t(`status.${aggregateStatus}`)} />
      </div>

      {#if connection}
        <div class="paths-block">
          <div class="path-row">
            <span class="path-label">{t("connections.card.repo_label")}</span>
            <Tooltip text={profile.path || `${profile.name}/`}>
              <span class="path-val">{profile.name}/</span>
            </Tooltip>
          </div>
          <div class="path-row">
            <span class="path-label">{t("connections.card.target_label")}</span>
            <Tooltip text={connection.targetPath}>
              <span class="path-val">{targetTruncated}</span>
            </Tooltip>
          </div>
        </div>
      {:else}
        <div class="target-line muted">
          <span>{t("connections.card.not_connected")}</span>
        </div>
      {/if}

      <div class="files-count">
        {t(`connections.card.managed_files_count_${profile.fileCount === 1 ? "one" : "other"}`, {
          count: profile.fileCount,
        })}
      </div>
    </div>
  </div>

  <div class="hairline" aria-hidden="true"></div>

  <!-- Actions footer -->
  <div class="card-actions">
    {#if !connection}
      <div class="actions-left">
        <Button variant="primary" size="sm" onclick={() => onconnect(profile.name)}>
          {t("connections.card.button_connect")}
        </Button>
      </div>
    {:else}
      <div class="actions-left">
        <Button
          variant="secondary"
          size="sm"
          disabled={!canUpdate}
          onclick={() => onupdate(connection.id)}
        >
          {t("connections.card.button_update")}
        </Button>

        <Button variant="ghost" size="sm" onclick={() => onbackups(connection.id, profile.name)}>
          {t("connections.card.button_backups")}
        </Button>
      </div>

      <div class="menu-container" bind:this={menuContainer}>
        <IconButton
          label={t("connections.card.menu_toggle")}
          onclick={() => (menuOpen = !menuOpen)}
          ariaExpanded={menuOpen}
        >
          <MoreHorizontal size={16} />
        </IconButton>

        {#if menuOpen}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <div class="dropdown-menu" onclick={closeMenu} role="menu" tabindex="-1">
            {#if profile.path}
              <button
                type="button"
                class="dropdown-item"
                role="menuitem"
                onclick={() => onopenExplorer(profile.path)}
              >
                <ExternalLink size={14} />
                <span>{t("connections.open_repo")}</span>
              </button>
            {/if}
            <button
              type="button"
              class="dropdown-item"
              role="menuitem"
              onclick={() => onopenExplorer(connection.targetPath)}
            >
              <ExternalLink size={14} />
              <span>{t("connections.open_target")}</span>
            </button>
            <button
              type="button"
              class="dropdown-item"
              role="menuitem"
              onclick={() => onchangeTarget(connection.id, profile.name)}
            >
              <FolderSync size={14} />
              <span>{t("connections.change_target")}</span>
            </button>
            <button
              type="button"
              class="dropdown-item"
              role="menuitem"
              onclick={() => ondisconnect(connection.id)}
            >
              <Unlink size={14} />
              <span>{t("connections.disconnect")}</span>
            </button>
          </div>
        {/if}
      </div>
    {/if}
  </div>

  <!-- Expandable Files Section -->
  {#if connection && status?.files && status.files.length > 0}
    <button
      type="button"
      class="expand-toggle"
      onclick={() => (expanded = !expanded)}
      aria-expanded={expanded}
    >
      {#if expanded}
        <ChevronDown size={14} />
        <span>{t("connections.card.hide_files")}</span>
      {:else}
        <ChevronRight size={14} />
        <span>{t("connections.card.view_files", { count: status.files.length })}</span>
      {/if}
    </button>

    {#if expanded}
      <div class="files-panel">
        {#each status.files as file (file.relPath)}
          <div class="file-row">
            <div class="file-left">
              <span class="file-dot {getFileDotClass(file.status)}"></span>
              <span class="file-path">{file.relPath}</span>
            </div>
            <span class="file-status-text">{t(`status.${file.status}`)}</span>
          </div>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  .profile-card {
    background-color: var(--surface);
    border-radius: var(--radius-lg);
    box-shadow: var(--elev-1);
    padding: var(--space-5);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    transition: box-shadow var(--duration-fast) var(--easing-out);
  }

  .profile-card:hover {
    box-shadow: var(--elev-2);
  }

  .card-top {
    display: flex;
    align-items: flex-start;
    gap: var(--space-4);
  }

  .tile {
    width: 40px;
    height: 40px;
    border-radius: var(--radius-md);
    background-color: var(--surface);
    box-shadow: var(--elev-1);
    color: var(--ink);
    font-size: var(--text-md);
    font-weight: var(--weight-bold);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    user-select: none;
  }

  .tile.tile-muted {
    opacity: 0.6;
  }

  .info-col {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .header-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .profile-name {
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
    letter-spacing: -0.01em;
  }

  .paths-block {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    column-gap: var(--space-2);
    row-gap: 2px;
  }

  .path-row {
    display: contents;
    font-size: var(--text-xs);
  }

  .path-label {
    color: var(--text-subtle);
    font-size: 11px;
    align-self: center;
  }

  .path-val {
    font-family: var(--font-mono);
    color: var(--text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .target-line {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--text-xs);
    color: var(--text-muted);
    min-width: 0;
  }

  .target-line.muted {
    opacity: 0.6;
  }

  .arrow {
    color: var(--text-subtle);
    flex-shrink: 0;
  }

  .target-path {
    font-family: var(--font-mono);
    color: var(--text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .files-count {
    font-size: var(--text-xs);
    color: var(--text-subtle);
    font-variant-numeric: tabular-nums;
  }

  .hairline {
    height: 1px;
    background-color: var(--hairline);
    width: 100%;
  }

  .card-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .actions-left {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .menu-container {
    position: relative;
  }

  .dropdown-menu {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: var(--space-1);
    background-color: var(--surface);
    border-radius: var(--radius-md);
    box-shadow: var(--elev-3);
    min-width: 180px;
    padding: var(--space-1);
    z-index: 50;
    display: flex;
    flex-direction: column;
  }

  .dropdown-item {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    font-size: var(--text-xs);
    color: var(--ink);
    background: none;
    border: none;
    border-radius: var(--radius-xs);
    cursor: pointer;
    text-align: left;
    transition: background-color var(--duration-fast) var(--easing-out);
  }

  .dropdown-item:hover {
    background-color: var(--surface-sunken);
  }

  .expand-toggle {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    background: none;
    border: none;
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
    cursor: pointer;
    padding: 0;
    align-self: flex-start;
  }

  .expand-toggle:hover {
    color: var(--ink);
  }

  .files-panel {
    background-color: var(--surface-sunken);
    box-shadow: var(--sunken);
    border-radius: var(--radius-md);
    padding: var(--space-2) var(--space-3);
    display: flex;
    flex-direction: column;
  }

  .file-row {
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    font-size: var(--text-xs);
  }

  .file-left {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }

  .file-dot {
    width: 8px;
    height: 8px;
    border-radius: var(--radius-full);
    flex-shrink: 0;
  }

  .dot-ok {
    background-color: var(--dot-ok);
  }

  .dot-down {
    background-color: var(--dot-down);
  }

  .dot-up {
    background-color: var(--dot-up);
  }

  .dot-conflict {
    background-color: var(--dot-conflict);
  }

  .dot-idle {
    background-color: var(--dot-idle);
  }

  .file-path {
    font-family: var(--font-mono);
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-status-text {
    color: var(--text-muted);
    flex-shrink: 0;
  }
</style>
