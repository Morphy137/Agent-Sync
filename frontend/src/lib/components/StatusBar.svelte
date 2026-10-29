<script lang="ts">
  import { GitBranch, ArrowUp, ArrowDown } from "@lucide/svelte";
  import { appState } from "../stores/appState.svelte";
  import { t, formatDate } from "../i18n";
  import Spinner from "./Spinner.svelte";

  const branch = $derived(appState.gitInfo?.branch || t("statusbar.no_branch"));
  const ahead = $derived(appState.gitInfo?.ahead || 0);
  const behind = $derived(appState.gitInfo?.behind || 0);
  const lastCheck = $derived(appState.statusPlan ? formatDate(new Date().toISOString()) : "");
</script>

<footer class="status-bar">
  <div class="bar-left">
    <span class="branch-icon"><GitBranch size={13} /></span>
    <span class="branch-name">{t("statusbar.branch", { branch })}</span>
  </div>

  <div class="bar-center">
    {#if appState.gitInfo?.isRepo && appState.gitInfo?.hasUpstream}
      <span class="ahead-behind">
        <ArrowUp size={12} />
        <span>{ahead}</span>
        <ArrowDown size={12} />
        <span>{behind}</span>
      </span>
    {/if}
  </div>

  <div class="bar-right">
    {#if appState.isChecking}
      <span class="checking-indicator">
        <Spinner size="sm" />
        <span>{t("statusbar.checking")}</span>
      </span>
    {:else if lastCheck}
      <span class="last-check">{t("statusbar.last_check", { time: lastCheck })}</span>
    {/if}
  </div>
</footer>

<style>
  .status-bar {
    height: 32px;
    padding: 0 var(--space-6);
    background-color: var(--surface);
    border-top: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: var(--text-xs);
    color: var(--text-muted);
    user-select: none;
    z-index: 10;
  }

  .bar-left,
  .bar-center,
  .bar-right {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .branch-icon {
    color: var(--text-subtle);
  }

  .ahead-behind {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    font-family: var(--font-mono);
  }

  .checking-indicator {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--accent);
  }
</style>
