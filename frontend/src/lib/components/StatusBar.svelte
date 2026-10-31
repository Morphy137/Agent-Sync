<script lang="ts">
  import { GitBranch } from "@lucide/svelte";
  import { appState } from "../stores/appState.svelte";
  import { t, formatDate } from "../i18n";
  import Spinner from "./Spinner.svelte";

  const branch = $derived(appState.gitInfo?.branch || t("statusbar.no_branch"));
  const ahead = $derived(appState.statusPlan?.ahead ?? appState.gitInfo?.ahead ?? 0);
  const behind = $derived(appState.statusPlan?.behind ?? appState.gitInfo?.behind ?? 0);
  const lastCheck = $derived(
    appState.statusPlan?.lastCheckAt ? formatDate(appState.statusPlan.lastCheckAt) : ""
  );
</script>

<footer class="status-bar">
  <div class="bar-left">
    <span class="branch-item">
      <GitBranch size={12} />
      <span>{branch}</span>
    </span>

    {#if appState.gitInfo?.isRepo && appState.gitInfo?.hasUpstream}
      <span class="sep">·</span>
      <span class="ahead-behind">
        {t("statusbar.ahead_behind", { ahead, behind })}
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
    height: var(--statusbar-height);
    padding: 0 var(--space-6);
    background-color: var(--canvas);
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: var(--text-xs);
    color: var(--text-muted);
    user-select: none;
    z-index: 10;
    font-variant-numeric: tabular-nums;
  }

  .bar-left,
  .bar-right {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .branch-item {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    font-family: var(--font-mono);
    color: var(--text-muted);
  }

  .sep {
    color: var(--text-subtle);
  }

  .ahead-behind {
    font-family: var(--font-mono);
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }

  .checking-indicator {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--ink);
  }
</style>
