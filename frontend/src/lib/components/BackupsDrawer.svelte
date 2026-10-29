<script lang="ts">
  import { RotateCcw, FolderOpen, History } from "@lucide/svelte";
  import type { backup } from "../../../wailsjs/go/models";
  import { t, formatDate } from "../i18n";
  import Drawer from "./Drawer.svelte";
  import Button from "./Button.svelte";
  import StatusPill from "./StatusPill.svelte";

  interface Props {
    open: boolean;
    profileName: string;
    backups: backup.BackupInfo[];
    onclose: () => void;
    onrestore: (backupId: string) => void;
    onopenFolder: (path: string) => void;
  }

  let {
    open,
    profileName,
    backups,
    onclose,
    onrestore,
    onopenFolder,
  }: Props = $props();

  function handleRestore(b: backup.BackupInfo) {
    if (confirm(t("backups.confirm_restore"))) {
      onrestore(b.id);
    }
  }
</script>

<Drawer {open} title={t("backups.title", { name: profileName })} {onclose}>
  <div class="backups-container">
    {#if backups.length === 0}
      <div class="empty-backups">
        <span class="history-icon"><History size={36} /></span>
        <p>{t("backups.empty")}</p>
      </div>
    {:else}
      <div class="backups-list">
        {#each backups as b (b.id)}
          <div class="backup-card">
            <div class="card-header">
              <span class="backup-date">{formatDate(b.createdAt)}</span>
              <StatusPill
                status="neutral"
                text={t(`backups.reason_${b.reason}`)}
              />
            </div>

            <div class="card-body">
              <span class="file-count">
                {t("backups.files_count", { count: b.files.length })}
              </span>
            </div>

            <div class="card-actions">
              <Button
                size="sm"
                variant="ghost"
                onclick={() => onopenFolder(b.path)}
              >
                <FolderOpen size={14} />
                <span>{t("backups.button_open_folder")}</span>
              </Button>
              <Button
                size="sm"
                variant="secondary"
                onclick={() => handleRestore(b)}
              >
                <RotateCcw size={14} />
                <span>{t("backups.button_restore")}</span>
              </Button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</Drawer>

<style>
  .backups-container {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .empty-backups {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
    padding: var(--space-10) var(--space-4);
    text-align: center;
    gap: var(--space-3);
    font-size: var(--text-sm);
  }

  .history-icon {
    color: var(--text-subtle);
  }

  .backups-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .backup-card {
    background-color: var(--surface-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .backup-date {
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    color: var(--text);
  }

  .card-body {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .card-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    border-top: 1px solid var(--border);
    padding-top: var(--space-3);
  }
</style>
