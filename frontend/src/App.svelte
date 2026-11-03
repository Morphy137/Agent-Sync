<script lang="ts">
  import { onMount } from "svelte";
  import { EventsOn } from "../wailsjs/runtime/runtime";
  import type { orchestrator } from "../wailsjs/go/models";
  import { api, parseError } from "./lib/api";
  import { appState } from "./lib/stores/appState.svelte";
  import { t } from "./lib/i18n";
  import TitleBar from "./lib/components/TitleBar.svelte";
  import StatusBar from "./lib/components/StatusBar.svelte";
  import WelcomeView from "./lib/components/WelcomeView.svelte";
  import SyncPreviewModal from "./lib/components/SyncPreviewModal.svelte";
  import SettingsDrawer from "./lib/components/SettingsDrawer.svelte";
  import TutorialModal from "./lib/components/TutorialModal.svelte";
  import { updateState } from "./lib/stores/updates.svelte";
  import NotificationCenter, {
    type AppNotification,
  } from "./lib/components/notifications/NotificationCenter.svelte";
  import AddRemoteModal from "./lib/components/AddRemoteModal.svelte";
  import GitIdentityModal from "./lib/components/GitIdentityModal.svelte";
  import Toast from "./lib/components/Toast.svelte";
  import Spinner from "./lib/components/Spinner.svelte";
  import ViewTabs from "./lib/components/ViewTabs.svelte";
  import RulesView from "./lib/views/RulesView.svelte";
  import FilesView from "./lib/views/FilesView.svelte";
  import SubscriptionsView from "./lib/views/SubscriptionsView.svelte";
  import LocationsView from "./lib/views/LocationsView.svelte";
  import RuleEditorModal from "./lib/components/rules/RuleEditorModal.svelte";
  import ConnectRemoteModal from "./lib/components/remote/ConnectRemoteModal.svelte";
  import RepoChangesModal from "./lib/components/remote/RepoChangesModal.svelte";

  /** Payload of the "rules:auto_applied" event (service.AutoAppliedPayload). */
  interface AutoAppliedPayload {
    count: number;
    sources: string[];
  }

  let connectRemoteOpen = $state(false);
  let repoChangesOpen = $state(false);

  onMount(() => {
    appState.loadInitialData();

    EventsOn("status:changed", (payload: orchestrator.SyncPlan) => {
      appState.handleStatusChanged(payload);
    });
    EventsOn("rules:changed", () => appState.bumpRules());
    EventsOn("subscription:blocked", (name: string) =>
      appState.addWarning(t("files.subscription_blocked", { name }))
    );
    EventsOn("rules:auto_applied", (payload: AutoAppliedPayload) =>
      appState.addNotice(
        "auto_applied",
        t("notifications.auto_applied_title"),
        t(`notifications.auto_applied_message_${payload.count === 1 ? "one" : "other"}`, {
          count: payload.count,
          sources: payload.sources.join(", "),
        })
      )
    );
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

  async function handleWelcomeSuccess() {
    await appState.refreshAll();
    if (!appState.config?.tutorialCompleted) appState.tutorialOpen = true;
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
    if (appState.isSyncing) return;
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

  /** Repository state shown as notifications (bell + closable popup) instead of page banners. */
  const updateNotification = $derived.by((): AppNotification[] => {
    const release = updateState.status?.release;
    if (!updateState.available || !release) return [];
    return [
      {
        id: `update_${release.version}`,
        variant: "info",
        title: t("updates.available_title", { version: release.version }),
        message: updateState.canAutoInstall
          ? t("updates.available_message")
          : t("updates.available_manual_message"),
        actions: [
          { label: t("updates.view_notes"), onclick: () => updateState.openReleasePage() },
          {
            label: updateState.canAutoInstall ? t("updates.install") : t("updates.download"),
            primary: true,
            onclick: () => void updateState.install((type, msg) => appState.addToast(type, msg)),
          },
        ],
      },
    ];
  });

  const repoNotifications = $derived.by((): AppNotification[] => {
    if (!appState.config?.rootPath) return [];
    const git = appState.gitInfo;
    const plan = appState.statusPlan;
    const connect = {
      label: t("remote.banner.connect"),
      primary: true,
      onclick: () => (connectRemoteOpen = true),
    };
    if (git && !git.isRepo) {
      return [
        {
          id: "not_repo",
          variant: "warning",
          title: t("banners.not_repo.title"),
          message: t("banners.not_repo.message"),
          actions: [{ label: t("banners.not_repo.button_init"), onclick: handleInitRepo }, connect],
        },
      ];
    }
    const items: AppNotification[] = [];
    if (git && !git.hasUpstream) {
      items.push({
        id: "no_remote",
        variant: "info",
        title: t("banners.no_remote.title"),
        message: t("banners.no_remote.message"),
        actions: [connect],
      });
    }
    if (plan && plan.behind > 0) {
      items.push({
        id: "remote_changes",
        variant: "info",
        title: t("banners.remote_changes.title"),
        message: t("banners.remote_changes.message"),
        actions: [
          { label: t("remote.banner.review"), onclick: () => (repoChangesOpen = true) },
          {
            label: t("banners.remote_changes.button_sync"),
            primary: true,
            onclick: handleSyncClick,
          },
        ],
      });
    }
    const localCount = plan?.commitPreview?.length ?? 0;
    if (localCount > 0) {
      items.push({
        id: "local_changes",
        variant: "info",
        title: t("remote.banner.local_changes_title"),
        message: t(`remote.banner.local_changes_message_${localCount === 1 ? "one" : "other"}`, {
          count: localCount,
        }),
        actions: [
          {
            label: t("remote.banner.review"),
            primary: true,
            onclick: () => (repoChangesOpen = true),
          },
        ],
      });
    }
    return items;
  });

  const allNotifications = $derived([
    ...updateNotification,
    ...repoNotifications,
    ...(appState.ruleNotifications ?? []),
  ]);
</script>

<div class="app-layout">
  <TitleBar
    onsync={handleSyncClick}
    oncheckRemote={() => appState.checkRemote()}
    onopenSettings={() => (appState.settingsDrawerOpen = true)}
    onchangeRoot={handleSelectRoot}
  >
    {#snippet notifications()}
      <NotificationCenter items={allNotifications} />
    {/snippet}
  </TitleBar>

  <main class="content-area">
    <div class="content-inner" class:files-layout={appState.activeView === "files"}>
      {#if !appState.configLoaded}
        <Spinner size="sm" />
      {:else if appState.config?.rootPath}
        <div class="tabs-container">
          <ViewTabs
            value={appState.activeView}
            onchange={(view) => {
              if (appState.fileDirty && !window.confirm(t("files.unsaved_confirm"))) return;
              appState.fileDirty = false;
              appState.activeView = view;
            }}
          />
        </div>

        <div class="view-area">
          {#if appState.activeView === "rules"}
            <RulesView />
          {:else if appState.activeView === "files"}
            <FilesView />
          {:else if appState.activeView === "subscriptions"}
            <SubscriptionsView />
          {:else if appState.activeView === "locations"}
            <LocationsView />
          {/if}
        </div>
      {:else if appState.config}
        <WelcomeView onsuccess={handleWelcomeSuccess} />
      {/if}
    </div>
  </main>

  <StatusBar />

  <SyncPreviewModal
    open={appState.previewModalOpen}
    plan={appState.statusPlan}
    syncing={appState.isSyncing}
    onclose={() => (appState.previewModalOpen = false)}
    onconfirm={executeSyncNow}
  />

  <TutorialModal open={appState.tutorialOpen} onfinish={() => appState.finishTutorial()} />

  <SettingsDrawer
    open={appState.settingsDrawerOpen}
    onclose={() => (appState.settingsDrawerOpen = false)}
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

  <RuleEditorModal />

  <ConnectRemoteModal
    open={connectRemoteOpen}
    onclose={() => (connectRemoteOpen = false)}
    onreview={() => {
      connectRemoteOpen = false;
      repoChangesOpen = true;
    }}
  />

  <RepoChangesModal
    open={repoChangesOpen}
    onclose={() => (repoChangesOpen = false)}
    onsync={() => {
      repoChangesOpen = false;
      handleSyncClick();
    }}
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
    scrollbar-gutter: stable;
    min-height: 0;
  }

  .content-inner {
    min-height: 100%;
    max-width: var(--content-max-width);
    margin: 0 auto;
    padding: var(--space-6) var(--space-8);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .content-inner.files-layout {
    height: 100%;
  }

  /* Long files scroll inside their panel instead of growing the page. */
  .content-inner.files-layout .view-area {
    min-height: 0;
  }

  .tabs-container {
    width: 100%;
    max-width: calc(var(--content-max-width) - 2 * var(--space-8));
    margin-inline: auto;
    flex-shrink: 0;
    display: flex;
  }

  .view-area {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
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
