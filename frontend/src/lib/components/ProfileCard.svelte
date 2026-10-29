<script lang="ts">
  import {
    Folder,
    ChevronDown,
    ChevronRight,
    MoreVertical,
    FileText,
    ArrowDown,
    ArrowUp,
    AlertCircle,
    Check,
    FolderSync,
    ExternalLink,
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
    onchangeTarget: (prof: string) => void;
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

  const isConnected = $derived(!!connection);
  const aggregateStatus = $derived(status?.status || (isConnected ? "in_sync" : "not_connected"));

  const targetTruncated = $derived.by(() => {
    if (!connection) return "";
    const p = connection.targetPath;
    if (p.length > 35) {
      return "..." + p.slice(-32);
    }
    return p;
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
</script>

<div class="profile-card">
  <div class="card-main">
    <!-- Row 1: Icon + Profile Name + Status Pill -->
    <div class="row-header">
      <div class="title-group">
        <span class="folder-icon">
          <Folder size={18} />
        </span>
        <span class="profile-name">{profile.name}</span>
        <StatusPill
          status={aggregateStatus}
          text={t(`status.${aggregateStatus}`)}
        />
      </div>

      <div class="actions-group">
        {#if !connection}
          <Button
            variant="secondary"
            size="sm"
            onclick={() => onconnect(profile.name)}
          >
            {t("connections.card.button_connect")}
          </Button>
        {:else}
          {#if canUpdate}
            <Button
              variant="secondary"
              size="sm"
              onclick={() => onupdate(connection.id)}
            >
              <FolderSync size={14} />
              {t("connections.card.button_update")}
            </Button>
          {/if}

          <Button
            variant="ghost"
            size="sm"
            onclick={() => onbackups(connection.id, profile.name)}
          >
            {t("connections.card.button_backups")}
          </Button>

          <div class="menu-container">
            <IconButton
              label={t("connections.card.menu_toggle")}
              variant="ghost"
              size="sm"
              onclick={() => (menuOpen = !menuOpen)}
            >
              <MoreVertical size={16} />
            </IconButton>

            {#if menuOpen}
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <div
                class="dropdown-menu"
                onclick={() => (menuOpen = false)}
                role="menu"
                tabindex="-1"
              >
                <button
                  type="button"
                  class="dropdown-item"
                  role="menuitem"
                  onclick={() => onchangeTarget(profile.name)}
                >
                  <FolderSync size={14} />
                  <span>{t("connections.card.menu_change_target")}</span>
                </button>
                <button
                  type="button"
                  class="dropdown-item"
                  role="menuitem"
                  onclick={() => onopenExplorer(connection.targetPath)}
                >
                  <ExternalLink size={14} />
                  <span>{t("connections.card.menu_open_explorer")}</span>
                </button>
                <button
                  type="button"
                  class="dropdown-item danger"
                  role="menuitem"
                  onclick={() => ondisconnect(connection.id)}
                >
                  <Unlink size={14} />
                  <span>{t("connections.card.menu_disconnect")}</span>
                </button>
              </div>
            {/if}
          </div>
        {/if}
      </div>
    </div>

    <!-- Row 2: Target Path + Managed Files Count + Expand Toggle -->
    <div class="row-details">
      {#if connection}
        <div class="target-info">
          <span class="arrow">→</span>
          <Tooltip text={connection.targetPath}>
            <span class="target-path">{targetTruncated}</span>
          </Tooltip>
          <span class="dot-separator">•</span>
          <span class="files-count">
            {t("connections.card.managed_files_count", { count: profile.fileCount })}
          </span>
        </div>
      {:else}
        <div class="target-info muted">
          <span>{t("connections.card.not_connected")}</span>
          <span class="dot-separator">•</span>
          <span>
            {t("connections.card.managed_files_count", { count: profile.fileCount })}
          </span>
        </div>
      {/if}

      {#if connection && status?.files && status.files.length > 0}
        <button
          type="button"
          class="expand-btn"
          onclick={() => (expanded = !expanded)}
          aria-expanded={expanded}
        >
          {#if expanded}
            <ChevronDown size={14} />
          {:else}
            <ChevronRight size={14} />
          {/if}
          <span>{expanded ? t("common.close") : `${status.files.length} files`}</span>
        </button>
      {/if}
    </div>
  </div>

  <!-- Expandable File List -->
  {#if expanded && status?.files}
    <div class="files-expansion">
      {#each status.files as file (file.relPath)}
        <div class="file-row">
          <div class="file-left">
            <FileText size={14} class="file-icon" />
            <span class="file-path">{file.relPath}</span>
          </div>
          <div class="file-right">
            <span class="file-status status-{file.status}">
              {#if file.status === "in_sync"}
                <Check size={12} />
              {:else if file.status === "target_changed"}
                <ArrowUp size={12} />
              {:else if file.status === "conflict"}
                <AlertCircle size={12} />
              {:else}
                <ArrowDown size={12} />
              {/if}
              <span>{t(`status.${file.status}`)}</span>
            </span>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .profile-card {
    background-color: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
    transition: border-color var(--duration-fast) var(--easing);
    overflow: visible;
  }

  .profile-card:hover {
    border-color: var(--border-strong);
  }

  .card-main {
    padding: var(--space-5);
  }

  .row-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
  }

  .title-group {
    display: flex;
    align-items: center;
    gap: var(--space-3);
  }

  .folder-icon {
    color: var(--accent);
    display: flex;
    align-items: center;
  }

  .profile-name {
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--text);
  }

  .actions-group {
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
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
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
    color: var(--text);
    background: none;
    border: none;
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
    transition: background-color var(--duration-fast) var(--easing);
  }

  .dropdown-item:hover {
    background-color: var(--surface-raised);
  }

  .dropdown-item.danger {
    color: var(--danger-fg);
  }

  .dropdown-item.danger:hover {
    background-color: var(--danger-bg);
  }

  .row-details {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: var(--space-3);
    font-size: var(--text-xs);
  }

  .target-info {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--text-muted);
  }

  .target-info.muted {
    opacity: 0.8;
  }

  .arrow {
    color: var(--text-subtle);
  }

  .target-path {
    font-family: var(--font-mono);
    color: var(--text);
    background-color: var(--surface-raised);
    padding: 1px var(--space-2);
    border-radius: var(--radius-sm);
  }

  .dot-separator {
    color: var(--text-subtle);
  }

  .expand-btn {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    background: none;
    border: none;
    color: var(--text-muted);
    font-size: var(--text-xs);
    cursor: pointer;
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
  }

  .expand-btn:hover {
    color: var(--text);
    background-color: var(--surface-raised);
  }

  .files-expansion {
    border-top: 1px solid var(--border);
    padding: var(--space-3) var(--space-5);
    background-color: var(--surface-raised);
    border-bottom-left-radius: var(--radius-lg);
    border-bottom-right-radius: var(--radius-lg);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .file-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: var(--text-xs);
    padding: var(--space-1) 0;
  }

  .file-left {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .file-icon {
    color: var(--text-subtle);
  }

  .file-path {
    font-family: var(--font-mono);
    color: var(--text);
  }

  .file-status {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--text-xs);
    font-weight: var(--weight-medium);
  }

  .status-in_sync {
    color: var(--success-fg);
  }

  .status-source_changed,
  .status-new_in_source,
  .status-target_missing {
    color: var(--info-fg);
  }

  .status-target_changed {
    color: var(--accent);
  }

  .status-conflict,
  .status-untracked_diff {
    color: var(--danger-fg);
  }
</style>
