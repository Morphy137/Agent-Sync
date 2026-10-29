<script lang="ts">
  import { onMount } from "svelte";
  import { EventsOn } from "../wailsjs/runtime/runtime";
  import type { main, orchestrator } from "../wailsjs/go/models";
  import { api } from "./lib/api";
  import { appState } from "./lib/stores/appState.svelte";
  import { t } from "./lib/i18n";
  import Header from "./lib/components/Header.svelte";
  import StatusBar from "./lib/components/StatusBar.svelte";
  import Banner from "./lib/components/Banner.svelte";
  import ProfileCard from "./lib/components/ProfileCard.svelte";
  import WelcomeView from "./lib/components/WelcomeView.svelte";
  import SyncPreviewModal from "./lib/components/SyncPreviewModal.svelte";
  import InitialLinkModal from "./lib/components/InitialLinkModal.svelte";
  import BackupsDrawer from "./lib/components/BackupsDrawer.svelte";
  import SettingsDrawer from "./lib/components/SettingsDrawer.svelte";
  import AddRemoteModal from "./lib/components/AddRemoteModal.svelte";
  import Toast from "./lib/components/Toast.svelte";

  let backupsList = $state<any[]>([]);

  onMount(() => {
    appState.loadInitialData();

    // Listen to background scheduler event
    EventsOn("status:changed", (payload: any) => {
      appState.handleStatusChanged(payload);
    });
  });

  async function handleSelectRoot() {
    try {
      const selected = await api.selectRootFolder();
      if (selected) {
        const updated = await api.setRootPath(selected);
        appState.config = updated;
        await appState.refreshAll();
      }
    } catch (err) {
      appState.showError(err);
    }
  }

  async function handleConnectProfile(profileName: string) {
    try {
      const targetDir = await api.selectTargetFolder(profileName);
      if (!targetDir) return;

      const result = await api.connectProfile(profileName, targetDir);
      if (result.needsDecision) {
        appState.initialLinkData = result;
        appState.initialLinkModalOpen = true;
      } else {
        await appState.refreshAll();
        appState.addToast("success", t("toasts.sync_success"));
      }
    } catch (err) {
      appState.showError(err);
    }
  }

  async function handleResolveInitialLink(choice: string) {
    if (!appState.initialLinkData) return;
    try {
      await api.resolveInitialLink(appState.initialLinkData.connection, choice);
      appState.initialLinkModalOpen = false;
      appState.initialLinkData = null;
      await appState.refreshAll();
      appState.addToast("success", t("toasts.sync_success"));
    } catch (err) {
      appState.showError(err);
    }
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

  async function handleDisconnect(connectionId: string) {
    try {
      const updated = await api.disconnectProfile(connectionId);
      appState.config = updated;
      await appState.refreshAll();
      appState.addToast("success", t("toasts.disconnect_success"));
    } catch (err) {
      appState.showError(err);
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
    if (appState.config?.confirmBeforeSync || appState.statusPlan?.direction === "conflict") {
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
      await api.executeSync(resolutions);
      appState.previewModalOpen = false;
      await appState.refreshAll();
      appState.addToast("success", t("toasts.sync_success"));
    } catch (err) {
      appState.showError(err);
    } finally {
      appState.isSyncing = false;
    }
  }

  async function handleSaveSettings(payload: main.SettingsPayload) {
    try {
      const updated = await api.updateSettings(payload);
      appState.config = updated;
      appState.settingsDrawerOpen = false;
      appState.addToast("success", t("toasts.save_success"));
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

  function getConnectionForProfile(profileName: string) {
    return appState.config?.connections?.find((c) => c.profile === profileName);
  }

  function getStatusForConnection(connId?: string) {
    if (!connId || !appState.statusPlan?.connections) return undefined;
    return appState.statusPlan.connections.find((c) => c.connectionId === connId);
  }
</script>

<div class="app-layout">
  <Header
    onsync={handleSyncClick}
    onopenSettings={() => (appState.settingsDrawerOpen = true)}
    onchangeRoot={handleSelectRoot}
  />

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

    <main class="main-content">
      <div class="profiles-grid">
        {#each appState.profiles as prof (prof.name)}
          {@const conn = getConnectionForProfile(prof.name)}
          {@const status = getStatusForConnection(conn?.id)}
          <ProfileCard
            profile={prof}
            connection={conn}
            {status}
            onconnect={handleConnectProfile}
            onupdate={handleUpdateConnection}
            onbackups={handleOpenBackups}
            onchangeTarget={handleConnectProfile}
            onopenExplorer={handleOpenExplorer}
            ondisconnect={handleDisconnect}
          />
        {/each}
      </div>
    </main>
  {:else}
    <main class="main-content">
      <WelcomeView onselectFolder={handleSelectRoot} />
    </main>
  {/if}

  <StatusBar />

  <!-- Modals & Drawers -->
  <SyncPreviewModal
    open={appState.previewModalOpen}
    plan={appState.statusPlan}
    onclose={() => (appState.previewModalOpen = false)}
    onconfirm={executeSyncNow}
  />

  <InitialLinkModal
    open={appState.initialLinkModalOpen}
    data={appState.initialLinkData}
    onclose={() => (appState.initialLinkModalOpen = false)}
    onconfirm={handleResolveInitialLink}
  />

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

  <Toast toasts={appState.toasts} ondismiss={(id) => appState.dismissToast(id)} />
</div>

<style>
  .app-layout {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
    background-color: var(--bg);
    color: var(--text);
    font-family: var(--font-sans);
  }

  .banners-container {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-4) var(--space-6) 0 var(--space-6);
  }

  .main-content {
    flex: 1;
    padding: var(--space-6);
    overflow-y: auto;
  }

  .profiles-grid {
    display: grid;
    grid-template-columns: 1fr;
    gap: var(--space-4);
  }

  @media (min-width: 1280px) {
    .profiles-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }
</style>
