<script lang="ts">
  import { onMount } from "svelte";
  import { EventsOn } from "../wailsjs/runtime/runtime";
  import type { config, service, orchestrator, backup } from "../wailsjs/go/models";
  import { api, parseError } from "./lib/api";
  import { appState } from "./lib/stores/appState.svelte";
  import { t } from "./lib/i18n";
  import { Plus, RefreshCw, FolderSync, ChevronDown, ChevronRight } from "@lucide/svelte";
  import TitleBar from "./lib/components/TitleBar.svelte";
  import StatusBar from "./lib/components/StatusBar.svelte";
  import Banner from "./lib/components/Banner.svelte";
  import ProfileCard from "./lib/components/ProfileCard.svelte";
  import WelcomeView from "./lib/components/WelcomeView.svelte";
  import SyncPreviewModal from "./lib/components/SyncPreviewModal.svelte";
  import ConnectionWizardModal from "./lib/components/ConnectionWizardModal.svelte";
  import BackupsDrawer from "./lib/components/BackupsDrawer.svelte";
  import SettingsDrawer from "./lib/components/SettingsDrawer.svelte";
  import AddRemoteModal from "./lib/components/AddRemoteModal.svelte";
  import GitIdentityModal from "./lib/components/GitIdentityModal.svelte";
  import Toast from "./lib/components/Toast.svelte";
  import Button from "./lib/components/Button.svelte";
  import IconButton from "./lib/components/IconButton.svelte";
  import Modal from "./lib/components/Modal.svelte";

  let backupsList = $state<backup.BackupInfo[]>([]);
  let unconnectedOpen = $state(true);
  let disconnectModalConnection = $state<config.Connection | null>(null);
  let isDisconnecting = $state(false);

  onMount(() => {
    appState.loadInitialData();

    EventsOn("status:changed", (payload: orchestrator.SyncPlan) => {
      appState.handleStatusChanged(payload);
    });
  });

  async function handleSelectRoot() {
    try {
      const selected = await api.selectRootFolder(t("dialogs.select_root_title"));
      if (selected) {
        const updated = await api.setRootPath(selected);
        appState.config = updated;
        await appState.refreshAll();
      }
    } catch (err) {
      appState.showError(err);
    }
  }

  function handleConnectProfile(profileName: string) {
    appState.connectionWizardProfile = profileName;
    appState.connectionWizardReplacesId = "";
    appState.connectionWizardOpen = true;
  }

  function handleChangeTarget(connectionId: string, profileName: string) {
    appState.connectionWizardProfile = profileName;
    appState.connectionWizardReplacesId = connectionId;
    appState.connectionWizardOpen = true;
  }

  async function handleUpdateConnection(connectionId: string) {
    try {
      await api.applyConnection(connectionId);
      await appState.refreshAll();
      appState.addToast("success", t("toasts.sync_success"));
    } catch (err) {
      appState.showError(err);
    }
  }

  async function handleOpenBackups(connectionId: string, profileName: string) {
    try {
      appState.activeBackupConnectionId = connectionId;
      appState.activeBackupProfileName = profileName;
      backupsList = await api.listBackups(connectionId);
      appState.backupsDrawerOpen = true;
    } catch (err) {
      appState.showError(err);
    }
  }

  async function handleRestoreBackup(backupId: string) {
    if (!appState.activeBackupConnectionId) return;
    try {
      await api.restoreBackup(appState.activeBackupConnectionId, backupId);
      backupsList = await api.listBackups(appState.activeBackupConnectionId);
      await appState.refreshAll();
      appState.addToast("success", t("toasts.restore_success"));
    } catch (err) {
      appState.showError(err);
    }
  }

  function handleDisconnect(connectionId: string) {
    disconnectModalConnection =
      appState.config?.connections?.find((c) => c.id === connectionId) || null;
  }

  async function handleConfirmDisconnect() {
    if (!disconnectModalConnection || isDisconnecting) return;
    isDisconnecting = true;
    try {
      const updated = await api.disconnectProfile(disconnectModalConnection.id);
      appState.config = updated;
      disconnectModalConnection = null;
      await appState.refreshAll();
      appState.addToast("success", t("toasts.disconnect_success"));
    } catch (err) {
      appState.showError(err);
    } finally {
      isDisconnecting = false;
    }
  }

  async function handleOpenExplorer(path: string) {
    try {
      await api.openInExplorer(path);
    } catch (err) {
      appState.showError(err);
    }
  }

  async function handleSyncClick() {
    if (
      appState.config?.confirmBeforeSync ||
      appState.statusPlan?.direction === "conflict"
    ) {
      try {
        const plan = await api.planSync();
        appState.statusPlan = plan;
        appState.previewModalOpen = true;
      } catch (err) {
        appState.showError(err);
      }
    } else {
      await executeSyncNow([]);
    }
  }

  async function executeSyncNow(resolutions: orchestrator.ConflictResolution[]) {
    try {
      appState.isSyncing = true;
      const fingerprint = appState.statusPlan?.fingerprint || "";
      await api.executeSync(fingerprint, resolutions);
      appState.previewModalOpen = false;
      await appState.refreshAll();
      appState.addToast("success", t("toasts.sync_success"));
    } catch (err) {
      const parsed = parseError(err);
      appState.showError(err);
      if (parsed.code === "plan_changed") {
        try {
          const freshPlan = await api.planSync();
          appState.statusPlan = freshPlan;
          appState.previewModalOpen = true;
        } catch (planErr) {
          appState.showError(planErr);
        }
      }
    } finally {
      appState.isSyncing = false;
    }
  }

  async function handleSaveSettings(
    payload: service.SettingsPayload,
    gitName: string,
    gitEmail: string,
    newRootPath?: string
  ) {
    try {
      const updated = await api.updateSettings(payload);
      appState.config = updated;

      // Only save identity if name or email changed compared to gitInfo (R5)
      const currentName = appState.gitInfo?.userName || "";
      const currentEmail = appState.gitInfo?.userEmail || "";
      const trimmedName = gitName.trim();
      const trimmedEmail = gitEmail.trim();
      if (
        appState.gitInfo?.isRepo &&
        trimmedEmail &&
        (trimmedName !== currentName || trimmedEmail !== currentEmail)
      ) {
        await api.setGitIdentity(trimmedName, trimmedEmail);
      }

      if (newRootPath && newRootPath.trim() !== appState.config?.rootPath) {
        const rootCfg = await api.setRootPath(newRootPath.trim());
        appState.config = rootCfg;
      }

      await appState.refreshAll();
      appState.settingsDrawerOpen = false;
      appState.addToast("success", t("toasts.save_success"));
    } catch (err) {
      appState.showError(err);
    }
  }

  async function handleSaveGitIdentity(name: string, email: string) {
    try {
      await api.setGitIdentity(name, email);
      appState.gitIdentityModalOpen = false;
      await appState.refreshAll();
      appState.addToast("success", t("toasts.identity_saved"));
    } catch (err) {
      appState.showError(err);
    }
  }

  async function handleInitRepo() {
    try {
      await api.initRepository();
      await appState.refreshAll();
      appState.addToast("success", t("toasts.init_repo_success"));
    } catch (err) {
      appState.showError(err);
    }
  }

  async function handleAddRemote(url: string) {
    try {
      await api.addRemote(url);
      appState.addRemoteModalOpen = false;
      await appState.refreshAll();
      appState.addToast("success", t("toasts.add_remote_success"));
    } catch (err) {
      appState.showError(err);
    }
  }

  function getStatusForConnection(connId?: string) {
    if (!connId || !appState.statusPlan?.connections) return undefined;
    return appState.statusPlan.connections.find((c) => c.connectionId === connId);
  }

  const connectedProfiles = $derived.by(() => {
    if (!appState.config?.connections) return [];
    return appState.config.connections.map((conn) => {
      const prof = appState.profiles.find((p) => p.name.toLowerCase() === conn.profile.toLowerCase()) || {
        name: conn.profile,
        path: `${appState.config?.rootPath || ""}/${conn.profile}`,
        fileCount: 0,
      };
      return { conn, prof };
    });
  });

  const unconnectedProfiles = $derived.by(() => {
    if (!appState.profiles) return [];
    const connectedNames = new Set(
      (appState.config?.connections || []).map((c) => c.profile.toLowerCase())
    );
    return appState.profiles.filter((p) => !connectedNames.has(p.name.toLowerCase()));
  });
</script>

<div class="app-layout">
  <TitleBar
    onsync={handleSyncClick}
    oncheckRemote={() => appState.checkRemote()}
    onopenSettings={() => (appState.settingsDrawerOpen = true)}
    onchangeRoot={handleSelectRoot}
  />

  <main class="content-area">
    <div class="content-inner">
      {#if appState.config?.rootPath}
        <div class="banners-container">
          {#if appState.gitInfo && !appState.gitInfo.isRepo}
            <Banner
              variant="warning"
              title={t("banners.not_repo.title")}
              message={t("banners.not_repo.message")}
              actionLabel={t("banners.not_repo.button_init")}
              onaction={handleInitRepo}
            />
          {:else if appState.gitInfo && !appState.gitInfo.hasUpstream}
            <Banner
              variant="info"
              title={t("banners.no_remote.title")}
              message={t("banners.no_remote.message")}
              actionLabel={t("banners.no_remote.button_add_remote")}
              onaction={() => (appState.addRemoteModalOpen = true)}
            />
          {:else if appState.statusPlan && appState.statusPlan.behind > 0}
            <Banner
              variant="info"
              title={t("banners.remote_changes.title")}
              message={t("banners.remote_changes.message")}
              actionLabel={t("banners.remote_changes.button_sync")}
              onaction={handleSyncClick}
            />
          {/if}
        </div>

        <div class="connections-header">
          <div class="connections-title-group">
            <h2 class="section-title">{t("connections.title")}</h2>
            <span class="section-summary">
              {t("connections.connected_count", {
                connected: connectedProfiles.length,
                unconnected: unconnectedProfiles.length,
              })}
            </span>
          </div>

          <div class="connections-header-actions">
            <IconButton
              label={t("titlebar.check_remote")}
              disabled={appState.isChecking}
              onclick={() => appState.checkRemote()}
            >
              <RefreshCw size={16} class={appState.isChecking ? "spin" : ""} />
            </IconButton>

            <Button
              variant="primary"
              size="sm"
              onclick={() => {
                appState.connectionWizardProfile = "";
                appState.connectionWizardReplacesId = "";
                appState.connectionWizardOpen = true;
              }}
            >
              <Plus size={16} />
              <span>{t("connections.add_connection")}</span>
            </Button>
          </div>
        </div>

        {#if connectedProfiles.length > 0}
          <div class="profiles-grid">
            {#each connectedProfiles as item (item.conn.id)}
              {@const status = getStatusForConnection(item.conn.id)}
              <ProfileCard
                profile={item.prof}
                connection={item.conn}
                {status}
                onconnect={handleConnectProfile}
                onupdate={handleUpdateConnection}
                onbackups={handleOpenBackups}
                onchangeTarget={handleChangeTarget}
                onopenExplorer={handleOpenExplorer}
                ondisconnect={handleDisconnect}
              />
            {/each}
          </div>
        {:else}
          <div class="empty-connections-card">
            <div class="empty-icon-box">
              <FolderSync size={32} />
            </div>
            <h3 class="empty-title">{t("connections.empty_title")}</h3>
            <p class="empty-desc">{t("connections.empty_desc")}</p>
            <Button
              variant="primary"
              size="md"
              onclick={() => {
                appState.connectionWizardProfile = "";
                appState.connectionWizardReplacesId = "";
                appState.connectionWizardOpen = true;
              }}
            >
              <Plus size={16} />
              <span>{t("connections.add_connection")}</span>
            </Button>
          </div>
        {/if}

        {#if unconnectedProfiles.length > 0}
          <div class="unconnected-section">
            <button
              type="button"
              class="unconnected-toggle"
              onclick={() => (unconnectedOpen = !unconnectedOpen)}
              aria-expanded={unconnectedOpen}
            >
              {#if unconnectedOpen}
                <ChevronDown size={16} />
              {:else}
                <ChevronRight size={16} />
              {/if}
              <span class="unconnected-title">
                {t("connections.unconnected_folders")} ({unconnectedProfiles.length})
              </span>
            </button>

            {#if unconnectedOpen}
              <div class="unconnected-list">
                {#each unconnectedProfiles as prof (prof.name)}
                  <div class="unconnected-row">
                    <div class="unconnected-info">
                      <span class="unconnected-name">{prof.name}</span>
                      <span class="unconnected-count">
                        · {t("wizard.files_count", { count: prof.fileCount })}
                      </span>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      onclick={() => handleConnectProfile(prof.name)}
                    >
                      <span>{t("connections.connect")}</span>
                    </Button>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {/if}
      {:else}
        <WelcomeView onsuccess={() => appState.refreshAll()} />
      {/if}
    </div>
  </main>

  <StatusBar />

  <SyncPreviewModal
    open={appState.previewModalOpen}
    plan={appState.statusPlan}
    onclose={() => (appState.previewModalOpen = false)}
    onconfirm={executeSyncNow}
  />

  <ConnectionWizardModal
    open={appState.connectionWizardOpen}
    initialProfile={appState.connectionWizardProfile}
    replacesConnectionId={appState.connectionWizardReplacesId}
    onclose={() => {
      appState.connectionWizardOpen = false;
      appState.connectionWizardProfile = "";
      appState.connectionWizardReplacesId = "";
    }}
    onsuccess={async () => {
      await appState.refreshAll();
      appState.addToast("success", t("toasts.sync_success"));
    }}
  />

  <Modal
    open={Boolean(disconnectModalConnection)}
    title={t("connections.disconnect_confirm_title", {
      profile: disconnectModalConnection?.profile || "",
    })}
    onclose={() => (disconnectModalConnection = null)}
  >
    <div class="disconnect-modal-content">
      <p class="disconnect-msg">{t("connections.disconnect_confirm_msg")}</p>
      <div class="disconnect-modal-actions">
        <Button
          variant="secondary"
          onclick={() => (disconnectModalConnection = null)}
        >
          {t("common.cancel")}
        </Button>
        <Button
          variant="primary"
          disabled={isDisconnecting}
          loading={isDisconnecting}
          onclick={handleConfirmDisconnect}
        >
          {t("connections.disconnect")}
        </Button>
      </div>
    </div>
  </Modal>

  <BackupsDrawer
    open={appState.backupsDrawerOpen}
    profileName={appState.activeBackupProfileName}
    backups={backupsList}
    onclose={() => (appState.backupsDrawerOpen = false)}
    onrestore={handleRestoreBackup}
    onopenFolder={handleOpenExplorer}
  />

  <SettingsDrawer
    open={appState.settingsDrawerOpen}
    onclose={() => (appState.settingsDrawerOpen = false)}
    onsave={handleSaveSettings}
  />

  <AddRemoteModal
    open={appState.addRemoteModalOpen}
    onclose={() => (appState.addRemoteModalOpen = false)}
    onsubmit={handleAddRemote}
  />

  <GitIdentityModal
    open={appState.gitIdentityModalOpen}
    initialName={appState.gitInfo?.userName || ""}
    initialEmail={appState.gitInfo?.userEmail || ""}
    onclose={() => (appState.gitIdentityModalOpen = false)}
    onsubmit={handleSaveGitIdentity}
  />

  <Toast toasts={appState.toasts} ondismiss={(id) => appState.dismissToast(id)} />
</div>

<style>
  .app-layout {
    display: grid;
    grid-template-rows: var(--titlebar-height) 1fr var(--statusbar-height);
    height: 100vh;
    background-color: var(--canvas);
    color: var(--ink);
    font-family: var(--font-sans);
  }

  .content-area {
    overflow-y: auto;
    min-height: 0;
  }

  .content-inner {
    max-width: var(--content-max-width);
    margin: 0 auto;
    padding: var(--space-6) var(--space-8);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .banners-container {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .connections-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }

  .connections-title-group {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
  }

  .connections-header-actions {
    display: flex;
    align-items: center;
    gap: var(--space-2);
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

  .profiles-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
    gap: var(--space-4);
  }

  .empty-connections-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    text-align: center;
    padding: var(--space-8);
    background-color: var(--surface);
    border: 1px dashed var(--border-subtle);
    border-radius: var(--radius-xl);
    gap: var(--space-3);
  }

  .empty-icon-box {
    width: 56px;
    height: 56px;
    border-radius: var(--radius-full);
    background-color: var(--surface-sunken);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
  }

  .empty-title {
    font-size: var(--text-md);
    font-weight: var(--weight-semibold);
    color: var(--ink);
    margin: 0;
  }

  .empty-desc {
    font-size: var(--text-sm);
    color: var(--text-muted);
    max-width: 360px;
    margin: 0;
  }

  .unconnected-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin-top: var(--space-2);
    border-top: 1px solid var(--border-subtle);
    padding-top: var(--space-3);
  }

  .unconnected-toggle {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    color: var(--ink);
  }

  .unconnected-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
  }

  .unconnected-list {
    display: flex;
    flex-direction: column;
    border-radius: var(--radius-lg);
    background-color: var(--surface);
    border: 1px solid var(--border-subtle);
    overflow: hidden;
  }

  .unconnected-row {
    height: 44px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 var(--space-4);
    border-bottom: 1px solid var(--border-subtle);
  }

  .unconnected-row:last-child {
    border-bottom: none;
  }

  .unconnected-info {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .unconnected-name {
    font-weight: var(--weight-medium);
    font-size: var(--text-sm);
    color: var(--ink);
  }

  .unconnected-count {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .disconnect-modal-content {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .disconnect-msg {
    font-size: var(--text-sm);
    color: var(--text-muted);
    margin: 0;
  }

  .disconnect-modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }
</style>
